package api

import (
	"errors"
	"net/http"

	"github.com/TeluTrix/seahorse/internal/auth"
	"github.com/TeluTrix/seahorse/internal/db"
	"github.com/TeluTrix/seahorse/internal/models"
	"github.com/TeluTrix/seahorse/internal/progress"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"gorm.io/gorm"
)

func (h *Handlers) ListMovies(w http.ResponseWriter, r *http.Request) {
	var movies []models.Movie
	if err := db.DB.Order("title").Find(&movies).Error; err != nil {
		writeError(w, http.StatusInternalServerError, "could not load movies")
		return
	}

	dtos := make([]MovieDTO, 0, len(movies))
	for _, m := range movies {
		dtos = append(dtos, toMovieDTO(m, nil))
	}
	writeJSON(w, http.StatusOK, dtos)
}

func (h *Handlers) GetMovie(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(mux.Vars(r)["id"])
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid movie id")
		return
	}

	var movie models.Movie
	if err := db.DB.First(&movie, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			writeError(w, http.StatusNotFound, "movie not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "could not load movie")
		return
	}

	var wp *models.WatchProgress
	if userID, ok := auth.UserIDFromContext(r.Context()); ok {
		wp, err = progress.Get(userID, models.MediaTypeMovie, movie.ID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "could not load progress")
			return
		}
	}

	writeJSON(w, http.StatusOK, toMovieDTO(movie, wp))
}

func (h *Handlers) ListTVShows(w http.ResponseWriter, r *http.Request) {
	var shows []models.TVShow
	if err := db.DB.Order("title").Find(&shows).Error; err != nil {
		writeError(w, http.StatusInternalServerError, "could not load tv shows")
		return
	}

	dtos := make([]TVShowDTO, 0, len(shows))
	for _, s := range shows {
		dtos = append(dtos, toTVShowDTO(s, nil))
	}
	writeJSON(w, http.StatusOK, dtos)
}

func (h *Handlers) GetTVShow(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(mux.Vars(r)["id"])
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid tv show id")
		return
	}

	var show models.TVShow
	query := db.DB.Preload("Seasons", func(tx *gorm.DB) *gorm.DB {
		return tx.Order("season_number")
	}).Preload("Seasons.Episodes", func(tx *gorm.DB) *gorm.DB {
		return tx.Order("episode_number")
	})

	if err := query.First(&show, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			writeError(w, http.StatusNotFound, "tv show not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "could not load tv show")
		return
	}

	var progressByEpisode map[uuid.UUID]models.WatchProgress
	if userID, ok := auth.UserIDFromContext(r.Context()); ok {
		episodeIDs := make([]uuid.UUID, 0)
		for _, s := range show.Seasons {
			for _, e := range s.Episodes {
				episodeIDs = append(episodeIDs, e.ID)
			}
		}
		progressByEpisode, err = progress.GetMany(userID, models.MediaTypeEpisode, episodeIDs)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "could not load progress")
			return
		}
	}

	writeJSON(w, http.StatusOK, toTVShowDTO(show, progressByEpisode))
}
