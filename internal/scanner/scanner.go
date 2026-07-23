package scanner

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/TeluTrix/seahorse/internal/db"
	"github.com/TeluTrix/seahorse/internal/ffmpeg"
	"github.com/TeluTrix/seahorse/internal/models"
	"github.com/TeluTrix/seahorse/internal/tmdb"
	"github.com/TeluTrix/seahorse/internal/transcode"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

var (
	folderRegex  = regexp.MustCompile(`^(.+?) \((\d{4})\)$`)
	seasonRegex  = regexp.MustCompile(`(?i)^season\s*0*(\d+)$`)
	episodeRegex = regexp.MustCompile(`(?i)s(\d{2})e(\d{2})`)
	videoExtSet  = map[string]bool{".mp4": true, ".mkv": true, ".avi": true, ".mov": true, ".webm": true}
	// webp checked first: covers cached after the WebP optimization was added
	// use it; jpg/jpeg/png remain for backward compatibility with covers
	// cached before that.
	coverExts = []string{"webp", "jpg", "jpeg", "png"}
)

// RemuxJob reports live progress for one in-flight transcode.RemuxAudio call.
// Percent is best-effort (0 if the source file's duration couldn't be
// determined) and File is just the base filename, for display purposes.
type RemuxJob struct {
	File    string  `json:"file"`
	Percent float64 `json:"percent"`
}

type Status struct {
	State         string     `json:"state"` // idle, running, done, error
	CurrentItem   string     `json:"current_item,omitempty"`
	MoviesFound   int        `json:"movies_found"`
	ShowsFound    int        `json:"shows_found"`
	EpisodesFound int        `json:"episodes_found"`
	RemuxJobs     []RemuxJob `json:"remux_jobs,omitempty"`
	Error         string     `json:"error,omitempty"`
	StartedAt     time.Time  `json:"started_at,omitempty"`
	FinishedAt    time.Time  `json:"finished_at,omitempty"`
}

type Scanner struct {
	tmdb *tmdb.Client

	mu          sync.Mutex
	status      Status
	subscribers []chan Status

	// remuxSlots bounds how many transcode.RemuxAudio jobs run at once: the
	// slow step (full read+write of a large video file) doesn't need the
	// TMDB lookups/DB writes around it to wait, so it's dispatched to the
	// background and only throttled by this semaphore. remuxWG lets a scan
	// wait for all its outstanding remux jobs to finish before reporting done.
	remuxSlots chan struct{}
	remuxWG    sync.WaitGroup

	transcodeOpts transcode.Options
}

// New creates a Scanner. remuxConcurrency bounds how many audio remux jobs
// (see transcode.RemuxAudio) run at once; callers should pass at least 1.
func New(tmdbClient *tmdb.Client, remuxConcurrency int, transcodeOpts transcode.Options) *Scanner {
	if remuxConcurrency < 1 {
		remuxConcurrency = 1
	}
	return &Scanner{
		tmdb:          tmdbClient,
		status:        Status{State: "idle"},
		remuxSlots:    make(chan struct{}, remuxConcurrency),
		transcodeOpts: transcodeOpts,
	}
}

// setRemuxProgress upserts the live progress entry for file (by base name).
func (s *Scanner) setRemuxProgress(file string, percent float64) {
	s.setStatus(func(st *Status) {
		for i := range st.RemuxJobs {
			if st.RemuxJobs[i].File == file {
				st.RemuxJobs[i].Percent = percent
				return
			}
		}
		st.RemuxJobs = append(st.RemuxJobs, RemuxJob{File: file, Percent: percent})
	})
}

// clearRemuxJob removes file's progress entry once its remux is done
// (successfully or not) so the status doesn't keep reporting a finished job.
func (s *Scanner) clearRemuxJob(file string) {
	s.setStatus(func(st *Status) {
		for i, j := range st.RemuxJobs {
			if j.File == file {
				st.RemuxJobs = append(st.RemuxJobs[:i], st.RemuxJobs[i+1:]...)
				return
			}
		}
	})
}

