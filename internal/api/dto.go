package api

import (
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

type MovieDTO struct {
	ID          uuid.UUID `json:"id"`
	Title       string    `json:"title"`
	Overview    string    `json:"overview"`
	PosterURL   string    `json:"poster_url"`
	BackdropURL string    `json:"backdrop_url"`
	ReleaseDate string    `json:"release_date"`
	VoteAverage float64   `json:"vote_average"`
	Genres      string    `json:"genres"`
}

func toMovieDTO(m models.Movie) MovieDTO {
	return MovieDTO{
		ID:          m.ID,
		Title:       m.Title,
		Overview:    m.Overview,
		PosterURL:   tmdb.ImageURL(m.PosterPath, "w500"),
		BackdropURL: tmdb.ImageURL(m.BackdropPath, "w1280"),
		ReleaseDate: m.ReleaseDate,
		VoteAverage: m.VoteAverage,
		Genres:      m.Genres,
	}
}

type EpisodeDTO struct {
	ID            uuid.UUID `json:"id"`
	EpisodeNumber int       `json:"episode_number"`
	Title         string    `json:"title"`
	Overview      string    `json:"overview"`
	StillURL      string    `json:"still_url"`
}

func toEpisodeDTO(e models.Episode) EpisodeDTO {
	return EpisodeDTO{
		ID:            e.ID,
		EpisodeNumber: e.EpisodeNumber,
		Title:         e.Title,
		Overview:      e.Overview,
		StillURL:      tmdb.ImageURL(e.StillPath, "w300"),
	}
}

type SeasonDTO struct {
	ID           uuid.UUID    `json:"id"`
	SeasonNumber int          `json:"season_number"`
	Episodes     []EpisodeDTO `json:"episodes"`
}

func toSeasonDTO(s models.Season) SeasonDTO {
	episodes := make([]EpisodeDTO, 0, len(s.Episodes))
	for _, e := range s.Episodes {
		episodes = append(episodes, toEpisodeDTO(e))
	}
	return SeasonDTO{ID: s.ID, SeasonNumber: s.SeasonNumber, Episodes: episodes}
}

type TVShowDTO struct {
	ID           uuid.UUID   `json:"id"`
	Title        string      `json:"title"`
	Overview     string      `json:"overview"`
	PosterURL    string      `json:"poster_url"`
	BackdropURL  string      `json:"backdrop_url"`
	FirstAirDate string      `json:"first_air_date"`
	VoteAverage  float64     `json:"vote_average"`
	Genres       string      `json:"genres"`
	Seasons      []SeasonDTO `json:"seasons,omitempty"`
}

func toTVShowDTO(t models.TVShow) TVShowDTO {
	seasons := make([]SeasonDTO, 0, len(t.Seasons))
	for _, s := range t.Seasons {
		seasons = append(seasons, toSeasonDTO(s))
	}
	return TVShowDTO{
		ID:           t.ID,
		Title:        t.Title,
		Overview:     t.Overview,
		PosterURL:    tmdb.ImageURL(t.PosterPath, "w500"),
		BackdropURL:  tmdb.ImageURL(t.BackdropPath, "w1280"),
		FirstAirDate: t.FirstAirDate,
		VoteAverage:  t.VoteAverage,
		Genres:       t.Genres,
		Seasons:      seasons,
	}
}
