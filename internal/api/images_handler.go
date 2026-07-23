package api

import (
	"errors"
	"net/http"
	"os"
	"path/filepath"

	"github.com/TeluTrix/seahorse/internal/db"
	"github.com/TeluTrix/seahorse/internal/models"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"gorm.io/gorm"
)

// webp checked first: covers cached after the WebP optimization was added
// use it; jpg/jpeg/png remain for backward compatibility with covers cached
// before that.
var coverExts = []string{"webp", "jpg", "jpeg", "png"}

func serveCover(w http.ResponseWriter, r *http.Request, dir string) {
	for _, ext := range coverExts {
		path := filepath.Join(dir, "cover."+ext)
		if _, err := os.Stat(path); err == nil {
			if ext == "webp" {
				// Go's built-in MIME table doesn't reliably know .webp on
				// every system, so set it explicitly (same reasoning as the
				// video Content-Type handling in stream_handler.go).
				w.Header().Set("Content-Type", "image/webp")
			}
			http.ServeFile(w, r, path)
			return
		}
	}
	writeError(w, http.StatusNotFound, "no cover cached for this item")
}

func (h *Handlers) MovieCover(w http.ResponseWriter, r *http.Request) {
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

	serveCover(w, r, filepath.Dir(movie.FilePath))
}

func (h *Handlers) TVShowCover(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(mux.Vars(r)["id"])
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid tv show id")
		return
	}

	var show models.TVShow
	if err := db.DB.First(&show, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			writeError(w, http.StatusNotFound, "tv show not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "could not load tv show")
		return
	}

	serveCover(w, r, show.FolderPath)
}
