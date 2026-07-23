package api

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/TeluTrix/seahorse/internal/db"
	"github.com/TeluTrix/seahorse/internal/models"
	"github.com/TeluTrix/seahorse/internal/subtitles"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"gorm.io/gorm"
)

func (h *Handlers) movieFilePath(w http.ResponseWriter, r *http.Request) (string, bool) {
	id, err := uuid.Parse(mux.Vars(r)["id"])
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid movie id")
		return "", false
	}
	var movie models.Movie
	if err := db.DB.First(&movie, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			writeError(w, http.StatusNotFound, "movie not found")
			return "", false
		}
		writeError(w, http.StatusInternalServerError, "could not load movie")
		return "", false
	}
	return movie.FilePath, true
}

func (h *Handlers) episodeFilePath(w http.ResponseWriter, r *http.Request) (string, bool) {
	id, err := uuid.Parse(mux.Vars(r)["id"])
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid episode id")
		return "", false
	}
	var episode models.Episode
	if err := db.DB.First(&episode, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			writeError(w, http.StatusNotFound, "episode not found")
			return "", false
		}
		writeError(w, http.StatusInternalServerError, "could not load episode")
		return "", false
	}
	return episode.FilePath, true
}

func (h *Handlers) MovieSubtitles(w http.ResponseWriter, r *http.Request) {
	filePath, ok := h.movieFilePath(w, r)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, subtitles.Discover(filePath))
}

func (h *Handlers) EpisodeSubtitles(w http.ResponseWriter, r *http.Request) {
	filePath, ok := h.episodeFilePath(w, r)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, subtitles.Discover(filePath))
}

func (h *Handlers) MovieSubtitleVTT(w http.ResponseWriter, r *http.Request) {
	filePath, ok := h.movieFilePath(w, r)
	if !ok {
		return
	}
	serveSubtitleTrack(w, r, filePath)
}

func (h *Handlers) EpisodeSubtitleVTT(w http.ResponseWriter, r *http.Request) {
	filePath, ok := h.episodeFilePath(w, r)
	if !ok {
		return
	}
	serveSubtitleTrack(w, r, filePath)
}

func serveSubtitleTrack(w http.ResponseWriter, r *http.Request, videoPath string) {
	trackID := r.URL.Query().Get("track")

	switch {
	case strings.HasPrefix(trackID, "ext:"):
		// filepath.Base neutralizes any path traversal in the filename.
		filename := filepath.Base(strings.TrimPrefix(trackID, "ext:"))
		path := filepath.Join(filepath.Dir(videoPath), filename)
		data, err := os.ReadFile(path)
		if err != nil {
			writeError(w, http.StatusNotFound, "subtitle file not found")
			return
		}
		alreadyVTT := strings.EqualFold(filepath.Ext(filename), ".vtt")
		w.Header().Set("Content-Type", "text/vtt")
		w.Write(subtitles.SRTToVTT(data, alreadyVTT))

	case strings.HasPrefix(trackID, "embedded-"):
		if !subtitles.Available() {
			writeError(w, http.StatusNotFound, "embedded subtitle extraction is unavailable")
			return
		}
		streamIndex, err := strconv.Atoi(strings.TrimPrefix(trackID, "embedded-"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid track id")
			return
		}

		dir := filepath.Dir(videoPath)
		base := strings.TrimSuffix(filepath.Base(videoPath), filepath.Ext(videoPath))
		cachePath := filepath.Join(dir, fmt.Sprintf("%s.embedded-%d.vtt", base, streamIndex))

		if _, err := os.Stat(cachePath); err != nil {
			if err := subtitles.ExtractToVTT(videoPath, streamIndex, cachePath); err != nil {
				writeError(w, http.StatusInternalServerError, "could not extract subtitle")
				return
			}
		}
		w.Header().Set("Content-Type", "text/vtt")
		http.ServeFile(w, r, cachePath)

	default:
		writeError(w, http.StatusBadRequest, "unknown subtitle track id")
	}
}