// queueRemux runs an audio remux of path in the background, bounded by
// remuxSlots. Errors are logged, not returned, matching the previous inline
// behavior where a failed remux never aborted the scan.
func (s *Scanner) queueRemux(path string) {
	s.remuxWG.Add(1)
	go func() {
		defer s.remuxWG.Done()
		s.remuxSlots <- struct{}{}
		defer func() { <-s.remuxSlots }()

		base := filepath.Base(path)
		s.setStatus(func(st *Status) { st.CurrentItem = "remuxing audio: " + base })
		s.setRemuxProgress(base, 0)
		defer s.clearRemuxJob(base)

		if err := transcode.RemuxAudio(path, s.transcodeOpts, func(percent float64) {
			s.setRemuxProgress(base, percent)
		}); err != nil {
			slog.Warn("could not remux incompatible audio", "file", path, "error", err)
		}
	}()
}

func (s *Scanner) Status() Status {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.status
}

// Subscribe registers for live status updates, immediately receiving the
// current status as the first message. The returned cancel func must be
// called (typically via defer) to unregister and release the channel once
// the subscriber (an SSE connection) goes away.
func (s *Scanner) Subscribe() (<-chan Status, func()) {
	ch := make(chan Status, 8)

	s.mu.Lock()
	s.subscribers = append(s.subscribers, ch)
	current := s.status
	s.mu.Unlock()

	ch <- current

	cancel := func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		for i, c := range s.subscribers {
			if c == ch {
				s.subscribers = append(s.subscribers[:i], s.subscribers[i+1:]...)
				close(c)
				break
			}
		}
	}
	return ch, cancel
}

// broadcast pushes a status snapshot to every subscriber (a non-blocking
// send — a subscriber slow enough to fill its buffer just misses an
// intermediate update, since the next one supersedes it anyway).
func (s *Scanner) broadcast(snapshot Status, subs []chan Status) {
	for _, ch := range subs {
		select {
		case ch <- snapshot:
		default:
		}
	}
}

// setStatus mutates the status under lock and broadcasts the result.
func (s *Scanner) setStatus(mutate func(*Status)) {
	s.mu.Lock()
	mutate(&s.status)
	snapshot := s.status
	subs := append([]chan Status(nil), s.subscribers...)
	s.mu.Unlock()

	s.broadcast(snapshot, subs)
}

var ErrScanInProgress = errors.New("a scan is already running")

// StartScan kicks off a scan in the background. When full is true, all
// existing movies/shows/seasons/episodes (and their cached cover images) are
// wiped first, so the scan re-fetches everything from scratch; otherwise
// only folders/episodes not already known are added, and existing metadata
// is left untouched.
func (s *Scanner) StartScan(libraryPath string, full bool) error {
	// The check-and-set must happen atomically under one lock acquisition —
	// otherwise two concurrent calls could both see "not running" and both
	// proceed to start a scan.
	s.mu.Lock()
	if s.status.State == "running" {
		s.mu.Unlock()
		return ErrScanInProgress
	}
	s.status = Status{State: "running", StartedAt: time.Now()}
	snapshot := s.status
	subs := append([]chan Status(nil), s.subscribers...)
	s.mu.Unlock()

	s.broadcast(snapshot, subs)

	go s.run(libraryPath, full)
	return nil
}

func (s *Scanner) run(libraryPath string, full bool) {
	if full {
		if err := wipeAllMedia(); err != nil {
			s.setStatus(func(st *Status) {
				st.State = "error"
				st.Error = err.Error()
				st.FinishedAt = time.Now()
			})
			return
		}
	}

	err := s.scan(libraryPath)

	s.setStatus(func(st *Status) { st.CurrentItem = "finishing audio remux jobs" })
	s.remuxWG.Wait()

	s.setStatus(func(st *Status) {
		st.CurrentItem = ""
		st.FinishedAt = time.Now()
		if err != nil {
			st.State = "error"
			st.Error = err.Error()
			return
		}
		st.State = "done"
	})
}

func removeCoverFiles(dir string) {
	for _, ext := range coverExts {
		_ = os.Remove(filepath.Join(dir, "cover."+ext))
	}
}

