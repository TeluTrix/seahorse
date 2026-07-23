package scanner

import (
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
	"github.com/TeluTrix/seahorse/internal/models"
	"github.com/TeluTrix/seahorse/internal/tmdb"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

var (
	folderRegex  = regexp.MustCompile(`^(.+?) \((\d{4})\)$`)
	seasonRegex  = regexp.MustCompile(`(?i)^season\s*0*(\d+)$`)
	episodeRegex = regexp.MustCompile(`(?i)s(\d{2})e(\d{2})`)
	videoExtSet  = map[string]bool{".mp4": true, ".mkv": true, ".avi": true, ".mov": true, ".webm": true}
	coverExts    = []string{"jpg", "jpeg", "png"}
)

type Status struct {
	State         string    `json:"state"` // idle, running, done, error
	MoviesFound   int       `json:"movies_found"`
	ShowsFound    int       `json:"shows_found"`
	EpisodesFound int       `json:"episodes_found"`
	Error         string    `json:"error,omitempty"`
	StartedAt     time.Time `json:"started_at,omitempty"`
	FinishedAt    time.Time `json:"finished_at,omitempty"`
}

type Scanner struct {
	tmdb *tmdb.Client

	mu     sync.Mutex
	status Status
}

func New(tmdbClient *tmdb.Client) *Scanner {
	return &Scanner{tmdb: tmdbClient, status: Status{State: "idle"}}
}

func (s *Scanner) Status() Status {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.status
}

var ErrScanInProgress = errors.New("a scan is already running")

// StartScan kicks off a scan in the background. When full is true, all
// existing movies/shows/seasons/episodes (and their cached cover images) are
// wiped first, so the scan re-fetches everything from scratch; otherwise
// only folders/episodes not already known are added, and existing metadata
// is left untouched.
func (s *Scanner) StartScan(libraryPath string, full bool) error {
	s.mu.Lock()
	if s.status.State == "running" {
		s.mu.Unlock()
		return ErrScanInProgress
	}
	s.status = Status{State: "running", StartedAt: time.Now()}
	s.mu.Unlock()

	go s.run(libraryPath, full)
	return nil
}

func (s *Scanner) run(libraryPath string, full bool) {
	if full {
		if err := wipeAllMedia(); err != nil {
			s.mu.Lock()
			s.status.State = "error"
			s.status.Error = err.Error()
			s.status.FinishedAt = time.Now()
			s.mu.Unlock()
			return
		}
	}

	movies, shows, episodes, err := s.scan(libraryPath)

	s.mu.Lock()
	defer s.mu.Unlock()
	s.status.FinishedAt = time.Now()
	if err != nil {
		s.status.State = "error"
		s.status.Error = err.Error()
		return
	}
	s.status.State = "done"
	s.status.MoviesFound = movies
	s.status.ShowsFound = shows
	s.status.EpisodesFound = episodes
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

// downloadCover ensures dir contains a local cover.{jpg,jpeg,png}. If one
// already exists it's left as-is (no network call). Returns whether a local
// cover ended up present.
func downloadCover(dir, posterPath string) bool {
	for _, ext := range coverExts {
		if _, err := os.Stat(filepath.Join(dir, "cover."+ext)); err == nil {
			return true
		}
	}
	if posterPath == "" {
		return false
	}

	resp, err := http.Get(tmdb.ImageURL(posterPath, "original"))
	if err != nil {
		slog.Warn("could not download cover image", "dir", dir, "error", err)
		return false
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		slog.Warn("could not download cover image", "dir", dir, "status", resp.StatusCode)
		return false
	}

	out, err := os.Create(filepath.Join(dir, "cover.jpg"))
	if err != nil {
		slog.Warn("could not write cover image", "dir", dir, "error", err)
		return false
	}
	defer out.Close()

	if _, err := io.Copy(out, resp.Body); err != nil {
		slog.Warn("could not write cover image", "dir", dir, "error", err)
		return false
	}
	return true
}

func (s *Scanner) scan(libraryPath string) (moviesFound, showsFound, episodesFound int, err error) {
	moviesPath := filepath.Join(libraryPath, "movies")
	tvPath := filepath.Join(libraryPath, "tvshows")

	if entries, readErr := os.ReadDir(moviesPath); readErr == nil {
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			added, scanErr := s.scanMovie(moviesPath, entry.Name())
			if scanErr != nil {
				slog.Warn("skipping movie folder", "folder", entry.Name(), "error", scanErr)
				continue
			}
			if added {
				moviesFound++
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
			isNewShow, n, scanErr := s.scanTVShow(tvPath, entry.Name())
			if scanErr != nil {
				slog.Warn("skipping tv show folder", "folder", entry.Name(), "error", scanErr)
				continue
			}
			if isNewShow {
				showsFound++
			}
			episodesFound += n
		}
	} else {
		slog.Warn("could not read tvshows library path", "path", tvPath, "error", readErr)
	}

	return moviesFound, showsFound, episodesFound, nil
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
	matches := folderRegex.FindStringSubmatch(folderName)
	if matches == nil {
		return false, fmt.Errorf("folder name %q does not match 'Title (Year)'", folderName)
	}
	title, year := matches[1], matches[2]
	yearNum, _ := strconv.Atoi(year)

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

	return true, db.DB.Create(&movie).Error
}

// scanTVShow returns whether the show itself is new, plus the number of new
// episodes found (which can be > 0 even for an already-known show).
func (s *Scanner) scanTVShow(tvRoot, folderName string) (bool, int, error) {
	matches := folderRegex.FindStringSubmatch(folderName)
	if matches == nil {
		return false, 0, fmt.Errorf("folder name %q does not match 'Title (Year)'", folderName)
	}
	title, year := matches[1], matches[2]
	yearNum, _ := strconv.Atoi(year)

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
		}

		if err := db.DB.Create(&episode).Error; err != nil {
			return count, err
		}
		count++
	}

	return count, nil
}
