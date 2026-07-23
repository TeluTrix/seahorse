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
	page, pageSize := parsePagination(r)
	order := "title"
	if r.URL.Query().Get("sort") == "newest" {
		order = "release_date DESC"
	}

	var total int64
	if err := db.DB.Model(&models.Movie{}).Count(&total).Error; err != nil {
		writeError(w, http.StatusInternalServerError, "could not load movies")
		return
	}

	var movies []models.Movie
	if err := db.DB.Order(order).Offset((page - 1) * pageSize).Limit(pageSize).Find(&movies).Error; err != nil {
		writeError(w, http.StatusInternalServerError, "could not load movies")
		return
	}

	progressByMovie, err := progressForMovies(r, movies)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not load progress")
		return
	}

	dtos := make([]MovieDTO, 0, len(movies))
	for _, m := range movies {
		var wp *models.WatchProgress
		if p, ok := progressByMovie[m.ID]; ok {
			wp = &p
		}
		dtos = append(dtos, toMovieDTO(m, wp, false))
	}
	writeJSON(w, http.StatusOK, MoviesPageDTO{Movies: dtos, Page: page, PageSize: pageSize, Total: total})
}

// progressForMovies batch-loads watch progress for the given movies under
// the authenticated user (empty map, no error, if unauthenticated).
func progressForMovies(r *http.Request, movies []models.Movie) (map[uuid.UUID]models.WatchProgress, error) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		return map[uuid.UUID]models.WatchProgress{}, nil
	}
	ids := make([]uuid.UUID, 0, len(movies))
	for _, m := range movies {
		ids = append(ids, m.ID)
	}
	return progress.GetMany(userID, models.MediaTypeMovie, ids)
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

	writeJSON(w, http.StatusOK, toMovieDTO(movie, wp, true))
}

func (h *Handlers) ListTVShows(w http.ResponseWriter, r *http.Request) {
	page, pageSize := parsePagination(r)
	order := "title"
	if r.URL.Query().Get("sort") == "newest" {
		order = "first_air_date DESC"
	}

	var total int64
	if err := db.DB.Model(&models.TVShow{}).Count(&total).Error; err != nil {
		writeError(w, http.StatusInternalServerError, "could not load tv shows")
		return
	}

	var shows []models.TVShow
	if err := db.DB.Order(order).Offset((page - 1) * pageSize).Limit(pageSize).Find(&shows).Error; err != nil {
		writeError(w, http.StatusInternalServerError, "could not load tv shows")
		return
	}

	dtos := make([]TVShowDTO, 0, len(shows))
	for _, s := range shows {
		dtos = append(dtos, toTVShowDTO(s, nil, false))
	}
	writeJSON(w, http.StatusOK, TVShowsPageDTO{TVShows: dtos, Page: page, PageSize: pageSize, Total: total})
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

	writeJSON(w, http.StatusOK, toTVShowDTO(show, progressByEpisode, true))
}