// wipeAllMedia hard-deletes every movie/show/season/episode row and
// best-effort removes their cached cover files, so a full rescan starts
// from a clean slate.
func wipeAllMedia() error {
	var movies []models.Movie
	if err := db.DB.Find(&movies).Error; err != nil {
		return err
	}
	for _, m := range movies {
		removeCoverFiles(filepath.Dir(m.FilePath))
	}

	var shows []models.TVShow
	if err := db.DB.Find(&shows).Error; err != nil {
		return err
	}
	for _, sh := range shows {
		removeCoverFiles(sh.FolderPath)
	}

	if err := db.DB.Where("1 = 1").Delete(&models.Episode{}).Error; err != nil {
		return err
	}
	if err := db.DB.Where("1 = 1").Delete(&models.Season{}).Error; err != nil {
		return err
	}
	if err := db.DB.Unscoped().Where("1 = 1").Delete(&models.TVShow{}).Error; err != nil {
		return err
	}
	if err := db.DB.Unscoped().Where("1 = 1").Delete(&models.Movie{}).Error; err != nil {
		return err
	}
	return nil
}

// downloadCover ensures dir contains a local cover.{webp,jpg,jpeg,png}. If
// one already exists it's left as-is (no network call). Returns whether a
// local cover ended up present.
//
// Fetches TMDB's w500 size rather than "original" (often 2000x3000px,
// multiple MB) since posters are never displayed larger than ~220px in this
// UI — w500 is already generous headroom at a fraction of the size. If
// ffmpeg is available, the downloaded JPEG is further converted to WebP
// (~25-35% smaller again) and the intermediate JPEG removed; otherwise the
// JPEG is kept as-is.
func downloadCover(dir, posterPath string) bool {
	for _, ext := range coverExts {
		if _, err := os.Stat(filepath.Join(dir, "cover."+ext)); err == nil {
			return true
		}
	}
	if posterPath == "" {
		return false
	}

	resp, err := http.Get(tmdb.ImageURL(posterPath, "w500"))
	if err != nil {
		slog.Warn("could not download cover image", "dir", dir, "error", err)
		return false
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		slog.Warn("could not download cover image", "dir", dir, "status", resp.StatusCode)
		return false
	}

	jpgPath := filepath.Join(dir, "cover.jpg")
	out, err := os.Create(jpgPath)
	if err != nil {
		slog.Warn("could not write cover image", "dir", dir, "error", err)
		return false
	}
	if _, err := io.Copy(out, resp.Body); err != nil {
		out.Close()
		slog.Warn("could not write cover image", "dir", dir, "error", err)
		return false
	}
	out.Close()

	if ffmpeg.Available() {
		webpPath := filepath.Join(dir, "cover.webp")
		if err := transcode.ConvertToWebP(jpgPath, webpPath); err != nil {
			slog.Warn("could not convert cover to webp, keeping jpg", "dir", dir, "error", err)
		} else {
			os.Remove(jpgPath)
		}
	}

	return true
}

func (s *Scanner) scan(libraryPath string) error {
	moviesPath := filepath.Join(libraryPath, "movies")
	tvPath := filepath.Join(libraryPath, "tvshows")

	if entries, readErr := os.ReadDir(moviesPath); readErr == nil {
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			s.setStatus(func(st *Status) { st.CurrentItem = "movie: " + entry.Name() })

			added, scanErr := s.scanMovie(moviesPath, entry.Name())
			if scanErr != nil {
				slog.Warn("skipping movie folder", "folder", entry.Name(), "error", scanErr)
				continue
			}
			if added {
				s.setStatus(func(st *Status) { st.MoviesFound++ })
			}
		}
	} else {
		slog.Warn("could not read movies library path", "path", moviesPath, "error", readErr)
	}

	if entries, readErr := os.ReadDir(tvPath); readErr == nil {
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			s.setStatus(func(st *Status) { st.CurrentItem = "tv show: " + entry.Name() })

			isNewShow, n, scanErr := s.scanTVShow(tvPath, entry.Name())
			if scanErr != nil {
				slog.Warn("skipping tv show folder", "folder", entry.Name(), "error", scanErr)
				continue
			}
			s.setStatus(func(st *Status) {
				if isNewShow {
					st.ShowsFound++
				}
				st.EpisodesFound += n
			})
		}
	} else {
		slog.Warn("could not read tvshows library path", "path", tvPath, "error", readErr)
	}

	return nil
}

