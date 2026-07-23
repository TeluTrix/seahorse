package api

import (
	"github.com/TeluTrix/seahorse/internal/auth"
	"github.com/TeluTrix/seahorse/internal/scanner"
)

// ClientConfig holds runtime-tunable values the frontend needs at load time
// (player seek amount, pagination default, etc.). The frontend is a prebuilt
// static bundle, so it can't read the server's env vars directly — instead
// it fetches these once from GET /api/config (see GetConfig).
type ClientConfig struct {
	DefaultPageSize               int `json:"default_page_size"`
	PlayerSeekSeconds             int `json:"player_seek_seconds"`
	ResumeThresholdSeconds        int `json:"resume_threshold_seconds"`
	ProgressReportIntervalSeconds int `json:"progress_report_interval_seconds"`
}

type Handlers struct {
	Auth        *auth.Authenticator
	Scanner     *scanner.Scanner
	LibraryPath string

	MaxPageSize  int
	ClientConfig ClientConfig
}

func NewHandlers(a *auth.Authenticator, s *scanner.Scanner, libraryPath string, maxPageSize int, clientConfig ClientConfig) *Handlers {
	return &Handlers{
		Auth:         a,
		Scanner:      s,
		LibraryPath:  libraryPath,
		MaxPageSize:  maxPageSize,
		ClientConfig: clientConfig,
	}
}
