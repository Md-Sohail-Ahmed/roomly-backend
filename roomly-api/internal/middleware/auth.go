package middleware

import (
	"context"
	"net/http"
	"strings"

	"roomly-api/internal/services"
)

type identityKey struct{}

type Identity struct {
	UserID int64
	Role   string
}

type UserStatus interface {
	IsActive(context.Context, int64) (bool, error)
}

func Auth(secret string, users UserStatus, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authorization := r.Header.Get("Authorization")
		parts := strings.Fields(authorization)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			unauthorized(w)
			return
		}
		id, role, err := services.ParseToken(parts[1], secret)
		if err != nil {
			unauthorized(w)
			return
		}
		active, err := users.IsActive(r.Context(), id)
		if err != nil || !active {
			unauthorized(w)
			return
		}
		ctx := context.WithValue(r.Context(), identityKey{}, Identity{UserID: id, Role: role})
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func RequireRole(role string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		identity, ok := AuthenticatedUser(r.Context())
		if !ok {
			unauthorized(w)
			return
		}
		if identity.Role != role {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`{"error":"forbidden"}`))
			return
		}
		next.ServeHTTP(w, r)
	})
}

func AuthenticatedUser(ctx context.Context) (Identity, bool) {
	identity, ok := ctx.Value(identityKey{}).(Identity)
	return identity, ok
}

func unauthorized(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	_, _ = w.Write([]byte(`{"error":"unauthorized"}`))
}
