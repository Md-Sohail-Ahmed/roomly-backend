package services

import (
	"errors"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"roomly-api/internal/models"
)

type TokenClaims struct {
	Role string `json:"role"`
	jwt.RegisteredClaims
}

func GenerateToken(user models.User, secret string) (string, error) {
	if len(secret) < 32 {
		return "", errors.New("JWT secret must be at least 32 bytes")
	}

	now := time.Now()

	claims := TokenClaims{
		Role: user.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   fmtInt(user.ID),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(24 * time.Hour)),
			Issuer:    "roomly-api",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	signedToken, err := token.SignedString([]byte(secret))
	if err != nil {
		return "", err
	}

	return signedToken, nil
}

func fmtInt(id int64) string {
	return strconv.FormatInt(id, 10)
}
