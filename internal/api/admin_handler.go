package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/TeluTrix/seahorse/internal/scanner"
	"github.com/TeluTrix/seahorse/internal/user"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
)

func (h *Handlers) ScanLibrary(w http.ResponseWriter, r *http.Request) {
	full := r.URL.Query().Get("mode") == "full"

	err := h.Scanner.StartScan(h.LibraryPath, full)
	if err != nil {
		if errors.Is(err, scanner.ErrScanInProgress) {
			writeError(w, http.StatusConflict, "a scan is already running")
			return
		}
		writeError(w, http.StatusInternalServerError, "could not start scan")
		return
	}

	writeJSON(w, http.StatusAccepted, h.Scanner.Status())
}

func (h *Handlers) ScanStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, h.Scanner.Status())
}

func (h *Handlers) ListUsers(w http.ResponseWriter, r *http.Request) {
	users, err := user.ListAll()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not load users")
		return
	}

	dtos := make([]PublicUser, 0, len(users))
	for _, u := range users {
		dtos = append(dtos, toPublicUser(u))
	}
	writeJSON(w, http.StatusOK, dtos)
}

type setUserPasswordRequest struct {
	NewPassword string `json:"new_password"`
}

func (h *Handlers) SetUserPassword(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(mux.Vars(r)["id"])
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid user id")
		return
	}

	var req setUserPasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if len(req.NewPassword) < 8 {
		writeError(w, http.StatusBadRequest, "password must be at least 8 characters")
		return
	}

	if err := user.SetPassword(id, req.NewPassword); err != nil {
		writeError(w, http.StatusInternalServerError, "could not update password")
		return
	}

	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}
