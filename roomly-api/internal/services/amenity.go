package services

import (
	"context"
	"errors"
	"strings"

	"roomly-api/internal/models"
	"roomly-api/internal/repository"
)

var ErrInvalidAmenity = errors.New("amenity name is required and must be at most 100 characters")

type amenityStore interface {
	List(context.Context, int, int) ([]models.Amenity, int, error)
	Get(context.Context, int64) (models.Amenity, error)
	Create(context.Context, models.Amenity) (models.Amenity, error)
	Update(context.Context, int64, models.Amenity) (models.Amenity, error)
	Delete(context.Context, int64) error
	ListForRoom(context.Context, int64) ([]models.Amenity, error)
	AddToRoom(context.Context, int64, int64) error
	RemoveFromRoom(context.Context, int64, int64) error
}

type AmenityService struct{ amenities amenityStore }

func NewAmenityService(amenities *repository.AmenityRepository) *AmenityService {
	return &AmenityService{amenities: amenities}
}

func (s *AmenityService) List(ctx context.Context, limit, offset int) ([]models.Amenity, int, error) {
	if limit < 1 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}
	return s.amenities.List(ctx, limit, offset)
}

func (s *AmenityService) Get(ctx context.Context, id int64) (models.Amenity, error) {
	if id <= 0 {
		return models.Amenity{}, ErrInvalidResourceID
	}
	return s.amenities.Get(ctx, id)
}

func (s *AmenityService) Create(ctx context.Context, a models.Amenity) (models.Amenity, error) {
	a, err := validateAmenity(a)
	if err != nil {
		return models.Amenity{}, err
	}
	return s.amenities.Create(ctx, a)
}

func (s *AmenityService) Update(ctx context.Context, id int64, a models.Amenity) (models.Amenity, error) {
	if id <= 0 {
		return models.Amenity{}, ErrInvalidResourceID
	}
	a, err := validateAmenity(a)
	if err != nil {
		return models.Amenity{}, err
	}
	return s.amenities.Update(ctx, id, a)
}

func (s *AmenityService) Delete(ctx context.Context, id int64) error {
	if id <= 0 {
		return ErrInvalidResourceID
	}
	return s.amenities.Delete(ctx, id)
}

func (s *AmenityService) ListForRoom(ctx context.Context, roomID int64) ([]models.Amenity, error) {
	if roomID <= 0 {
		return nil, ErrInvalidResourceID
	}
	return s.amenities.ListForRoom(ctx, roomID)
}

func (s *AmenityService) AddToRoom(ctx context.Context, roomID, amenityID int64) error {
	if roomID <= 0 || amenityID <= 0 {
		return ErrInvalidResourceID
	}
	return s.amenities.AddToRoom(ctx, roomID, amenityID)
}

func (s *AmenityService) RemoveFromRoom(ctx context.Context, roomID, amenityID int64) error {
	if roomID <= 0 || amenityID <= 0 {
		return ErrInvalidResourceID
	}
	return s.amenities.RemoveFromRoom(ctx, roomID, amenityID)
}

func validateAmenity(a models.Amenity) (models.Amenity, error) {
	a.Name = strings.TrimSpace(a.Name)
	if a.Name == "" || len([]rune(a.Name)) > 100 {
		return models.Amenity{}, ErrInvalidAmenity
	}
	if a.Description != nil {
		description := strings.TrimSpace(*a.Description)
		if len([]rune(description)) > 255 {
			return models.Amenity{}, ErrInvalidAmenity
		}
		if description == "" {
			a.Description = nil
		} else {
			a.Description = &description
		}
	}
	return a, nil
}
