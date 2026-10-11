package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"roomly-api/internal/models"
)

var ErrRoomInactive = errors.New("room is inactive")
var ErrBookingStartPassed = errors.New("booking start time must be in the future")

type BookingRepository struct{ db *sql.DB }

func NewBookingRepository(db *sql.DB) *BookingRepository { return &BookingRepository{db: db} }

func (r *BookingRepository) CheckAvailability(ctx context.Context, roomID int64, start, end time.Time) (bool, error) {
	var active bool
	err := r.db.QueryRowContext(ctx, `SELECT is_active FROM rooms WHERE id = ?`, roomID).Scan(&active)
	if errors.Is(err, sql.ErrNoRows) {
		return false, ErrRoomNotFound
	}
	if err != nil {
		return false, fmt.Errorf("check availability room: %w", err)
	}
	if !active {
		return false, ErrRoomInactive
	}
	var conflict bool
	err = r.db.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM bookings WHERE room_id = ? AND status = 'active' AND start_time < ? AND end_time > ?)`, roomID, end, start).Scan(&conflict)
	if err != nil {
		return false, fmt.Errorf("check booking overlap: %w", err)
	}
	return !conflict, nil
}

// Create locks the parent room row before checking for overlap. Concurrent
// creators for one room therefore serialize even when no booking row exists
// yet; every create path must use this transaction and lock order.
func (r *BookingRepository) Create(ctx context.Context, booking models.Booking) (models.Booking, error) {
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return models.Booking{}, fmt.Errorf("begin booking transaction: %w", err)
	}
	defer tx.Rollback()
	var capacity int
	var active bool
	err = tx.QueryRowContext(ctx, `SELECT capacity, is_active FROM rooms WHERE id = ? FOR UPDATE`, booking.RoomID).Scan(&capacity, &active)
	if errors.Is(err, sql.ErrNoRows) {
		return models.Booking{}, ErrRoomNotFound
	}
	if err != nil {
		return models.Booking{}, fmt.Errorf("lock booking room: %w", err)
	}
	if !active {
		return models.Booking{}, ErrRoomInactive
	}
	if !booking.StartTime.After(time.Now().UTC()) {
		return models.Booking{}, ErrBookingStartPassed
	}
	if booking.AttendeeCount > capacity {
		return models.Booking{}, ErrBookingCapacity
	}
	var conflict bool
	err = tx.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM bookings WHERE room_id = ? AND status = 'active' AND start_time < ? AND end_time > ?)`, booking.RoomID, booking.EndTime, booking.StartTime).Scan(&conflict)
	if err != nil {
		return models.Booking{}, fmt.Errorf("check booking overlap: %w", err)
	}
	if conflict {
		return models.Booking{}, ErrBookingConflict
	}
	result, err := tx.ExecContext(ctx, `INSERT INTO bookings (room_id, user_id, title, attendee_count, start_time, end_time) VALUES (?, ?, ?, ?, ?, ?)`, booking.RoomID, booking.UserID, booking.Title, booking.AttendeeCount, booking.StartTime.UTC(), booking.EndTime.UTC())
	if err != nil {
		return models.Booking{}, fmt.Errorf("insert booking: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return models.Booking{}, fmt.Errorf("get created booking ID: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return models.Booking{}, fmt.Errorf("commit booking transaction: %w", err)
	}
	return r.Get(ctx, id)
}

var ErrBookingCapacity = errors.New("attendee count exceeds room capacity")
var ErrBookingConflict = errors.New("room is already booked for the requested time")

func (r *BookingRepository) Get(ctx context.Context, id int64) (models.Booking, error) {
	var b models.Booking
	err := r.db.QueryRowContext(ctx, `SELECT id, room_id, user_id, title, attendee_count, start_time, end_time, status, created_at, updated_at FROM bookings WHERE id = ?`, id).Scan(&b.ID, &b.RoomID, &b.UserID, &b.Title, &b.AttendeeCount, &b.StartTime, &b.EndTime, &b.Status, &b.CreatedAt, &b.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return models.Booking{}, ErrBookingNotFound
	}
	if err != nil {
		return models.Booking{}, fmt.Errorf("get booking: %w", err)
	}
	return b, nil
}

var ErrBookingNotFound = errors.New("booking not found")

func (r *BookingRepository) List(ctx context.Context, userID *int64, status string, limit, offset int) ([]models.Booking, int, error) {
	where := " WHERE 1=1"
	args := make([]any, 0, 2)
	if userID != nil {
		where += " AND user_id = ?"
		args = append(args, *userID)
	}
	if status != "" {
		where += " AND status = ?"
		args = append(args, status)
	}
	var total int
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM bookings`+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count bookings: %w", err)
	}
	query := `SELECT id, room_id, user_id, title, attendee_count, start_time, end_time, status, created_at, updated_at FROM bookings` + where + ` ORDER BY start_time DESC LIMIT ? OFFSET ?`
	queryArgs := append(append([]any{}, args...), limit, offset)
	rows, err := r.db.QueryContext(ctx, query, queryArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("list bookings: %w", err)
	}
	defer rows.Close()
	items := make([]models.Booking, 0)
	for rows.Next() {
		var b models.Booking
		if err := rows.Scan(&b.ID, &b.RoomID, &b.UserID, &b.Title, &b.AttendeeCount, &b.StartTime, &b.EndTime, &b.Status, &b.CreatedAt, &b.UpdatedAt); err != nil {
			return nil, 0, fmt.Errorf("scan booking: %w", err)
		}
		items = append(items, b)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate bookings: %w", err)
	}
	return items, total, nil
}

// Cancel is idempotent: cancelling an already-cancelled booking returns its
// current record. Cancelled rows do not participate in overlap checks.
func (r *BookingRepository) Cancel(ctx context.Context, id int64) (models.Booking, error) {
	result, err := r.db.ExecContext(ctx, `UPDATE bookings SET status = 'cancelled' WHERE id = ? AND status = 'active'`, id)
	if err != nil {
		return models.Booking{}, fmt.Errorf("cancel booking: %w", err)
	}
	if _, err := result.RowsAffected(); err != nil {
		return models.Booking{}, fmt.Errorf("check booking cancellation: %w", err)
	}
	return r.Get(ctx, id)
}
