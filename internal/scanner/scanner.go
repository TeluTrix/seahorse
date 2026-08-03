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
	"golang.org/x/text/unicode/norm"
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

// remuxTask is one backlogged transcode.RemuxAudio job, discovered during a
// scan's metadata-import pass but not run until that pass fully completes
// (see processBacklog) — so heavy remux I/O never competes with directory
// walking, TMDB calls, or cover downloads for the rest of the library.
type remuxTask struct {
	MediaID  uuid.UUID
	FilePath string
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

	// remuxBacklog collects pending remux jobs during the metadata-import
	// pass (see queueRemux); processBacklog drains it afterward.
	remuxBacklog []remuxTask
	// mediaRemuxState tracks "pending"/"active" per movie/episode ID so the
	// API can tell a detail page's viewer "this file's audio fix hasn't run
	// yet" instead of it just silently not working. Absent (no entry) means
	// no remux is needed or it already finished. Lost on restart, same as
	// the rest of the scanner's in-memory state — harmless, since a future
	// scan naturally re-detects and re-queues any file that still needs it.
	mediaRemuxState map[uuid.UUID]string

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

// queueRemux records a backlogged remux job for mediaID/path. It does not
// start any work itself — see processBacklog, which runs everything queued
// this way after the scan's metadata-import pass finishes.
func (s *Scanner) queueRemux(mediaID uuid.UUID, path string) {
	s.mu.Lock()
	s.remuxBacklog = append(s.remuxBacklog, remuxTask{MediaID: mediaID, FilePath: path})
	if s.mediaRemuxState == nil {
		s.mediaRemuxState = map[uuid.UUID]string{}
	}
	s.mediaRemuxState[mediaID] = "pending"
	s.mu.Unlock()
}

// setMediaRemuxState upserts (or, for state == "", clears) the remux state
// tracked for mediaID.
func (s *Scanner) setMediaRemuxState(mediaID uuid.UUID, state string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if state == "" {
		delete(s.mediaRemuxState, mediaID)
		return
	}
	if s.mediaRemuxState == nil {
		s.mediaRemuxState = map[uuid.UUID]string{}
	}
	s.mediaRemuxState[mediaID] = state
}

// RemuxState reports whether mediaID (a movie or episode ID) currently has
// a backlogged ("pending") or in-flight ("active") audio remux, or "" if
// neither (no fix needed, or it already finished).
func (s *Scanner) RemuxState(mediaID uuid.UUID) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.mediaRemuxState[mediaID]
}

// TMDBClient exposes the scanner's already-configured TMDB client for
// one-off lookups outside a full scan, e.g. an admin-triggered metadata
// refresh for a single movie/show.
func (s *Scanner) TMDBClient() *tmdb.Client {
	return s.tmdb
}

// TranscodeOptions exposes the scanner's configured transcode options for
// one-off use outside a full scan, e.g. lazily preparing a specific audio
// track when it's first requested for streaming.
func (s *Scanner) TranscodeOptions() transcode.Options {
	return s.transcodeOpts
}

