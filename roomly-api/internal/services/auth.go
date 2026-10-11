package services

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/mail"
	"strings"

	"roomly-api/internal/models"
	"roomly-api/internal/repository"
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

func NewAuthService(users *repository.UserRepository) *AuthService {
	return &AuthService{users: users}
}

func (s *AuthService) Register(ctx context.Context, name, email, password string) (models.User, error) {
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
	hash, err := HashPassword(password)
	if err != nil {
		return models.User{}, fmt.Errorf("hash password: %w", err)
	}
	return s.users.Create(ctx, name, email, hash)
}

func (s *AuthService) Login(ctx context.Context, email, password string) (models.User, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if validateEmail(email) != nil || password == "" {
		return models.User{}, ErrInvalidCredentials
	}
	user, err := s.users.FindByEmail(ctx, email)
	if errors.Is(err, sql.ErrNoRows) {
		return models.User{}, ErrInvalidCredentials
	}
	if err != nil {
		return models.User{}, fmt.Errorf("find user for login: %w", err)
	}
	if !user.IsActive || CheckPassword(password, user.PasswordHash) != nil {
		return models.User{}, ErrInvalidCredentials
	}
	return user, nil
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
