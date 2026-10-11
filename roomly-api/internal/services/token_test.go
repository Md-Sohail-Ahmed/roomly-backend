package services

import (
	"testing"
	"time"

	"roomly-api/internal/models"

	"github.com/golang-jwt/jwt/v5"
)

func TestGenerateAndParseToken(t *testing.T) {
	secret := "0123456789abcdef0123456789abcdef"
	user := models.User{ID: 42, Role: "employee"}
	token, err := GenerateToken(user, secret)
	if err != nil {
		t.Fatal(err)
	}
	id, role, err := ParseToken(token, secret)
	if err != nil || id != user.ID || role != user.Role {
		t.Fatalf("ParseToken() = (%d, %q, %v)", id, role, err)
	}
}

func TestParseTokenRejectsInvalidTokens(t *testing.T) {
	secret := "0123456789abcdef0123456789abcdef"
	if _, _, err := ParseToken("not-a-token", secret); err == nil {
		t.Fatal("expected malformed token to be rejected")
	}
	token, err := GenerateToken(models.User{ID: 7, Role: "employee"}, secret)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := ParseToken(token, "abcdef0123456789abcdef0123456789"); err == nil {
		t.Fatal("expected wrong signing secret to be rejected")
	}
	expired := jwt.NewWithClaims(jwt.SigningMethodHS256, TokenClaims{
		Role: "employee",
		RegisteredClaims: jwt.RegisteredClaims{
			Subject: "7", Issuer: TokenIssuer,
			IssuedAt:  jwt.NewNumericDate(time.Now().Add(-2 * time.Hour)),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-time.Hour)),
		},
	})
	oldToken, err := expired.SignedString([]byte(secret))
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := ParseToken(oldToken, secret); err == nil {
		t.Fatal("expected expired token to be rejected")
	}
}
