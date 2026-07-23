package api

import (
	"encoding/json"
	"net/http"

	"github.com/TeluTrix/seahorse/internal/auth"
	"github.com/TeluTrix/seahorse/internal/models"
	"github.com/TeluTrix/seahorse/internal/progress"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
)

type saveProgressRequest struct {
	MediaType       models.MediaType `json:"media_type"`
	MediaID         uuid.UUID        `json:"media_id"`
	PositionSeconds float64          `json:"position_seconds"`
	DurationSeconds float64          `json:"duration_seconds"`
}

func parseMediaType(raw string) (models.MediaType, bool) {
	switch models.MediaType(raw) {
	case models.MediaTypeMovie, models.MediaTypeEpisode:
		return models.MediaType(raw), true
	default:
		return "", false
	}
}

func (h *Handlers) SaveProgress(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "not authenticated")
		return
	}

	var req saveProgressRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if _, ok := parseMediaType(string(req.MediaType)); !ok {
		writeError(w, http.StatusBadRequest, "media_type must be movie or episode")
		return
	}

	wp, err := progress.Upsert(userID, req.MediaType, req.MediaID, req.PositionSeconds, req.DurationSeconds)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not save progress")
		return
	}

	writeJSON(w, http.StatusOK, toProgressDTO(&wp))
}

func (h *Handlers) GetProgress(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "not authenticated")
		return
	}

	mediaType, ok := parseMediaType(mux.Vars(r)["mediaType"])
	if !ok {
		writeError(w, http.StatusBadRequest, "media type must be movie or episode")
		return
	}
	mediaID, err := uuid.Parse(mux.Vars(r)["mediaId"])
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid media id")
		return
	}

	wp, err := progress.Get(userID, mediaType, mediaID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not load progress")
		return
	}
	if wp == nil {
		writeError(w, http.StatusNotFound, "no progress found")
		return
	}

	writeJSON(w, http.StatusOK, toProgressDTO(wp))
}
