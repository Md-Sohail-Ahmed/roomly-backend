package services

import (
	"context"

	"roomly-api/internal/models"
	"roomly-api/internal/repository"
)

var ErrInvalidRole = repository.ErrInvalidRole
var ErrUserNotFound = repository.ErrUserNotFound

type adminUserStore interface {
	List(context.Context, int, int) ([]models.User, int, error)
	FindByID(context.Context, int64) (models.User, error)
	SetActive(context.Context, int64, bool) (models.User, error)
	SetRole(context.Context, int64, string) (models.User, error)
}

type AdminUserService struct{ users adminUserStore }

func NewAdminUserService(users *repository.UserRepository) *AdminUserService {
	return &AdminUserService{users: users}
}

func (s *AdminUserService) List(ctx context.Context, limit, offset int) ([]models.User, int, error) {
	if limit < 1 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}
	return s.users.List(ctx, limit, offset)
}

func (s *AdminUserService) Get(ctx context.Context, id int64) (models.User, error) {
	if id <= 0 {
		return models.User{}, ErrInvalidResourceID
	}
	return s.users.FindByID(ctx, id)
}

func (s *AdminUserService) SetActive(ctx context.Context, id int64, active bool) (models.User, error) {
	if id <= 0 {
		return models.User{}, ErrInvalidResourceID
	}
	return s.users.SetActive(ctx, id, active)
}

func (s *AdminUserService) SetRole(ctx context.Context, id int64, role string) (models.User, error) {
	if id <= 0 {
		return models.User{}, ErrInvalidResourceID
	}
	if role != "employee" && role != "admin" {
		return models.User{}, ErrInvalidRole
	}
	return s.users.SetRole(ctx, id, role)
}
