package services

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"roomly-api/internal/models"
	"roomly-api/internal/repository"
)

type authStoreStub struct {
	createdName, createdEmail, createdHash string
	findEmail                              string
	user                                   models.User
	createErr, findErr                     error
	createCalls                            int
}

func (s *authStoreStub) Create(_ context.Context, name, email, hash string) (models.User, error) {
	s.createCalls++
	s.createdName, s.createdEmail, s.createdHash = name, email, hash
	if s.createErr != nil {
		return models.User{}, s.createErr
	}
	return models.User{ID: 1, Name: name, Email: email, PasswordHash: hash, Role: "employee", IsActive: true}, nil
}
func (s *authStoreStub) FindByEmail(_ context.Context, email string) (models.User, error) {
	s.findEmail = email
	return s.user, s.findErr
}

func TestRegisterNormalizesAndHashesPassword(t *testing.T) {
	store := &authStoreStub{}
	service := NewAuthService(store)
	user, err := service.Register(context.Background(), "  Riya Shah ", " RIYA@EXAMPLE.COM ", "correct-horse-77")
	if err != nil {
		t.Fatal(err)
	}
	if store.createdName != "Riya Shah" || store.createdEmail != "riya@example.com" {
		t.Fatalf("stored identity = %q, %q", store.createdName, store.createdEmail)
	}
	if store.createdHash == "correct-horse-77" || CheckPassword("correct-horse-77", store.createdHash) != nil {
		t.Fatal("password was not bcrypt hashed")
	}
	if user.Role != "employee" || user.PasswordHash != store.createdHash {
		t.Fatalf("unexpected created user: %#v", user)
	}
}

func TestRegisterRejectsInvalidInputAndReturnsDuplicateError(t *testing.T) {
	store := &authStoreStub{createErr: repository.ErrEmailAlreadyExists}
	service := NewAuthService(store)
	if _, err := service.Register(context.Background(), "X", "bad", "short"); !errors.Is(err, ErrInvalidName) {
		t.Fatalf("invalid name error = %v", err)
	}
	if store.createCalls != 0 {
		t.Fatal("repository called for invalid input")
	}
	if _, err := service.Register(context.Background(), "Valid Name", "valid@example.com", "password-123"); !errors.Is(err, repository.ErrEmailAlreadyExists) {
		t.Fatalf("duplicate error = %v", err)
	}
}

func TestLoginSuccessWrongPasswordInactiveAndMissingUser(t *testing.T) {
	hash, err := HashPassword("correct-horse-77")
	if err != nil {
		t.Fatal(err)
	}
	store := &authStoreStub{user: models.User{ID: 4, Email: "staff@example.com", PasswordHash: hash, Role: "employee", IsActive: true}}
	service := NewAuthService(store)
	user, err := service.Login(context.Background(), " STAFF@EXAMPLE.COM ", "correct-horse-77")
	if err != nil || user.ID != 4 || store.findEmail != "staff@example.com" {
		t.Fatalf("Login = %#v, %v; looked up %q", user, err, store.findEmail)
	}
	if _, err := service.Login(context.Background(), "staff@example.com", "wrong-password"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("wrong password error = %v", err)
	}
	store.user.IsActive = false
	if _, err := service.Login(context.Background(), "staff@example.com", "correct-horse-77"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("inactive user error = %v", err)
	}
	store.findErr = sql.ErrNoRows
	if _, err := service.Login(context.Background(), "missing@example.com", "anything"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("missing user error = %v", err)
	}
}
