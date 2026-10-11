package services

import (
	"context"
	"testing"

	"roomly-api/internal/models"
)

type roomStoreStub struct {
	created models.Room
	filter  models.RoomFilter
}

func (s *roomStoreStub) List(_ context.Context, f models.RoomFilter) ([]models.Room, int, error) {
	s.filter = f
	return []models.Room{}, 0, nil
}
func (*roomStoreStub) Get(context.Context, int64) (models.Room, error) { return models.Room{}, nil }
func (s *roomStoreStub) Create(_ context.Context, room models.Room) (models.Room, error) {
	s.created = room
	return room, nil
}
func (*roomStoreStub) Update(context.Context, int64, models.Room) (models.Room, error) {
	return models.Room{}, nil
}
func (*roomStoreStub) SetActive(context.Context, int64, bool) (models.Room, error) {
	return models.Room{}, nil
}

func TestRoomServiceValidatesAndNormalizesCreate(t *testing.T) {
	store := &roomStoreStub{}
	service := &RoomService{rooms: store}
	room, err := service.Create(context.Background(), models.Room{Name: "  Cedar  ", Capacity: 8, Location: "  Floor 2 "})
	if err != nil {
		t.Fatal(err)
	}
	if room.Name != "Cedar" || room.Location != "Floor 2" {
		t.Fatalf("room was not normalized: %#v", room)
	}
	if store.created.Capacity != 8 {
		t.Fatalf("stored capacity = %d, want 8", store.created.Capacity)
	}
}

func TestRoomServiceRejectsInvalidCapacityAndIDs(t *testing.T) {
	service := &RoomService{rooms: &roomStoreStub{}}
	if _, err := service.Create(context.Background(), models.Room{Name: "Room", Location: "HQ", Capacity: 0}); err != ErrInvalidRoom {
		t.Fatalf("Create error = %v, want %v", err, ErrInvalidRoom)
	}
	if _, err := service.Get(context.Background(), 0); err != ErrInvalidResourceID {
		t.Fatalf("Get error = %v, want %v", err, ErrInvalidResourceID)
	}
}

func TestRoomServiceBoundsPagination(t *testing.T) {
	store := &roomStoreStub{}
	service := &RoomService{rooms: store}
	_, _, err := service.List(context.Background(), models.RoomFilter{Location: "  HQ ", Limit: 900, Offset: -3})
	if err != nil {
		t.Fatal(err)
	}
	if store.filter.Location != "HQ" || store.filter.Limit != 100 || store.filter.Offset != 0 {
		t.Fatalf("unexpected bounded filter: %#v", store.filter)
	}
}
