package api

import (
	"fmt"

	"github.com/TeluTrix/seahorse/internal/models"
	"github.com/TeluTrix/seahorse/internal/tmdb"
	"github.com/google/uuid"
)

type PublicUser struct {
	UserID    uuid.UUID   `json:"user_id"`
	UserEmail string      `json:"user_email"`
	UserRole  models.Role `json:"user_role"`
}

func toPublicUser(u models.User) PublicUser {
	return PublicUser{
		UserID:    u.UserID,
		UserEmail: u.UserEmail,
		UserRole:  u.UserRole,
	}
}

type ProgressDTO struct {
	PositionSeconds float64 `json:"position_seconds"`
	DurationSeconds float64 `json:"duration_seconds"`
	Completed       bool    `json:"completed"`
}

func toProgressDTO(wp *models.WatchProgress) *ProgressDTO {
	if wp == nil {
		return nil
	}
	return &ProgressDTO{
		PositionSeconds: wp.PositionSeconds,
		DurationSeconds: wp.DurationSeconds,
		Completed:       wp.Completed,
	}
}

func moviePosterURL(m models.Movie) string {
	if m.CoverCached {
		return fmt.Sprintf("/api/images/movies/%s/cover", m.ID)
	}
	return tmdb.ImageURL(m.PosterPath, "w500")
}

func tvShowPosterURL(t models.TVShow) string {
	if t.CoverCached {
		return fmt.Sprintf("/api/images/tvshows/%s/cover", t.ID)
	}
	return tmdb.ImageURL(t.PosterPath, "w500")
}

type MovieDTO struct {
	ID            uuid.UUID    `json:"id"`
	Title         string       `json:"title"`
	Overview      string       `json:"overview"`
	PosterURL     string       `json:"poster_url"`
	BackdropURL   string       `json:"backdrop_url"`
	HasLocalCover bool         `json:"has_local_cover"`
	ReleaseDate   string       `json:"release_date"`
	VoteAverage   float64      `json:"vote_average"`
	Genres        string       `json:"genres"`
	Progress      *ProgressDTO `json:"progress,omitempty"`
}

func toMovieDTO(m models.Movie, wp *models.WatchProgress) MovieDTO {
	return MovieDTO{
		ID:            m.ID,
		Title:         m.Title,
		Overview:      m.Overview,
		PosterURL:     moviePosterURL(m),
		BackdropURL:   tmdb.ImageURL(m.BackdropPath, "w1280"),
		HasLocalCover: m.CoverCached,
		ReleaseDate:   m.ReleaseDate,
		VoteAverage:   m.VoteAverage,
		Genres:        m.Genres,
		Progress:      toProgressDTO(wp),
	}
}

type EpisodeDTO struct {
	ID            uuid.UUID    `json:"id"`
	EpisodeNumber int          `json:"episode_number"`
	Title         string       `json:"title"`
	Overview      string       `json:"overview"`
	StillURL      string       `json:"still_url"`
	Progress      *ProgressDTO `json:"progress,omitempty"`
}

func toEpisodeDTO(e models.Episode, wp *models.WatchProgress) EpisodeDTO {
	return EpisodeDTO{
		ID:            e.ID,
		EpisodeNumber: e.EpisodeNumber,
		Title:         e.Title,
		Overview:      e.Overview,
		StillURL:      tmdb.ImageURL(e.StillPath, "w300"),
		Progress:      toProgressDTO(wp),
	}
}

type SeasonDTO struct {
	ID           uuid.UUID    `json:"id"`
	SeasonNumber int          `json:"season_number"`
	Episodes     []EpisodeDTO `json:"episodes"`
}

func toSeasonDTO(s models.Season, progressByEpisode map[uuid.UUID]models.WatchProgress) SeasonDTO {
	episodes := make([]EpisodeDTO, 0, len(s.Episodes))
	for _, e := range s.Episodes {
		var wp *models.WatchProgress
		if p, ok := progressByEpisode[e.ID]; ok {
			wp = &p
		}
		episodes = append(episodes, toEpisodeDTO(e, wp))
	}
	return SeasonDTO{ID: s.ID, SeasonNumber: s.SeasonNumber, Episodes: episodes}
}

type TVShowDTO struct {
	ID            uuid.UUID   `json:"id"`
	Title         string      `json:"title"`
	Overview      string      `json:"overview"`
	PosterURL     string      `json:"poster_url"`
	BackdropURL   string      `json:"backdrop_url"`
	HasLocalCover bool        `json:"has_local_cover"`
	FirstAirDate  string      `json:"first_air_date"`
	VoteAverage   float64     `json:"vote_average"`
	Genres        string      `json:"genres"`
	Seasons       []SeasonDTO `json:"seasons,omitempty"`
}

func toTVShowDTO(t models.TVShow, progressByEpisode map[uuid.UUID]models.WatchProgress) TVShowDTO {
	seasons := make([]SeasonDTO, 0, len(t.Seasons))
	for _, s := range t.Seasons {
		seasons = append(seasons, toSeasonDTO(s, progressByEpisode))
	}
	return TVShowDTO{
		ID:            t.ID,
		Title:         t.Title,
		Overview:      t.Overview,
		PosterURL:     tvShowPosterURL(t),
		BackdropURL:   tmdb.ImageURL(t.BackdropPath, "w1280"),
		HasLocalCover: t.CoverCached,
		FirstAirDate:  t.FirstAirDate,
		VoteAverage:   t.VoteAverage,
		Genres:        t.Genres,
		Seasons:       seasons,
	}
}

type MoviesPageDTO struct {
	Movies   []MovieDTO `json:"movies"`
	Page     int        `json:"page"`
	PageSize int        `json:"page_size"`
	Total    int64      `json:"total"`
}

type TVShowsPageDTO struct {
	TVShows  []TVShowDTO `json:"tv_shows"`
	Page     int         `json:"page"`
	PageSize int         `json:"page_size"`
	Total    int64       `json:"total"`
}

type SearchResultDTO struct {
	Movies       []MovieDTO  `json:"movies"`
	MoviesTotal  int64       `json:"movies_total"`
	TVShows      []TVShowDTO `json:"tv_shows"`
	TVShowsTotal int64       `json:"tv_shows_total"`
	Page         int         `json:"page"`
	PageSize     int         `json:"page_size"`
}
