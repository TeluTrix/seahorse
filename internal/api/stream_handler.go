package api

import (
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/TeluTrix/seahorse/internal/db"
	"github.com/TeluTrix/seahorse/internal/models"
	"github.com/TeluTrix/seahorse/internal/transcode"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"gorm.io/gorm"
)

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

var videoMimeTypes = map[string]string{
	".mp4":  "video/mp4",
	".mkv":  "video/x-matroska",
	".webm": "video/webm",
	".mov":  "video/quicktime",
	".avi":  "video/x-msvideo",
}

func serveVideoFile(w http.ResponseWriter, r *http.Request, filePath string, opts transcode.Options) {
	// An explicit "?track=" means the viewer picked a specific audio
	// language out of a multi-audio-track file (see
	// transcode.EnsureAudioTrack for why this has to mean "serve a
	// different, single-track file" rather than a client-side track
	// switch) — lazily prepared here, on first request for that track.
	// Absent, this is a completely ordinary request: same fast path as
	// before, no new probing cost for the overwhelmingly common case of a
	// file with zero or one audio stream.
	if trackParam := r.URL.Query().Get("track"); trackParam != "" {
		streamIndex, err := strconv.Atoi(trackParam)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid track parameter")
			return
		}
		if err := transcode.EnsureAudioTrack(filePath, streamIndex, opts); err != nil {
			writeError(w, http.StatusInternalServerError, "could not prepare requested audio track")
			return
		}
		filePath = transcode.AudioTrackPath(filePath, streamIndex)
	} else if remuxed := transcode.RemuxedPath(filePath); fileExists(remuxed) {
		// Prefer a cached audio-remuxed copy over the original, when the
		// scanner found the original's (single) audio codec incompatible
		// with browser playback (see internal/transcode) — resolved at
		// request time, same pattern as cover/subtitle caching, no DB
		// column needed.
		filePath = remuxed
	}

	file, err := os.Open(filePath)
	if err != nil {
		writeError(w, http.StatusNotFound, "media file not found on disk")
		return
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not read media file")
		return
	}

	if mimeType, ok := videoMimeTypes[strings.ToLower(filepath.Ext(filePath))]; ok {
		w.Header().Set("Content-Type", mimeType)
	}

	http.ServeContent(w, r, filePath, info.ModTime(), file)
}

func (h *Handlers) StreamMovie(w http.ResponseWriter, r *http.Request) {
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

	serveVideoFile(w, r, movie.FilePath, h.Scanner.TranscodeOptions())
}

func (h *Handlers) StreamEpisode(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(mux.Vars(r)["id"])
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid episode id")
		return
	}

	var episode models.Episode
	if err := db.DB.First(&episode, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			writeError(w, http.StatusNotFound, "episode not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "could not load episode")
		return
	}

	serveVideoFile(w, r, episode.FilePath, h.Scanner.TranscodeOptions())
}
