package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/TeluTrix/seahorse/internal/user"
	"gorm.io/gorm"
)

type registerRequest struct {
	Email    string `json:"user_email"`
	Password string `json:"user_password"`
}

type loginRequest struct {
	Email    string `json:"user_email"`
	Password string `json:"user_password"`
}

type authResponse struct {
	Token string     `json:"token"`
	User  PublicUser `json:"user"`
}

func (h *Handlers) Register(w http.ResponseWriter, r *http.Request) {
	var req registerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	req.Email = strings.TrimSpace(strings.ToLower(req.Email))
	if req.Email == "" || len(req.Password) < 8 {
		writeError(w, http.StatusBadRequest, "email is required and password must be at least 8 characters")
		return
	}

	// registrationEnabled always allows the very first (bootstrap admin)
	// account regardless of SEAHORSE_DISABLE_REGISTRATION — otherwise a
	// fresh install with that set would have no way to ever create a user.
	if !h.registrationEnabled() {
		writeError(w, http.StatusForbidden, "registration is disabled")
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

	token, err := h.Auth.GenerateToken(newUser.UserID, newUser.UserRole)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not generate token")
		return
	}

	writeJSON(w, http.StatusCreated, authResponse{Token: token, User: toPublicUser(newUser)})
}

func (h *Handlers) Login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	req.Email = strings.TrimSpace(strings.ToLower(req.Email))

	authenticated, err := user.Authenticate(req.Email, req.Password)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid email or password")
		return
	}

	token, err := h.Auth.GenerateToken(authenticated.UserID, authenticated.UserRole)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not generate token")
		return
	}

	writeJSON(w, http.StatusOK, authResponse{Token: token, User: toPublicUser(authenticated)})
}
