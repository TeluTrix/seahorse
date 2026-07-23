package auth

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/TeluTrix/seahorse/internal/models"
	"github.com/google/uuid"
)

type contextKey string

const (
	userIDKey contextKey = "userID"
	roleKey   contextKey = "role"
)

type Authenticator struct {
	Secret string
	TTL    time.Duration
}

func New(secret string, ttl time.Duration) *Authenticator {
	return &Authenticator{Secret: secret, TTL: ttl}
}

func (a *Authenticator) GenerateToken(userID uuid.UUID, role models.Role) (string, error) {
	return GenerateToken(a.Secret, userID, role, a.TTL)
}

func (a *Authenticator) extractToken(r *http.Request) string {
	if header := r.Header.Get("Authorization"); header != "" {
		if strings.HasPrefix(header, "Bearer ") {
			return strings.TrimPrefix(header, "Bearer ")
		}
	}
	return r.URL.Query().Get("token")
}

// RequireAuth validates the JWT (from the Authorization header or a ?token=
// query param, since native <video> elements can't set custom headers) and
// stores the user id/role on the request context.
func (a *Authenticator) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tokenStr := a.extractToken(r)
		if tokenStr == "" {
			http.Error(w, "missing token", http.StatusUnauthorized)
			return
		}

		claims, err := ParseToken(a.Secret, tokenStr)
		if err != nil {
			http.Error(w, "invalid or expired token", http.StatusUnauthorized)
			return
		}

		ctx := context.WithValue(r.Context(), userIDKey, claims.UserID)
		ctx = context.WithValue(ctx, roleKey, claims.Role)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (a *Authenticator) RequireAdmin(next http.Handler) http.Handler {
	return a.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		role, ok := RoleFromContext(r.Context())
		if !ok || role != models.RoleAdmin {
			http.Error(w, "admin access required", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	}))
}

func UserIDFromContext(ctx context.Context) (uuid.UUID, bool) {
	id, ok := ctx.Value(userIDKey).(uuid.UUID)
	return id, ok
}

func RoleFromContext(ctx context.Context) (models.Role, bool) {
	role, ok := ctx.Value(roleKey).(models.Role)
	return role, ok
}
