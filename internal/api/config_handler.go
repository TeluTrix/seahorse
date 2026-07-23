package api

import (
	"net/http"

	"github.com/TeluTrix/seahorse/internal/user"
)

// GetConfig serves the frontend's runtime-tunable values (see ClientConfig).
// Public — it carries no user data, just non-sensitive UI/behavior knobs
// the frontend needs before a user has necessarily logged in.
func (h *Handlers) GetConfig(w http.ResponseWriter, r *http.Request) {
	config := h.ClientConfig
	config.RegistrationEnabled = h.registrationEnabled()
	writeJSON(w, http.StatusOK, config)
}

// registrationEnabled mirrors the gate in Register: always true until the
// first (bootstrap admin) account exists, since SEAHORSE_DISABLE_REGISTRATION
// only makes sense to enforce after that.
func (h *Handlers) registrationEnabled() bool {
	if !h.DisableRegistration {
		return true
	}
	count, err := user.Count()
	if err != nil {
		return true
	}
	return count == 0
}
