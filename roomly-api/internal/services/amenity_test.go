package services

import (
	"context"
	"testing"

	"roomly-api/internal/models"
)

type amenityStoreStub struct {
	created               models.Amenity
	listLimit, listOffset int
}

func (s *amenityStoreStub) List(_ context.Context, limit, offset int) ([]models.Amenity, int, error) {
	s.listLimit, s.listOffset = limit, offset
	return []models.Amenity{}, 0, nil
}
func (*amenityStoreStub) Get(context.Context, int64) (models.Amenity, error) {
	return models.Amenity{}, nil
}
func (s *amenityStoreStub) Create(_ context.Context, a models.Amenity) (models.Amenity, error) {
	s.created = a
	return a, nil
}
func (*amenityStoreStub) Update(context.Context, int64, models.Amenity) (models.Amenity, error) {
	return models.Amenity{}, nil
}
func (*amenityStoreStub) Delete(context.Context, int64) error { return nil }
func (*amenityStoreStub) ListForRoom(context.Context, int64) ([]models.Amenity, error) {
	return []models.Amenity{}, nil
}
func (*amenityStoreStub) AddToRoom(context.Context, int64, int64) error      { return nil }
func (*amenityStoreStub) RemoveFromRoom(context.Context, int64, int64) error { return nil }

func TestAmenityServiceValidatesAndNormalizes(t *testing.T) {
	store := &amenityStoreStub{}
	service := &AmenityService{amenities: store}
	description := "  HDMI display  "
	item, err := service.Create(context.Background(), models.Amenity{Name: "  Projector ", Description: &description})
	if err != nil {
		t.Fatal(err)
	}
	if item.Name != "Projector" || *item.Description != "HDMI display" {
		t.Fatalf("amenity not normalized: %#v", item)
	}
	if _, err := service.Create(context.Background(), models.Amenity{Name: " "}); err != ErrInvalidAmenity {
		t.Fatalf("Create error = %v, want %v", err, ErrInvalidAmenity)
	}
}

func TestAmenityServiceBoundsPaginationAndValidatesAssociationIDs(t *testing.T) {
	store := &amenityStoreStub{}
	service := &AmenityService{amenities: store}
	if _, _, err := service.List(context.Background(), 500, -8); err != nil {
		t.Fatal(err)
	}
	if store.listLimit != 100 || store.listOffset != 0 {
		t.Fatalf("list args = %d, %d", store.listLimit, store.listOffset)
	}
	if err := service.AddToRoom(context.Background(), 1, 0); err != ErrInvalidResourceID {
		t.Fatalf("association error = %v, want %v", err, ErrInvalidResourceID)
	}
}
