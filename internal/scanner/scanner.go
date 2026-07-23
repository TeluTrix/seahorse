package scanner

import (
	"errors"
	"fmt"
	"log/slog"
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

func (s *Scanner) StartScan(libraryPath string) error {
	s.mu.Lock()
	if s.status.State == "running" {
		s.mu.Unlock()
		return ErrScanInProgress
	}
	s.status = Status{State: "running", StartedAt: time.Now()}
	s.mu.Unlock()

	go s.run(libraryPath)
	return nil
}

func (s *Scanner) run(libraryPath string) {
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

func (s *Scanner) scan(libraryPath string) (moviesFound, showsFound, episodesFound int, err error) {
	moviesPath := filepath.Join(libraryPath, "movies")
	tvPath := filepath.Join(libraryPath, "tvshows")

	if entries, readErr := os.ReadDir(moviesPath); readErr == nil {
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			if scanErr := s.scanMovie(moviesPath, entry.Name()); scanErr != nil {
				slog.Warn("skipping movie folder", "folder", entry.Name(), "error", scanErr)
				continue
			}
			moviesFound++
		}
	} else {
		slog.Warn("could not read movies library path", "path", moviesPath, "error", readErr)
	}

	if entries, readErr := os.ReadDir(tvPath); readErr == nil {
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			n, scanErr := s.scanTVShow(tvPath, entry.Name())
			if scanErr != nil {
				slog.Warn("skipping tv show folder", "folder", entry.Name(), "error", scanErr)
				continue
			}
			showsFound++
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

func (s *Scanner) scanMovie(moviesRoot, folderName string) error {
	matches := folderRegex.FindStringSubmatch(folderName)
	if matches == nil {
		return fmt.Errorf("folder name %q does not match 'Title (Year)'", folderName)
	}
	title, year := matches[1], matches[2]
	yearNum, _ := strconv.Atoi(year)

	videoFile, err := findVideoFile(filepath.Join(moviesRoot, folderName))
	if err != nil {
		return err
	}

	meta, err := s.tmdb.SearchMovie(title, yearNum)
	if err != nil {
		return err
	}

	var existing models.Movie
	result := db.DB.Where("file_path = ?", videoFile).First(&existing)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		existing = models.Movie{ID: uuid.New(), FilePath: videoFile}
	} else if result.Error != nil {
		return result.Error
	}

	existing.TMDBID = meta.TMDBID
	existing.Title = meta.Title
	existing.Overview = meta.Overview
	existing.PosterPath = meta.PosterPath
	existing.BackdropPath = meta.BackdropPath
	existing.ReleaseDate = meta.ReleaseDate
	existing.VoteAverage = meta.VoteAverage
	existing.Genres = tmdb.JoinGenres(meta.Genres)

	return db.DB.Save(&existing).Error
}

func (s *Scanner) scanTVShow(tvRoot, folderName string) (int, error) {
	matches := folderRegex.FindStringSubmatch(folderName)
	if matches == nil {
		return 0, fmt.Errorf("folder name %q does not match 'Title (Year)'", folderName)
	}
	title, year := matches[1], matches[2]
	yearNum, _ := strconv.Atoi(year)

	showFolder := filepath.Join(tvRoot, folderName)

	meta, err := s.tmdb.SearchTV(title, yearNum)
	if err != nil {
		return 0, err
	}

	var show models.TVShow
	result := db.DB.Where("folder_path = ?", showFolder).First(&show)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		show = models.TVShow{ID: uuid.New(), FolderPath: showFolder}
	} else if result.Error != nil {
		return 0, result.Error
	}

	show.TMDBID = meta.TMDBID
	show.Title = meta.Title
	show.Overview = meta.Overview
	show.PosterPath = meta.PosterPath
	show.BackdropPath = meta.BackdropPath
	show.FirstAirDate = meta.FirstAirDate
	show.VoteAverage = meta.VoteAverage
	show.Genres = tmdb.JoinGenres(meta.Genres)

	if err := db.DB.Save(&show).Error; err != nil {
		return 0, err
	}

	seasonEntries, err := os.ReadDir(showFolder)
	if err != nil {
		return 0, err
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

	return episodeCount, nil
}

func (s *Scanner) scanSeason(show models.TVShow, seasonPath string, seasonNumber int) (int, error) {
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

	entries, err := os.ReadDir(seasonPath)
	if err != nil {
		return 0, err
	}

	count := 0
	for _, entry := range entries {
		if entry.IsDir() || !videoExtSet[strings.ToLower(filepath.Ext(entry.Name()))] {
			continue
		}
		epMatch := episodeRegex.FindStringSubmatch(entry.Name())
		if epMatch == nil {
			continue
		}
		episodeNumber, _ := strconv.Atoi(epMatch[2])
		filePath := filepath.Join(seasonPath, entry.Name())

		var episode models.Episode
		result := db.DB.Where("file_path = ?", filePath).First(&episode)
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			episode = models.Episode{ID: uuid.New(), SeasonID: season.ID, FilePath: filePath}
		} else if result.Error != nil {
			return count, result.Error
		}

		episode.EpisodeNumber = episodeNumber
		episode.Title = fmt.Sprintf("Episode %d", episodeNumber)
		if meta, found := tmdb.FindEpisode(episodeMeta, episodeNumber); found {
			episode.Title = meta.Title
			episode.Overview = meta.Overview
			episode.StillPath = meta.StillPath
		}

		if err := db.DB.Save(&episode).Error; err != nil {
			return count, err
		}
		count++
	}

	return count, nil
}
