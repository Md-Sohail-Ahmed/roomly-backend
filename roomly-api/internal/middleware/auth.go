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

func Auth(secret string, next http.Handler) http.Handler {
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
		ctx := context.WithValue(r.Context(), identityKey{}, Identity{UserID: id, Role: role})
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
