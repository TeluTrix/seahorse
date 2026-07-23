package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/TeluTrix/seahorse/internal/scanner"
	"github.com/TeluTrix/seahorse/internal/user"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"gorm.io/gorm"
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

// ScanEvents streams live scan status over Server-Sent Events: a plain,
// one-way HTTP stream the browser reads with the native EventSource API
// (which reconnects automatically), instead of the frontend polling this
// endpoint's REST sibling on a timer.
func (h *Handlers) ScanEvents(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming not supported")
		return
	}

	ch, cancel := h.Scanner.Subscribe()
	defer cancel()

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	heartbeat := time.NewTicker(30 * time.Second)
	defer heartbeat.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case status, ok := <-ch:
			if !ok {
				return
			}
			data, err := json.Marshal(status)
			if err != nil {
				continue
			}
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		case <-heartbeat.C:
			fmt.Fprint(w, ": keep-alive\n\n")
			flusher.Flush()
		}
	}
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

type createUserRequest struct {
	Email    string `json:"user_email"`
	Password string `json:"user_password"`
}

// CreateUser lets an admin manually create an account, independent of
// SEAHORSE_DISABLE_REGISTRATION — that setting only gates the public
// self-service /auth/register endpoint, not this admin-only one.
func (h *Handlers) CreateUser(w http.ResponseWriter, r *http.Request) {
	var req createUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	req.Email = strings.TrimSpace(strings.ToLower(req.Email))
	if req.Email == "" || len(req.Password) < 8 {
		writeError(w, http.StatusBadRequest, "email is required and password must be at least 8 characters")
		return
	}

	newUser, err := user.CreateUser(req.Email, req.Password)
	if err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) || strings.Contains(err.Error(), "UNIQUE constraint") {
			writeError(w, http.StatusConflict, "a user with that email already exists")
			return
		}
		writeError(w, http.StatusInternalServerError, "could not create user")
		return
	}

	writeJSON(w, http.StatusCreated, toPublicUser(newUser))
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
