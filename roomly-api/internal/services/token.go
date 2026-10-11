package services

import (
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"roomly-api/internal/models"
)

type TokenClaims struct {
	Role string `json:"role"`
	jwt.RegisteredClaims
}

const TokenIssuer = "roomly-api"

func GenerateToken(user models.User, secret string) (string, error) {
	if len(secret) < 32 || user.ID <= 0 || user.Role == "" {
		return "", errors.New("JWT secret must be at least 32 bytes")
	}

	now := time.Now().UTC()

	claims := TokenClaims{
		Role: user.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   fmtInt(user.ID),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(24 * time.Hour)),
			Issuer:    TokenIssuer,
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

func ParseToken(raw, secret string) (int64, string, error) {
	if len(secret) < 32 {
		return 0, "", errors.New("invalid token configuration")
	}
	claims := &TokenClaims{}
	token, err := jwt.ParseWithClaims(raw, claims, func(token *jwt.Token) (any, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, fmt.Errorf("unexpected signing method")
		}
		return []byte(secret), nil
	}, jwt.WithIssuer(TokenIssuer), jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}))
	if err != nil || token == nil || !token.Valid {
		return 0, "", errors.New("invalid token")
	}
	id, err := strconv.ParseInt(claims.Subject, 10, 64)
	if err != nil || id <= 0 || claims.Role == "" {
		return 0, "", errors.New("invalid token claims")
	}
	return id, claims.Role, nil
}