// processBacklog runs every remux job queued via queueRemux since the last
// call, bounded by remuxSlots, and blocks until they've all finished.
// Errors are logged, not returned, matching the previous inline behavior
// where a failed remux never aborted the scan.
func (s *Scanner) processBacklog() {
	s.mu.Lock()
	backlog := s.remuxBacklog
	s.remuxBacklog = nil
	s.mu.Unlock()

	for _, task := range backlog {
		s.remuxWG.Add(1)
		go func(task remuxTask) {
			defer s.remuxWG.Done()
			s.remuxSlots <- struct{}{}
			defer func() { <-s.remuxSlots }()

			base := filepath.Base(task.FilePath)
			s.setStatus(func(st *Status) { st.CurrentItem = "remuxing audio: " + base })
			s.setRemuxProgress(base, 0)
			s.setMediaRemuxState(task.MediaID, "active")
			defer s.clearRemuxJob(base)
			defer s.setMediaRemuxState(task.MediaID, "")

			if err := transcode.RemuxAudio(task.FilePath, s.transcodeOpts, func(percent float64) {
				s.setRemuxProgress(base, percent)
			}); err != nil {
				slog.Warn("could not remux incompatible audio", "file", task.FilePath, "error", err)
			}
		}(task)
	}

	s.remuxWG.Wait()
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
	if err := dedupeExistingMedia(); err != nil {
		slog.Warn("could not clean up duplicate media rows", "error", err)
	}

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

	s.setStatus(func(st *Status) { st.CurrentItem = "processing audio remux backlog" })
	s.processBacklog()

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

func RemoveCoverFiles(dir string) {
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
		RemoveCoverFiles(filepath.Dir(m.FilePath))
	}

	var shows []models.TVShow
	if err := db.DB.Find(&shows).Error; err != nil {
		return err
	}
	for _, sh := range shows {
		RemoveCoverFiles(sh.FolderPath)
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

// dedupeExistingMedia removes duplicate movie/episode rows left behind by
// the (now-fixed, see normalizedPath) bug where a differently-normalized
// duplicate of an already-known file's path slipped past the "already
// scanned" check and got inserted as a second row. Runs at the start of
// every scan so any stragglers self-heal; a no-op once nothing's left to
// merge.
func dedupeExistingMedia() error {
	if err := dedupeMoviesByPath(); err != nil {
		return fmt.Errorf("movies: %w", err)
	}
	if err := dedupeEpisodesByPath(); err != nil {
		return fmt.Errorf("episodes: %w", err)
	}
	return nil
}

type dupCandidate struct {
	ID        uuid.UUID
	CreatedAt time.Time
}

// resolveDuplicates picks which of a group of same-identity rows to keep.
// If none of them has its own watch progress, the oldest row survives.  If
// exactly one does, that one survives (never discard someone's watch
// history to keep an arbitrary "first" row). If *more than one* has its own
// progress, merging is ambiguous — ok is false and the group is left alone
// entirely for manual attention rather than guessed at.
func resolveDuplicates(mediaType models.MediaType, group []dupCandidate) (toDelete []uuid.UUID, ok bool) {
	ids := make([]uuid.UUID, len(group))
	for i, c := range group {
		ids[i] = c.ID
	}

	var withProgress []uuid.UUID
	if err := db.DB.Model(&models.WatchProgress{}).
		Where("media_type = ? AND media_id IN ?", mediaType, ids).
		Distinct("media_id").Pluck("media_id", &withProgress).Error; err != nil {
		return nil, false
	}
	if len(withProgress) > 1 {
		return nil, false
	}

	survivor := group[0].ID
	if len(withProgress) == 1 {
		survivor = withProgress[0]
	} else {
		earliest := group[0].CreatedAt
		for _, c := range group[1:] {
			if c.CreatedAt.Before(earliest) {
				survivor, earliest = c.ID, c.CreatedAt
			}
		}
	}

	for _, id := range ids {
		if id != survivor {
			toDelete = append(toDelete, id)
		}
	}
	return toDelete, true
}

func dedupeMoviesByPath() error {
	var movies []models.Movie
	if err := db.DB.Find(&movies).Error; err != nil {
		return err
	}

	groups := map[string][]models.Movie{}
	for _, m := range movies {
		key := norm.NFC.String(m.FilePath)
		groups[key] = append(groups[key], m)
	}

	for path, group := range groups {
		if len(group) < 2 {
			continue
		}
		candidates := make([]dupCandidate, len(group))
		for i, m := range group {
			candidates[i] = dupCandidate{ID: m.ID, CreatedAt: m.CreatedAt}
		}
		toDelete, ok := resolveDuplicates(models.MediaTypeMovie, candidates)
		if !ok {
			slog.Warn("multiple duplicate movie rows each have their own watch progress; leaving them for manual cleanup", "path", path)
			continue
		}
		if len(toDelete) == 0 {
			continue
		}
		if err := db.DB.Unscoped().Where("id IN ?", toDelete).Delete(&models.Movie{}).Error; err != nil {
			return err
		}
		slog.Info("removed duplicate movie rows (stale path-normalization mismatch)", "path", path, "removed", len(toDelete))
	}
	return nil
}

func dedupeEpisodesByPath() error {
	var episodes []models.Episode
	if err := db.DB.Find(&episodes).Error; err != nil {
		return err
	}

	groups := map[string][]models.Episode{}
	for _, e := range episodes {
		key := norm.NFC.String(e.FilePath)
		groups[key] = append(groups[key], e)
	}

	for path, group := range groups {
		if len(group) < 2 {
			continue
		}
		candidates := make([]dupCandidate, len(group))
		for i, e := range group {
			candidates[i] = dupCandidate{ID: e.ID, CreatedAt: e.CreatedAt}
		}
		toDelete, ok := resolveDuplicates(models.MediaTypeEpisode, candidates)
		if !ok {
			slog.Warn("multiple duplicate episode rows each have their own watch progress; leaving them for manual cleanup", "path", path)
			continue
		}
		if len(toDelete) == 0 {
			continue
		}
		if err := db.DB.Where("id IN ?", toDelete).Delete(&models.Episode{}).Error; err != nil {
			return err
		}
		slog.Info("removed duplicate episode rows (stale path-normalization mismatch)", "path", path, "removed", len(toDelete))
	}
	return nil
}

// DownloadCover ensures dir contains a local cover.{webp,jpg,jpeg,png}. If
// one already exists it's left as-is (no network call). Returns whether a
// local cover ended up present.
//
// Fetches TMDB's w500 size rather than "original" (often 2000x3000px,
// multiple MB) since posters are never displayed larger than ~220px in this
// UI — w500 is already generous headroom at a fraction of the size. If
// ffmpeg is available, the downloaded JPEG is further converted to WebP
// (~25-35% smaller again) and the intermediate JPEG removed; otherwise the
// JPEG is kept as-is.
func DownloadCover(dir, posterPath string) bool {
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

	if ffmpeg.WebPSupported() {
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

			added, scanErr := s.ScanMovie(moviesPath, entry.Name())
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

			isNewShow, n, scanErr := s.ScanTVShow(tvPath, entry.Name())
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
			return normalizedPath(dir, entry.Name()), nil
		}
	}
	return "", fmt.Errorf("no video file found in %s", dir)
}

// normalizedPath joins dir and name into a path, normalized to NFC. Some
// filesystems/tools return accented filenames decomposed (NFD) rather than
// precomposed (NFC) — two visually-identical filenames that are different
// byte sequences — which previously let a rescan's "already known" lookup
// (a plain string comparison against the stored FilePath/FolderPath) miss
// and create a duplicate row for the same file. Every path used as that
// lookup key or stored value goes through this so the comparison is always
// apples-to-apples regardless of which form the filesystem handed back.
func normalizedPath(dir, name string) string {
	return norm.NFC.String(filepath.Join(dir, name))
}

// ScanMovie returns whether a new movie was added. Movies already known by
// FilePath are left completely untouched (no re-fetch of metadata) — exported
// so an admin action can force-rediscover a single movie from scratch (a
// fresh TMDB search, not just a re-fetch by the same TMDB ID) by deleting its
// row first and calling this directly, the per-item equivalent of a full
// library rescan.
func (s *Scanner) ScanMovie(moviesRoot, folderName string) (bool, error) {
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

	movieID := uuid.New()
	if needsFix, err := transcode.NeedsAudioRemux(videoFile, s.transcodeOpts); err != nil {
		slog.Warn("could not probe audio codec", "file", videoFile, "error", err)
	} else if needsFix {
		s.queueRemux(movieID, videoFile)
	}

	movie := models.Movie{
		ID:           movieID,
		FilePath:     videoFile,
		TMDBID:       meta.TMDBID,
		Title:        meta.Title,
		Overview:     meta.Overview,
		PosterPath:   meta.PosterPath,
		BackdropPath: meta.BackdropPath,
		ReleaseDate:  meta.ReleaseDate,
		VoteAverage:  meta.VoteAverage,
		Genres:       tmdb.JoinGenres(meta.Genres),
		CoverCached:  DownloadCover(filepath.Dir(videoFile), meta.PosterPath),
	}

	if details, err := s.tmdb.GetMovieDetails(meta.TMDBID); err != nil {
		slog.Warn("could not fetch tmdb movie details (runtime/cast)", "title", meta.Title, "error", err)
	} else {
		movie.Runtime = details.Runtime
		movie.Director = details.Director
		if encoded, err := json.Marshal(details.Cast); err == nil {
			movie.Cast = string(encoded)
		}
		movie.Tagline = details.Tagline
		movie.OriginalLanguage = details.OriginalLanguage
		movie.Budget = details.Budget
		movie.Revenue = details.Revenue
		movie.ProductionCompanies = strings.Join(details.ProductionCompanies, ", ")
		movie.ProductionCountries = strings.Join(details.ProductionCountries, ", ")
	}

	return true, db.DB.Create(&movie).Error
}

// ScanTVShow returns whether the show itself is new, plus the number of new
// episodes found (which can be > 0 even for an already-known show) —
// exported for the same reason as ScanMovie: an admin action can
// force-rediscover a single show (and all its seasons/episodes) from scratch
// by deleting its rows first and calling this directly.
func (s *Scanner) ScanTVShow(tvRoot, folderName string) (bool, int, error) {
	// See scanMovie: fall back to a title-only search (no year constraint)
	// rather than skipping the folder outright when it doesn't match the
	// strict "Title (Year)" pattern.
	title, yearNum := folderName, 0
	if matches := folderRegex.FindStringSubmatch(folderName); matches != nil {
		title = matches[1]
		yearNum, _ = strconv.Atoi(matches[2])
	}

	showFolder := normalizedPath(tvRoot, folderName)

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
			CoverCached:  DownloadCover(showFolder, meta.PosterPath),
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
		filePath := normalizedPath(seasonPath, entry.Name())

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
			s.queueRemux(episode.ID, p.filePath)
		}

		if err := db.DB.Create(&episode).Error; err != nil {
			return count, err
		}
		count++
	}

	return count, nil
}
