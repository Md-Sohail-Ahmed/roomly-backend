package services

import (
	"context"
	"errors"
	"strings"
	"time"

	"roomly-api/internal/models"
	"roomly-api/internal/repository"
)

var (
	ErrInvalidBooking   = errors.New("booking fields are invalid")
	ErrBookingInPast    = repository.ErrBookingStartPassed
	ErrBookingDuration  = errors.New("booking duration must be between 15 minutes and 4 hours")
	ErrBookingForbidden = errors.New("booking belongs to another user")
)

type bookingStore interface {
	CheckAvailability(context.Context, int64, time.Time, time.Time) (bool, error)
	ListRoomSchedule(context.Context, int64, time.Time, time.Time) ([]models.RoomScheduleItem, error)
	Create(context.Context, models.Booking) (models.Booking, error)
	Get(context.Context, int64) (models.Booking, error)
	List(context.Context, *int64, string, int, int) ([]models.Booking, int, error)
	Cancel(context.Context, int64) (models.Booking, error)
}

type BookingService struct{ bookings bookingStore }

func NewBookingService(bookings *repository.BookingRepository) *BookingService {
	return &BookingService{bookings: bookings}
}

func ParseTimestamp(value string) (time.Time, error) {
	t, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return time.Time{}, ErrInvalidBooking
	}
	return t.UTC(), nil
}

func (s *BookingService) Availability(ctx context.Context, roomID int64, start, end time.Time) (bool, error) {
	if roomID <= 0 {
		return false, ErrInvalidResourceID
	}
	if err := validateBookingInterval(start, end, time.Now().UTC()); err != nil {
		return false, err
	}
	return s.bookings.CheckAvailability(ctx, roomID, start.UTC(), end.UTC())
}

func (s *BookingService) RoomSchedule(ctx context.Context, roomID int64, start, end time.Time) ([]models.RoomScheduleItem, error) {
	if roomID <= 0 || start.IsZero() || end.IsZero() || !end.After(start) {
		return nil, ErrInvalidBooking
	}
	return s.bookings.ListRoomSchedule(ctx, roomID, start.UTC(), end.UTC())
}

func (s *BookingService) Create(ctx context.Context, booking models.Booking) (models.Booking, error) {
	booking.Title = strings.TrimSpace(booking.Title)
	if booking.RoomID <= 0 || booking.UserID <= 0 || booking.Title == "" || len([]rune(booking.Title)) > 200 || booking.AttendeeCount <= 0 {
		return models.Booking{}, ErrInvalidBooking
	}
	if err := validateBookingInterval(booking.StartTime, booking.EndTime, time.Now().UTC()); err != nil {
		return models.Booking{}, err
	}
	booking.StartTime = booking.StartTime.UTC()
	booking.EndTime = booking.EndTime.UTC()
	return s.bookings.Create(ctx, booking)
}

func (s *BookingService) List(ctx context.Context, userID *int64, status string, limit, offset int) ([]models.Booking, int, error) {
	if status != "" && status != "active" && status != "cancelled" {
		return nil, 0, ErrInvalidBooking
	}
	if limit < 1 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}
	return s.bookings.List(ctx, userID, status, limit, offset)
}

func (s *BookingService) Get(ctx context.Context, id, userID int64, admin bool) (models.Booking, error) {
	if id <= 0 {
		return models.Booking{}, ErrInvalidResourceID
	}
	b, err := s.bookings.Get(ctx, id)
	if err != nil {
		return models.Booking{}, err
	}
	if !admin && b.UserID != userID {
		return models.Booking{}, ErrBookingForbidden
	}
	return b, nil
}

func (s *BookingService) Cancel(ctx context.Context, id, userID int64, admin bool) (models.Booking, error) {
	if id <= 0 {
		return models.Booking{}, ErrInvalidResourceID
	}
	b, err := s.bookings.Get(ctx, id)
	if err != nil {
		return models.Booking{}, err
	}
	if !admin && b.UserID != userID {
		return models.Booking{}, ErrBookingForbidden
	}
	return s.bookings.Cancel(ctx, id)
}

func validateBookingInterval(start, end, now time.Time) error {
	if start.IsZero() || end.IsZero() || !end.After(start) {
		return ErrInvalidBooking
	}
	if !start.After(now) {
		return ErrBookingInPast
	}
	duration := end.Sub(start)
	if duration < 15*time.Minute || duration > 4*time.Hour {
		return ErrBookingDuration
	}
	return nil
}
