package middleware

import (
	"context"
	"database/sql"
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

	"roomly-api/internal/services"
)

type identityKey struct{}

type Identity struct {
	UserID int64
	Role   string
}

type UserStatus interface {
	AuthorizationState(context.Context, int64) (bool, string, error)
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
		active, currentRole, err := users.AuthorizationState(r.Context(), id)
		if errors.Is(err, sql.ErrNoRows) || (err == nil && (!active || currentRole != role)) {
			unauthorized(w)
			return
		}
		if err != nil {
			log.Print("authentication state lookup failed")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"error":"internal server error"}`))
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

func RequestTimeout(timeout time.Duration, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), timeout)
		defer cancel()
		next.ServeHTTP(w, r.WithContext(ctx))
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
