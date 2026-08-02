package api

import (
	"errors"
	"net/http"

	"github.com/TeluTrix/seahorse/internal/db"
	"github.com/TeluTrix/seahorse/internal/models"
	"github.com/TeluTrix/seahorse/internal/transcode"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"gorm.io/gorm"
)

// MovieMediaInfo probes a movie's file for technical details (resolution,
// codecs, bitrate, file size) — deliberately not fetched as part of the
// regular movie detail response, since it's a niche, closed-by-default
// panel in the frontend and running ffprobe on every page view for
// something most viewers never open would be wasted work.
func (h *Handlers) MovieMediaInfo(w http.ResponseWriter, r *http.Request) {
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

	info, err := transcode.ProbeMediaInfo(movie.FilePath)
	if err != nil {
		writeError(w, http.StatusNotFound, "media info unavailable")
		return
	}

	writeJSON(w, http.StatusOK, info)
}
