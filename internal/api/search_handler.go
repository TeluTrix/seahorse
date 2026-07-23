package api

import (
	"net/http"

	"github.com/TeluTrix/seahorse/internal/db"
	"github.com/TeluTrix/seahorse/internal/models"
)

// Search backs both the combined nav search (movies + tv shows together) and
// the type-scoped filter bar on the Movies/TVShows overview pages — pass
// ?type=movies or ?type=tvshows to skip querying the other type entirely
// rather than fetching and discarding it.
func (h *Handlers) Search(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	year := r.URL.Query().Get("year")
	genre := r.URL.Query().Get("genre")
	mediaType := r.URL.Query().Get("type")
	page, pageSize := h.parsePagination(r)

	var movieDTOs []MovieDTO
	var moviesTotal int64
	if mediaType != "tvshows" {
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

		if err := movieQuery.Count(&moviesTotal).Error; err != nil {
			writeError(w, http.StatusInternalServerError, "could not search movies")
			return
		}
		var movies []models.Movie
		if err := movieQuery.Order("title").Offset((page - 1) * pageSize).Limit(pageSize).Find(&movies).Error; err != nil {
			writeError(w, http.StatusInternalServerError, "could not search movies")
			return
		}
		progressByMovie, err := progressForMovies(r, movies)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "could not load progress")
			return
		}
		movieDTOs = make([]MovieDTO, 0, len(movies))
		for _, m := range movies {
			var wp *models.WatchProgress
			if p, ok := progressByMovie[m.ID]; ok {
				wp = &p
			}
			movieDTOs = append(movieDTOs, toMovieDTO(m, wp, false))
		}
	}

	var showDTOs []TVShowDTO
	var showsTotal int64
	if mediaType != "movies" {
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

		if err := showQuery.Count(&showsTotal).Error; err != nil {
			writeError(w, http.StatusInternalServerError, "could not search tv shows")
			return
		}
		var shows []models.TVShow
		if err := showQuery.Order("title").Offset((page - 1) * pageSize).Limit(pageSize).Find(&shows).Error; err != nil {
			writeError(w, http.StatusInternalServerError, "could not search tv shows")
			return
		}
		showDTOs = make([]TVShowDTO, 0, len(shows))
		for _, s := range shows {
			showDTOs = append(showDTOs, toTVShowDTO(s, nil, false))
		}
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
