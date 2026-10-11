package models

import "time"

type Booking struct {
	ID            int64     `json:"id"`
	RoomID        int64     `json:"room_id"`
	UserID        int64     `json:"user_id"`
	Title         string    `json:"title"`
	AttendeeCount int       `json:"attendee_count"`
	StartTime     time.Time `json:"start_time"`
	EndTime       time.Time `json:"end_time"`
	Status        string    `json:"status"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}
