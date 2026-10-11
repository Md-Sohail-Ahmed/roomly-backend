package services

import (
	"context"
	"testing"

	"roomly-api/internal/models"
)

type adminUserStoreStub struct {
	limit, offset int
	role          string
}

func (s *adminUserStoreStub) List(_ context.Context, limit, offset int) ([]models.User, int, error) {
	s.limit, s.offset = limit, offset
	return []models.User{}, 0, nil
}
func (*adminUserStoreStub) FindByID(context.Context, int64) (models.User, error) {
	return models.User{}, nil
}
func (*adminUserStoreStub) SetActive(context.Context, int64, bool) (models.User, error) {
	return models.User{}, nil
}
func (s *adminUserStoreStub) SetRole(_ context.Context, _ int64, role string) (models.User, error) {
	s.role = role
	return models.User{Role: role}, nil
}

func TestAdminUserServiceValidatesRoleAndPagination(t *testing.T) {
	store := &adminUserStoreStub{}
	service := &AdminUserService{users: store}
	if _, err := service.SetRole(context.Background(), 2, "root"); err != ErrInvalidRole {
		t.Fatalf("SetRole error = %v, want %v", err, ErrInvalidRole)
	}
	if user, err := service.SetRole(context.Background(), 2, "admin"); err != nil || user.Role != "admin" {
		t.Fatalf("SetRole = %#v, %v", user, err)
	}
	if _, _, err := service.List(context.Background(), 500, -3); err != nil {
		t.Fatal(err)
	}
	if store.limit != 100 || store.offset != 0 {
		t.Fatalf("list args = %d, %d", store.limit, store.offset)
	}
}
