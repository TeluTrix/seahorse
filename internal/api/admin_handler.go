package api

import (
	"errors"
	"net/http"

	"github.com/TeluTrix/seahorse/internal/scanner"
)

func (h *Handlers) ScanLibrary(w http.ResponseWriter, r *http.Request) {
	err := h.Scanner.StartScan(h.LibraryPath)
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
