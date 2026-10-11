package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"roomly-api/internal/models"
	"roomly-api/internal/services"
)

func TestAuthAcceptsVerifiedBearerToken(t *testing.T) {
	secret := "0123456789abcdef0123456789abcdef"
	token, err := services.GenerateToken(models.User{ID: 9, Role: "admin"}, secret)
	if err != nil {
		t.Fatal(err)
	}
	handler := Auth(secret, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		identity, ok := AuthenticatedUser(r.Context())
		if !ok || identity.UserID != 9 || identity.Role != "admin" {
			t.Errorf("unexpected authenticated identity: %#v, %v", identity, ok)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	if res.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusNoContent)
	}
}

func TestAuthRejectsMissingMalformedAndForgedTokens(t *testing.T) {
	secret := "0123456789abcdef0123456789abcdef"
	handler := Auth(secret, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("next handler must not be called")
	}))
	for _, authorization := range []string{"", "Basic abc", "Bearer broken", "Bearer a.b.c"} {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Authorization", authorization)
		res := httptest.NewRecorder()
		handler.ServeHTTP(res, req)
		if res.Code != http.StatusUnauthorized {
			t.Errorf("Authorization %q: status = %d, want %d", authorization, res.Code, http.StatusUnauthorized)
		}
	}
}
