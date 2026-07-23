package api

import (
	"github.com/TeluTrix/seahorse/internal/auth"
	"github.com/TeluTrix/seahorse/internal/scanner"
)

type Handlers struct {
	Auth        *auth.Authenticator
	Scanner     *scanner.Scanner
	LibraryPath string
}

func NewHandlers(a *auth.Authenticator, s *scanner.Scanner, libraryPath string) *Handlers {
	return &Handlers{Auth: a, Scanner: s, LibraryPath: libraryPath}
}
