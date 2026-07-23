package api

import (
	"errors"
	"net/http"

	"github.com/TeluTrix/seahorse/internal/db"
	"github.com/TeluTrix/seahorse/internal/models"
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
		dtos = append(dtos, toMovieDTO(m))
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

	writeJSON(w, http.StatusOK, toMovieDTO(movie))
}

func (h *Handlers) ListTVShows(w http.ResponseWriter, r *http.Request) {
	var shows []models.TVShow
	if err := db.DB.Order("title").Find(&shows).Error; err != nil {
		writeError(w, http.StatusInternalServerError, "could not load tv shows")
		return
	}

	dtos := make([]TVShowDTO, 0, len(shows))
	for _, s := range shows {
		dtos = append(dtos, toTVShowDTO(s))
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

	writeJSON(w, http.StatusOK, toTVShowDTO(show))
}
