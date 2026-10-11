package services

import (
	"context"
	"errors"
	"strings"

	"roomly-api/internal/models"
	"roomly-api/internal/repository"
)

var ErrInvalidRoom = errors.New("room name and location are required; capacity must be positive")

type roomStore interface {
	List(context.Context, models.RoomFilter) ([]models.Room, int, error)
	Get(context.Context, int64) (models.Room, error)
	Create(context.Context, models.Room) (models.Room, error)
	Update(context.Context, int64, models.Room) (models.Room, error)
	SetActive(context.Context, int64, bool) (models.Room, error)
}

type RoomService struct{ rooms roomStore }

func NewRoomService(rooms *repository.RoomRepository) *RoomService { return &RoomService{rooms: rooms} }

func (s *RoomService) List(ctx context.Context, filter models.RoomFilter) ([]models.Room, int, error) {
	filter.Location = strings.TrimSpace(filter.Location)
	if filter.MinCapacity < 0 {
		return nil, 0, ErrInvalidRoom
	}
	if filter.Limit < 1 {
		filter.Limit = 20
	}
	if filter.Limit > 100 {
		filter.Limit = 100
	}
	if filter.Offset < 0 {
		filter.Offset = 0
	}
	return s.rooms.List(ctx, filter)
}

func (s *RoomService) Get(ctx context.Context, id int64) (models.Room, error) {
	if id <= 0 {
		return models.Room{}, ErrInvalidResourceID
	}
	return s.rooms.Get(ctx, id)
}

func (s *RoomService) Create(ctx context.Context, room models.Room) (models.Room, error) {
	room, err := validateRoom(room)
	if err != nil {
		return models.Room{}, err
	}
	return s.rooms.Create(ctx, room)
}

func (s *RoomService) Update(ctx context.Context, id int64, room models.Room) (models.Room, error) {
	if id <= 0 {
		return models.Room{}, ErrInvalidResourceID
	}
	room, err := validateRoom(room)
	if err != nil {
		return models.Room{}, err
	}
	return s.rooms.Update(ctx, id, room)
}

func (s *RoomService) SetActive(ctx context.Context, id int64, active bool) (models.Room, error) {
	if id <= 0 {
		return models.Room{}, ErrInvalidResourceID
	}
	return s.rooms.SetActive(ctx, id, active)
}

var ErrInvalidResourceID = errors.New("resource ID must be a positive integer")

func validateRoom(room models.Room) (models.Room, error) {
	room.Name = strings.TrimSpace(room.Name)
	room.Location = strings.TrimSpace(room.Location)
	if room.Name == "" || len([]rune(room.Name)) > 100 || room.Location == "" || len([]rune(room.Location)) > 255 || room.Capacity <= 0 {
		return models.Room{}, ErrInvalidRoom
	}
	return room, nil
}
