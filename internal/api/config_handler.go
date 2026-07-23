package api

import "net/http"

// GetConfig serves the frontend's runtime-tunable values (see ClientConfig).
// Public — it carries no user data, just non-sensitive UI/behavior knobs
// the frontend needs before a user has necessarily logged in.
func (h *Handlers) GetConfig(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, h.ClientConfig)
}
