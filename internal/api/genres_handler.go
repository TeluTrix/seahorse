package api

import (
	"net/http"
	"sort"
	"strings"

	"github.com/TeluTrix/seahorse/internal/db"
	"github.com/TeluTrix/seahorse/internal/models"
)

// ListGenres returns the distinct genre names actually present across all
// movies and tv shows, so the frontend's filter dropdown always reflects
// real library content instead of a hardcoded/guessed list.
func (h *Handlers) ListGenres(w http.ResponseWriter, r *http.Request) {
	var movieGenres []string
	if err := db.DB.Model(&models.Movie{}).Pluck("genres", &movieGenres).Error; err != nil {
		writeError(w, http.StatusInternalServerError, "could not load genres")
		return
	}
	var showGenres []string
	if err := db.DB.Model(&models.TVShow{}).Pluck("genres", &showGenres).Error; err != nil {
		writeError(w, http.StatusInternalServerError, "could not load genres")
		return
	}

	set := map[string]bool{}
	for _, joined := range append(movieGenres, showGenres...) {
		for _, part := range strings.Split(joined, ",") {
			part = strings.TrimSpace(part)
			if part != "" {
				set[part] = true
			}
		}
	}

	genres := make([]string, 0, len(set))
	for g := range set {
		genres = append(genres, g)
	}
	sort.Strings(genres)

	writeJSON(w, http.StatusOK, genres)
}
