package api

import (
	"errors"
	"net/http"

	"github.com/TeluTrix/seahorse/internal/db"
	"github.com/TeluTrix/seahorse/internal/models"
	"github.com/TeluTrix/seahorse/internal/tmdb"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"gorm.io/gorm"
)

// EpisodeContextDTO is enough about an episode's place in its show for the
// player's breadcrumb trail ("TV Shows / {show} / S{n}E{n} · {title}") —
// the player itself only ever receives an episode id, not its show/season.
type EpisodeContextDTO struct {
	ID            uuid.UUID `json:"id"`
	Title         string    `json:"title"`
	Overview      string    `json:"overview"`
	EpisodeNumber int       `json:"episode_number"`
	SeasonNumber  int       `json:"season_number"`
	ShowID        uuid.UUID `json:"show_id"`
	ShowTitle     string    `json:"show_title"`
}

// episodeSeasonShow loads an episode along with its season and show, since
// both GetEpisode and NextEpisode need to walk that chain.
func episodeSeasonShow(id uuid.UUID) (models.Episode, models.Season, models.TVShow, error) {
	var episode models.Episode
	if err := db.DB.First(&episode, "id = ?", id).Error; err != nil {
		return episode, models.Season{}, models.TVShow{}, err
	}
	var season models.Season
	if err := db.DB.First(&season, "id = ?", episode.SeasonID).Error; err != nil {
		return episode, season, models.TVShow{}, err
	}
	var show models.TVShow
	if err := db.DB.First(&show, "id = ?", season.TVShowID).Error; err != nil {
		return episode, season, show, err
	}
	return episode, season, show, nil
}

func (h *Handlers) GetEpisode(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(mux.Vars(r)["id"])
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid episode id")
		return
	}

	episode, season, show, err := episodeSeasonShow(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			writeError(w, http.StatusNotFound, "episode not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "could not load episode")
		return
	}

	writeJSON(w, http.StatusOK, EpisodeContextDTO{
		ID:            episode.ID,
		Title:         episode.Title,
		Overview:      episode.Overview,
		EpisodeNumber: episode.EpisodeNumber,
		SeasonNumber:  season.SeasonNumber,
		ShowID:        show.ID,
		ShowTitle:     show.Title,
	})
}

type NextEpisodeDTO struct {
	ID            uuid.UUID `json:"id"`
	Title         string    `json:"title"`
	EpisodeNumber int       `json:"episode_number"`
	SeasonNumber  int       `json:"season_number"`
	StillURL      string    `json:"still_url"`
}

// NextEpisode returns whichever episode plays right after {id}: the next
// episode number in the same season, or failing that the first episode of
// the next season. 404 if {id} is the show's last episode.
func (h *Handlers) NextEpisode(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(mux.Vars(r)["id"])
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid episode id")
		return
	}

	current, season, _, err := episodeSeasonShow(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			writeError(w, http.StatusNotFound, "episode not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "could not load episode")
		return
	}

	var next models.Episode
	nextSeasonNumber := season.SeasonNumber

	err = db.DB.Where("season_id = ? AND episode_number > ?", season.ID, current.EpisodeNumber).
		Order("episode_number").First(&next).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		var nextSeason models.Season
		err = db.DB.Where("tv_show_id = ? AND season_number > ?", season.TVShowID, season.SeasonNumber).
			Order("season_number").First(&nextSeason).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			writeError(w, http.StatusNotFound, "no next episode")
			return
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, "could not load next season")
			return
		}
		nextSeasonNumber = nextSeason.SeasonNumber
		err = db.DB.Where("season_id = ?", nextSeason.ID).Order("episode_number").First(&next).Error
	}

	if errors.Is(err, gorm.ErrRecordNotFound) {
		writeError(w, http.StatusNotFound, "no next episode")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not load next episode")
		return
	}

	writeJSON(w, http.StatusOK, NextEpisodeDTO{
		ID:            next.ID,
		Title:         next.Title,
		EpisodeNumber: next.EpisodeNumber,
		SeasonNumber:  nextSeasonNumber,
		StillURL:      tmdb.ImageURL(next.StillPath, "w300"),
	})
}
