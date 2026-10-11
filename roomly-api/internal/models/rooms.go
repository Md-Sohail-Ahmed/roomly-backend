package models

import "time"

type Amenity struct {
	ID          int64     `json:"id"`
	Name        string    `json:"name"`
	Description *string   `json:"description,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

type Room struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	Capacity  int       `json:"capacity"`
	Location  string    `json:"location"`
	IsActive  bool      `json:"is_active"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Amenities []Amenity `json:"amenities"`
}

type RoomFilter struct {
	Location    string
	MinCapacity int
	Limit       int
	Offset      int
}
