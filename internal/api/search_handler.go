package api

import (
	"net/http"

	"github.com/TeluTrix/seahorse/internal/db"
	"github.com/TeluTrix/seahorse/internal/models"
)

func (h *Handlers) Search(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	year := r.URL.Query().Get("year")
	genre := r.URL.Query().Get("genre")
	page, pageSize := parsePagination(r)

	movieQuery := db.DB.Model(&models.Movie{})
	if q != "" {
		movieQuery = movieQuery.Where("title LIKE ?", "%"+q+"%")
	}
	if year != "" {
		movieQuery = movieQuery.Where("release_date LIKE ?", year+"%")
	}
	if genre != "" {
		movieQuery = movieQuery.Where("genres LIKE ?", "%"+genre+"%")
	}

	var moviesTotal int64
	if err := movieQuery.Count(&moviesTotal).Error; err != nil {
		writeError(w, http.StatusInternalServerError, "could not search movies")
		return
	}
	var movies []models.Movie
	if err := movieQuery.Order("title").Offset((page - 1) * pageSize).Limit(pageSize).Find(&movies).Error; err != nil {
		writeError(w, http.StatusInternalServerError, "could not search movies")
		return
	}

	showQuery := db.DB.Model(&models.TVShow{})
	if q != "" {
		showQuery = showQuery.Where("title LIKE ?", "%"+q+"%")
	}
	if year != "" {
		showQuery = showQuery.Where("first_air_date LIKE ?", year+"%")
	}
	if genre != "" {
		showQuery = showQuery.Where("genres LIKE ?", "%"+genre+"%")
	}

	var showsTotal int64
	if err := showQuery.Count(&showsTotal).Error; err != nil {
		writeError(w, http.StatusInternalServerError, "could not search tv shows")
		return
	}
	var shows []models.TVShow
	if err := showQuery.Order("title").Offset((page - 1) * pageSize).Limit(pageSize).Find(&shows).Error; err != nil {
		writeError(w, http.StatusInternalServerError, "could not search tv shows")
		return
	}

	movieDTOs := make([]MovieDTO, 0, len(movies))
	for _, m := range movies {
		movieDTOs = append(movieDTOs, toMovieDTO(m, nil))
	}
	showDTOs := make([]TVShowDTO, 0, len(shows))
	for _, s := range shows {
		showDTOs = append(showDTOs, toTVShowDTO(s, nil))
	}

	writeJSON(w, http.StatusOK, SearchResultDTO{
		Movies:       movieDTOs,
		MoviesTotal:  moviesTotal,
		TVShows:      showDTOs,
		TVShowsTotal: showsTotal,
		Page:         page,
		PageSize:     pageSize,
	})
}