func findVideoFile(dir string) (string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if videoExtSet[strings.ToLower(filepath.Ext(entry.Name()))] {
			return filepath.Join(dir, entry.Name()), nil
		}
	}
	return "", fmt.Errorf("no video file found in %s", dir)
}

// scanMovie returns whether a new movie was added. Movies already known by
// FilePath are left completely untouched (no re-fetch of metadata).
func (s *Scanner) scanMovie(moviesRoot, folderName string) (bool, error) {
	// Prefer the strict "Title (Year)" pattern for a more accurate TMDB
	// match, but don't give up on folders that lack it — fall back to
	// searching by the whole folder name with no year constraint (which
	// tmdb.SearchMovie already handles for year == 0). Only if that
	// title-only search also comes up empty does this end up logged as
	// unmatched by the caller.
	title, yearNum := folderName, 0
	if matches := folderRegex.FindStringSubmatch(folderName); matches != nil {
		title = matches[1]
		yearNum, _ = strconv.Atoi(matches[2])
	}

	videoFile, err := findVideoFile(filepath.Join(moviesRoot, folderName))
	if err != nil {
		return false, err
	}

	var existing models.Movie
	result := db.DB.Where("file_path = ?", videoFile).First(&existing)
	if result.Error == nil {
		return false, nil
	}
	if !errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return false, result.Error
	}

	meta, err := s.tmdb.SearchMovie(title, yearNum)
	if err != nil {
		return false, err
	}

	if needsFix, err := transcode.NeedsAudioRemux(videoFile, s.transcodeOpts); err != nil {
		slog.Warn("could not probe audio codec", "file", videoFile, "error", err)
	} else if needsFix {
		s.queueRemux(videoFile)
	}

	movie := models.Movie{
		ID:           uuid.New(),
		FilePath:     videoFile,
		TMDBID:       meta.TMDBID,
		Title:        meta.Title,
		Overview:     meta.Overview,
		PosterPath:   meta.PosterPath,
		BackdropPath: meta.BackdropPath,
		ReleaseDate:  meta.ReleaseDate,
		VoteAverage:  meta.VoteAverage,
		Genres:       tmdb.JoinGenres(meta.Genres),
		CoverCached:  downloadCover(filepath.Dir(videoFile), meta.PosterPath),
	}

	if details, err := s.tmdb.GetMovieDetails(meta.TMDBID); err != nil {
		slog.Warn("could not fetch tmdb movie details (runtime/cast)", "title", meta.Title, "error", err)
	} else {
		movie.Runtime = details.Runtime
		movie.Director = details.Director
		if encoded, err := json.Marshal(details.Cast); err == nil {
			movie.Cast = string(encoded)
		}
	}

	return true, db.DB.Create(&movie).Error
}

// scanTVShow returns whether the show itself is new, plus the number of new
// episodes found (which can be > 0 even for an already-known show).
func (s *Scanner) scanTVShow(tvRoot, folderName string) (bool, int, error) {
	// See scanMovie: fall back to a title-only search (no year constraint)
	// rather than skipping the folder outright when it doesn't match the
	// strict "Title (Year)" pattern.
	title, yearNum := folderName, 0
	if matches := folderRegex.FindStringSubmatch(folderName); matches != nil {
		title = matches[1]
		yearNum, _ = strconv.Atoi(matches[2])
	}

	showFolder := filepath.Join(tvRoot, folderName)

	var show models.TVShow
	result := db.DB.Where("folder_path = ?", showFolder).First(&show)
	isNewShow := errors.Is(result.Error, gorm.ErrRecordNotFound)
	if result.Error != nil && !isNewShow {
		return false, 0, result.Error
	}

	if isNewShow {
		meta, err := s.tmdb.SearchTV(title, yearNum)
		if err != nil {
			return false, 0, err
		}
		show = models.TVShow{
			ID:           uuid.New(),
			FolderPath:   showFolder,
			TMDBID:       meta.TMDBID,
			Title:        meta.Title,
			Overview:     meta.Overview,
			PosterPath:   meta.PosterPath,
			BackdropPath: meta.BackdropPath,
			FirstAirDate: meta.FirstAirDate,
			VoteAverage:  meta.VoteAverage,
			Genres:       tmdb.JoinGenres(meta.Genres),
			CoverCached:  downloadCover(showFolder, meta.PosterPath),
		}

		if details, err := s.tmdb.GetTVDetails(meta.TMDBID); err != nil {
			slog.Warn("could not fetch tmdb tv details (cast)", "title", meta.Title, "error", err)
		} else {
			show.Creators = strings.Join(details.Creators, ", ")
			if encoded, err := json.Marshal(details.Cast); err == nil {
				show.Cast = string(encoded)
			}
		}

		if err := db.DB.Create(&show).Error; err != nil {
			return false, 0, err
		}
	}

	seasonEntries, err := os.ReadDir(showFolder)
	if err != nil {
		return isNewShow, 0, err
	}

	episodeCount := 0
	for _, entry := range seasonEntries {
		if !entry.IsDir() {
			continue
		}
		seasonMatch := seasonRegex.FindStringSubmatch(entry.Name())
		if seasonMatch == nil {
			continue
		}
		seasonNumber, _ := strconv.Atoi(seasonMatch[1])

		s.setStatus(func(st *Status) {
			st.CurrentItem = fmt.Sprintf("%s: season %d", show.Title, seasonNumber)
		})

		n, err := s.scanSeason(show, filepath.Join(showFolder, entry.Name()), seasonNumber)
		if err != nil {
			slog.Warn("skipping season folder", "show", show.Title, "folder", entry.Name(), "error", err)
			continue
		}
		episodeCount += n
	}

	return isNewShow, episodeCount, nil
}

