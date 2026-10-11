package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"roomly-api/internal/models"
	"roomly-api/internal/services"
)

type staticUserStatus struct {
	active bool
	role   string
	err    error
}

func (s staticUserStatus) AuthorizationState(context.Context, int64) (bool, string, error) {
	return s.active, s.role, s.err
}

func TestAuthAcceptsVerifiedBearerToken(t *testing.T) {
	secret := "0123456789abcdef0123456789abcdef"
	token, err := services.GenerateToken(models.User{ID: 9, Role: "admin"}, secret)
	if err != nil {
		t.Fatal(err)
	}
	handler := Auth(secret, staticUserStatus{active: true, role: "admin"}, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
	handler := Auth(secret, staticUserStatus{active: true, role: "employee"}, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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

func TestAuthRejectsDeactivatedUserAndRoleMiddlewareForbidsEmployee(t *testing.T) {
	secret := "0123456789abcdef0123456789abcdef"
	employeeToken, err := services.GenerateToken(models.User{ID: 5, Role: "employee"}, secret)
	if err != nil {
		t.Fatal(err)
	}
	protected := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) })
	adminOnly := RequireRole("admin", protected)
	for _, tc := range []struct {
		name   string
		active bool
		role   string
		want   int
	}{
		{name: "inactive user", active: false, role: "employee", want: http.StatusUnauthorized},
		{name: "employee lacks admin role", active: true, role: "employee", want: http.StatusForbidden},
	} {
		t.Run(tc.name, func(t *testing.T) {
			handler := Auth(secret, staticUserStatus{active: tc.active, role: tc.role}, adminOnly)
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.Header.Set("Authorization", "Bearer "+employeeToken)
			res := httptest.NewRecorder()
			handler.ServeHTTP(res, req)
			if res.Code != tc.want {
				t.Fatalf("status = %d, want %d", res.Code, tc.want)
			}
		})
	}
}

func TestAuthRejectsStaleRoleClaim(t *testing.T) {
	secret := "0123456789abcdef0123456789abcdef"
	token, err := services.GenerateToken(models.User{ID: 12, Role: "employee"}, secret)
	if err != nil {
		t.Fatal(err)
	}
	handler := Auth(secret, staticUserStatus{active: true, role: "admin"}, http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Error("stale role token must not reach protected handler")
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	if res.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusUnauthorized)
	}
}

func TestAuthReturnsServerErrorWhenUserLookupFails(t *testing.T) {
	secret := "0123456789abcdef0123456789abcdef"
	token, err := services.GenerateToken(models.User{ID: 12, Role: "employee"}, secret)
	if err != nil {
		t.Fatal(err)
	}
	handler := Auth(secret, staticUserStatus{err: errors.New("database unavailable")}, http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Error("handler must not run when user status cannot be checked")
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	if res.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusInternalServerError)
	}
}
