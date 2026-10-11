package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/mail"
	"strings"

	"roomly-api/internal/models"
	"roomly-api/internal/repository"
	"roomly-api/internal/services"
)

var (
	ErrInvalidName        = errors.New("name must be between 2 and 100 characters")
	ErrInvalidEmail       = errors.New("valid email is required")
	ErrInvalidPassword    = errors.New("password must be between 8 and 72 bytes")
	ErrInvalidCredentials = errors.New("invalid email or password")
)

type AuthService struct {
	users *repository.UserRepository
}

type AuthHandler struct {
	auth      *services.AuthService
	jwtSecret string
}

func NewAuthHandler(
	auth *services.AuthService,
	jwtSecret string,
) *AuthHandler {
	return &AuthHandler{
		auth:      auth,
		jwtSecret: jwtSecret,
	}
}

func (s *AuthService) Register(
	ctx context.Context,
	name string,
	email string,
	password string,
) (models.User, error) {
	name = strings.TrimSpace(name)
	email = strings.ToLower(strings.TrimSpace(email))

	if len([]rune(name)) < 2 || len([]rune(name)) > 100 {
		return models.User{}, ErrInvalidName
	}

	if err := validateEmail(email); err != nil {
		return models.User{}, err
	}

	if len(password) < 8 || len(password) > 72 {
		return models.User{}, ErrInvalidPassword
	}

	passwordHash, err := HashPassword(password)
	if err != nil {
		return models.User{}, fmt.Errorf("hash password: %w", err)
	}

	user, err := s.users.Create(ctx, name, email, passwordHash)
	if err != nil {
		return models.User{}, err
	}

	return user, nil
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	// Limit the request body size to prevent oversized payloads.
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

	// Decode the JSON request and reject unknown fields.
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	var req loginRequest

	if err := decoder.Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Ensure the body contains exactly one JSON object.
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		writeError(
			w,
			http.StatusBadRequest,
			"request must contain one JSON object",
		)
		return
	}

	// Authenticate the user.
	user, err := h.auth.Login(
		r.Context(),
		req.Email,
		req.Password,
	)
	if err != nil {
		if errors.Is(err, services.ErrInvalidCredentials) {
			writeError(
				w,
				http.StatusUnauthorized,
				"invalid email or password",
			)
			return
		}

		log.Printf("login failed: %v", err)
		writeError(
			w,
			http.StatusInternalServerError,
			"could not log in",
		)
		return
	}

	// Generate a JWT after successful authentication.
	token, err := services.GenerateToken(user, h.jwtSecret)
	if err != nil {
		log.Printf("generate login token: %v", err)
		writeError(
			w,
			http.StatusInternalServerError,
			"could not log in",
		)
		return
	}

	// Return the token and user details.
	writeJSON(w, http.StatusOK, map[string]any{
		"token": token,
		"user":  user,
	})
}

func validateEmail(email string) error {
	if len(email) == 0 || len(email) > 255 {
		return ErrInvalidEmail
	}

	address, err := mail.ParseAddress(email)
	if err != nil || address.Address != email {
		return ErrInvalidEmail
	}

	return nil
}