func (s *Scanner) scanSeason(show models.TVShow, seasonPath string, seasonNumber int) (int, error) {
	entries, err := os.ReadDir(seasonPath)
	if err != nil {
		return 0, err
	}

	// Only consider episode files not already known, so an already-scanned
	// season with nothing new never triggers a TMDB call.
	type pendingEpisode struct {
		filePath      string
		episodeNumber int
	}
	var pending []pendingEpisode
	for _, entry := range entries {
		if entry.IsDir() || !videoExtSet[strings.ToLower(filepath.Ext(entry.Name()))] {
			continue
		}
		epMatch := episodeRegex.FindStringSubmatch(entry.Name())
		if epMatch == nil {
			continue
		}
		filePath := filepath.Join(seasonPath, entry.Name())

		var count int64
		if err := db.DB.Model(&models.Episode{}).Where("file_path = ?", filePath).Count(&count).Error; err != nil {
			return 0, err
		}
		if count > 0 {
			continue
		}

		episodeNumber, _ := strconv.Atoi(epMatch[2])
		pending = append(pending, pendingEpisode{filePath: filePath, episodeNumber: episodeNumber})
	}

	if len(pending) == 0 {
		return 0, nil
	}

	var season models.Season
	result := db.DB.Where("tv_show_id = ? AND season_number = ?", show.ID, seasonNumber).First(&season)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		season = models.Season{ID: uuid.New(), TVShowID: show.ID, SeasonNumber: seasonNumber}
		if err := db.DB.Create(&season).Error; err != nil {
			return 0, err
		}
	} else if result.Error != nil {
		return 0, result.Error
	}

	episodeMeta, err := s.tmdb.GetTVSeasonEpisodes(show.TMDBID, seasonNumber)
	if err != nil {
		slog.Warn("could not fetch tmdb episode metadata", "show", show.Title, "season", seasonNumber, "error", err)
		episodeMeta = nil
	}

	count := 0
	for _, p := range pending {
		episode := models.Episode{ID: uuid.New(), SeasonID: season.ID, FilePath: p.filePath, EpisodeNumber: p.episodeNumber}
		episode.Title = fmt.Sprintf("Episode %d", p.episodeNumber)
		if meta, found := tmdb.FindEpisode(episodeMeta, p.episodeNumber); found {
			episode.Title = meta.Title
			episode.Overview = meta.Overview
			episode.StillPath = meta.StillPath
			episode.Runtime = meta.Runtime
		}

		if needsFix, err := transcode.NeedsAudioRemux(p.filePath, s.transcodeOpts); err != nil {
			slog.Warn("could not probe audio codec", "file", p.filePath, "error", err)
		} else if needsFix {
			s.queueRemux(p.filePath)
		}

		if err := db.DB.Create(&episode).Error; err != nil {
			return count, err
		}
		count++
	}

	return count, nil
}
