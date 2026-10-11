package services

import (
	"context"
	"testing"
	"time"

	"roomly-api/internal/models"
)

type bookingStoreStub struct {
	created       models.Booking
	booking       models.Booking
	userID        *int64
	status        string
	limit, offset int
	available     bool
}

func (s *bookingStoreStub) CheckAvailability(context.Context, int64, time.Time, time.Time) (bool, error) {
	return s.available, nil
}
func (s *bookingStoreStub) ListRoomSchedule(context.Context, int64, time.Time, time.Time) ([]models.RoomScheduleItem, error) {
	return nil, nil
}
func (s *bookingStoreStub) Create(_ context.Context, b models.Booking) (models.Booking, error) {
	s.created = b
	return b, nil
}
func (s *bookingStoreStub) Get(context.Context, int64) (models.Booking, error) { return s.booking, nil }
func (s *bookingStoreStub) List(_ context.Context, userID *int64, status string, limit, offset int) ([]models.Booking, int, error) {
	s.userID, s.status, s.limit, s.offset = userID, status, limit, offset
	return nil, 0, nil
}
func (s *bookingStoreStub) Cancel(context.Context, int64) (models.Booking, error) {
	s.booking.Status = "cancelled"
	return s.booking, nil
}

func TestParseTimestampRequiresRFC3339AndNormalizesUTC(t *testing.T) {
	parsed, err := ParseTimestamp("2030-01-02T04:00:00+02:00")
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Location() != time.UTC || parsed.Hour() != 2 {
		t.Fatalf("timestamp not normalized to UTC: %v", parsed)
	}
	if _, err := ParseTimestamp("2030-01-02 04:00:00"); err == nil {
		t.Fatal("expected timestamp without timezone to fail")
	}
}

func TestBookingServiceValidatesTimeAndCapacityInputs(t *testing.T) {
	store := &bookingStoreStub{}
	service := &BookingService{bookings: store}
	start := time.Now().UTC().Add(time.Hour)
	b := models.Booking{RoomID: 1, UserID: 3, Title: "Planning", AttendeeCount: 4, StartTime: start, EndTime: start.Add(time.Hour)}
	if _, err := service.Create(context.Background(), b); err != nil {
		t.Fatal(err)
	}
	if store.created.UserID != 3 {
		t.Fatalf("created booking user = %d, want 3", store.created.UserID)
	}
	b.EndTime = b.StartTime.Add(10 * time.Minute)
	if _, err := service.Create(context.Background(), b); err != ErrBookingDuration {
		t.Fatalf("short duration error = %v", err)
	}
	b.StartTime = time.Now().UTC().Add(-time.Hour)
	b.EndTime = b.StartTime.Add(time.Hour)
	if _, err := service.Create(context.Background(), b); err != ErrBookingInPast {
		t.Fatalf("past booking error = %v", err)
	}
}

func TestBookingServiceRestrictsOwnershipAndBoundsLists(t *testing.T) {
	store := &bookingStoreStub{booking: models.Booking{ID: 7, UserID: 8, Status: "active"}}
	service := &BookingService{bookings: store}
	if _, err := service.Get(context.Background(), 7, 9, false); err != ErrBookingForbidden {
		t.Fatalf("Get error = %v", err)
	}
	if _, err := service.Cancel(context.Background(), 7, 9, false); err != ErrBookingForbidden {
		t.Fatalf("Cancel error = %v", err)
	}
	if _, err := service.Cancel(context.Background(), 7, 9, true); err != nil {
		t.Fatalf("admin cancel: %v", err)
	}
	userID := int64(8)
	if _, _, err := service.List(context.Background(), &userID, "active", 500, -1); err != nil {
		t.Fatal(err)
	}
	if store.limit != 100 || store.offset != 0 || store.userID == nil || *store.userID != userID {
		t.Fatalf("unexpected list filter: %#v", store)
	}
}
