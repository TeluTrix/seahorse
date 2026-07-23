package api

import (
	"encoding/json"
	"fmt"
	"time"

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
	PositionSeconds float64   `json:"position_seconds"`
	DurationSeconds float64   `json:"duration_seconds"`
	Completed       bool      `json:"completed"`
	UpdatedAt       time.Time `json:"updated_at"`
}

func toProgressDTO(wp *models.WatchProgress) *ProgressDTO {
	if wp == nil {
		return nil
	}
	return &ProgressDTO{
		PositionSeconds: wp.PositionSeconds,
		DurationSeconds: wp.DurationSeconds,
		Completed:       wp.Completed,
		UpdatedAt:       wp.UpdatedAt,
	}
}

// remuxStatusLookup resolves a movie/episode ID to its current audio-remux
// state ("pending", "active", or "" if none) — see scanner.Scanner.RemuxState.
// Passed as a function rather than importing the scanner package directly,
// and left as noRemuxStatus for list/search results where it isn't shown.
type remuxStatusLookup func(uuid.UUID) string

func noRemuxStatus(uuid.UUID) string { return "" }

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

type CastMemberDTO struct {
	Name       string `json:"name"`
	Character  string `json:"character"`
	ProfileURL string `json:"profile_url,omitempty"`
}

// decodeCast parses a JSON-encoded []tmdb.CastMember column into DTOs with
// resolved image URLs. Malformed/empty input just yields no cast members
// rather than an error — cast is supplementary metadata, never essential.
func decodeCast(raw string) []CastMemberDTO {
	if raw == "" {
		return nil
	}
	var members []tmdb.CastMember
	if err := json.Unmarshal([]byte(raw), &members); err != nil {
		return nil
	}
	dtos := make([]CastMemberDTO, 0, len(members))
	for _, m := range members {
		dtos = append(dtos, CastMemberDTO{
			Name:       m.Name,
			Character:  m.Character,
			ProfileURL: tmdb.ImageURL(m.ProfilePath, "w185"),
		})
	}
	return dtos
}

type MovieDTO struct {
	ID            uuid.UUID       `json:"id"`
	Title         string          `json:"title"`
	Overview      string          `json:"overview"`
	PosterURL     string          `json:"poster_url"`
	BackdropURL   string          `json:"backdrop_url"`
	HasLocalCover bool            `json:"has_local_cover"`
	ReleaseDate   string          `json:"release_date"`
	VoteAverage   float64         `json:"vote_average"`
	Genres        string          `json:"genres"`
	Runtime       int             `json:"runtime_minutes,omitempty"`
	Director      string          `json:"director,omitempty"`
	Cast          []CastMemberDTO `json:"cast,omitempty"`
	Progress      *ProgressDTO    `json:"progress,omitempty"`
	RemuxStatus   string          `json:"remux_status,omitempty"`
}

// toMovieDTO builds the movie DTO. includeCast is false for list/search
// results (cast isn't rendered on poster cards, so there's no reason to
// bloat those payloads with it) and true for the single-movie detail view.
func toMovieDTO(m models.Movie, wp *models.WatchProgress, includeCast bool, remuxStatus remuxStatusLookup) MovieDTO {
	dto := MovieDTO{
		ID:            m.ID,
		Title:         m.Title,
		Overview:      m.Overview,
		PosterURL:     moviePosterURL(m),
		BackdropURL:   tmdb.ImageURL(m.BackdropPath, "w1280"),
		HasLocalCover: m.CoverCached,
		ReleaseDate:   m.ReleaseDate,
		VoteAverage:   m.VoteAverage,
		Genres:        m.Genres,
		Runtime:       m.Runtime,
		Director:      m.Director,
		Progress:      toProgressDTO(wp),
		RemuxStatus:   remuxStatus(m.ID),
	}
	if includeCast {
		dto.Cast = decodeCast(m.Cast)
	}
	return dto
}

type EpisodeDTO struct {
	ID            uuid.UUID    `json:"id"`
	EpisodeNumber int          `json:"episode_number"`
	Title         string       `json:"title"`
	Overview      string       `json:"overview"`
	StillURL      string       `json:"still_url"`
	Runtime       int          `json:"runtime_minutes,omitempty"`
	Progress      *ProgressDTO `json:"progress,omitempty"`
	RemuxStatus   string       `json:"remux_status,omitempty"`
}

func toEpisodeDTO(e models.Episode, wp *models.WatchProgress, remuxStatus remuxStatusLookup) EpisodeDTO {
	return EpisodeDTO{
		ID:            e.ID,
		EpisodeNumber: e.EpisodeNumber,
		Title:         e.Title,
		Overview:      e.Overview,
		StillURL:      tmdb.ImageURL(e.StillPath, "w300"),
		Runtime:       e.Runtime,
		Progress:      toProgressDTO(wp),
		RemuxStatus:   remuxStatus(e.ID),
	}
}

type SeasonDTO struct {
	ID           uuid.UUID    `json:"id"`
	SeasonNumber int          `json:"season_number"`
	Episodes     []EpisodeDTO `json:"episodes"`
}

func toSeasonDTO(s models.Season, progressByEpisode map[uuid.UUID]models.WatchProgress, remuxStatus remuxStatusLookup) SeasonDTO {
	episodes := make([]EpisodeDTO, 0, len(s.Episodes))
	for _, e := range s.Episodes {
		var wp *models.WatchProgress
		if p, ok := progressByEpisode[e.ID]; ok {
			wp = &p
		}
		episodes = append(episodes, toEpisodeDTO(e, wp, remuxStatus))
	}
	return SeasonDTO{ID: s.ID, SeasonNumber: s.SeasonNumber, Episodes: episodes}
}

type TVShowDTO struct {
	ID            uuid.UUID       `json:"id"`
	Title         string          `json:"title"`
	Overview      string          `json:"overview"`
	PosterURL     string          `json:"poster_url"`
	BackdropURL   string          `json:"backdrop_url"`
	HasLocalCover bool            `json:"has_local_cover"`
	FirstAirDate  string          `json:"first_air_date"`
	VoteAverage   float64         `json:"vote_average"`
	Genres        string          `json:"genres"`
	Creators      string          `json:"creators,omitempty"`
	Cast          []CastMemberDTO `json:"cast,omitempty"`
	Seasons       []SeasonDTO     `json:"seasons,omitempty"`
}

// toTVShowDTO builds the show DTO. See toMovieDTO for why includeCast is
// only set for the single-show detail view; remuxStatus is likewise
// noRemuxStatus outside the detail view.
func toTVShowDTO(t models.TVShow, progressByEpisode map[uuid.UUID]models.WatchProgress, includeCast bool, remuxStatus remuxStatusLookup) TVShowDTO {
	seasons := make([]SeasonDTO, 0, len(t.Seasons))
	for _, s := range t.Seasons {
		seasons = append(seasons, toSeasonDTO(s, progressByEpisode, remuxStatus))
	}
	dto := TVShowDTO{
		ID:            t.ID,
		Title:         t.Title,
		Overview:      t.Overview,
		PosterURL:     tvShowPosterURL(t),
		BackdropURL:   tmdb.ImageURL(t.BackdropPath, "w1280"),
		HasLocalCover: t.CoverCached,
		FirstAirDate:  t.FirstAirDate,
		VoteAverage:   t.VoteAverage,
		Genres:        t.Genres,
		Creators:      t.Creators,
		Seasons:       seasons,
	}
	if includeCast {
		dto.Cast = decodeCast(t.Cast)
	}
	return dto
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
