package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"roomly-api/internal/models"
)

type RoomRepository struct{ db *sql.DB }

func NewRoomRepository(db *sql.DB) *RoomRepository { return &RoomRepository{db: db} }

func (r *RoomRepository) List(ctx context.Context, filter models.RoomFilter) ([]models.Room, int, error) {
	where := " WHERE 1=1"
	args := make([]any, 0, 2)
	if filter.Location != "" {
		where += " AND location LIKE ?"
		args = append(args, "%"+filter.Location+"%")
	}
	if filter.MinCapacity > 0 {
		where += " AND capacity >= ?"
		args = append(args, filter.MinCapacity)
	}
	var total int
	if err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM rooms"+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count rooms: %w", err)
	}
	query := `SELECT id, name, capacity, location, is_active, created_at, updated_at FROM rooms` + where + ` ORDER BY id LIMIT ? OFFSET ?`
	queryArgs := append(append([]any{}, args...), filter.Limit, filter.Offset)
	rows, err := r.db.QueryContext(ctx, query, queryArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("list rooms: %w", err)
	}
	defer rows.Close()
	rooms := make([]models.Room, 0)
	ids := make([]int64, 0)
	for rows.Next() {
		var room models.Room
		if err := rows.Scan(&room.ID, &room.Name, &room.Capacity, &room.Location, &room.IsActive, &room.CreatedAt, &room.UpdatedAt); err != nil {
			return nil, 0, fmt.Errorf("scan room: %w", err)
		}
		room.Amenities = []models.Amenity{}
		rooms = append(rooms, room)
		ids = append(ids, room.ID)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate rooms: %w", err)
	}
	if err := r.attachAmenities(ctx, rooms, ids); err != nil {
		return nil, 0, err
	}
	return rooms, total, nil
}

func (r *RoomRepository) Get(ctx context.Context, id int64) (models.Room, error) {
	room, err := scanRoom(r.db.QueryRowContext(ctx, `SELECT id, name, capacity, location, is_active, created_at, updated_at FROM rooms WHERE id = ?`, id))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return models.Room{}, ErrRoomNotFound
		}
		return models.Room{}, fmt.Errorf("get room: %w", err)
	}
	room.Amenities = []models.Amenity{}
	return roomWithAmenities(ctx, r, room)
}

func roomWithAmenities(ctx context.Context, r *RoomRepository, room models.Room) (models.Room, error) {
	query := `SELECT a.id, a.name, a.description, a.created_at FROM amenities a JOIN room_amenities ra ON ra.amenity_id = a.id WHERE ra.room_id = ? ORDER BY a.name`
	rows, err := r.db.QueryContext(ctx, query, room.ID)
	if err != nil {
		return models.Room{}, fmt.Errorf("list room amenities: %w", err)
	}
	defer rows.Close()
	room.Amenities = make([]models.Amenity, 0)
	for rows.Next() {
		var a models.Amenity
		if err := rows.Scan(&a.ID, &a.Name, &a.Description, &a.CreatedAt); err != nil {
			return models.Room{}, fmt.Errorf("scan amenity: %w", err)
		}
		room.Amenities = append(room.Amenities, a)
	}
	if err := rows.Err(); err != nil {
		return models.Room{}, fmt.Errorf("iterate amenities: %w", err)
	}
	return room, nil
}

func scanRoom(row *sql.Row) (models.Room, error) {
	var room models.Room
	err := row.Scan(&room.ID, &room.Name, &room.Capacity, &room.Location, &room.IsActive, &room.CreatedAt, &room.UpdatedAt)
	return room, err
}

func (r *RoomRepository) Create(ctx context.Context, room models.Room) (models.Room, error) {
	result, err := r.db.ExecContext(ctx, `INSERT INTO rooms (name, capacity, location) VALUES (?, ?, ?)`, room.Name, room.Capacity, room.Location)
	if err != nil {
		return models.Room{}, fmt.Errorf("create room: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return models.Room{}, fmt.Errorf("get created room ID: %w", err)
	}
	return r.Get(ctx, id)
}

func (r *RoomRepository) Update(ctx context.Context, id int64, room models.Room) (models.Room, error) {
	result, err := r.db.ExecContext(ctx, `UPDATE rooms SET name = ?, capacity = ?, location = ? WHERE id = ?`, room.Name, room.Capacity, room.Location, id)
	if err != nil {
		return models.Room{}, fmt.Errorf("update room: %w", err)
	}
	if n, e := result.RowsAffected(); e != nil {
		return models.Room{}, fmt.Errorf("check updated room: %w", e)
	} else if n == 0 {
		if _, e := r.Get(ctx, id); e != nil {
			return models.Room{}, e
		}
	}
	return r.Get(ctx, id)
}

func (r *RoomRepository) SetActive(ctx context.Context, id int64, active bool) (models.Room, error) {
	result, err := r.db.ExecContext(ctx, `UPDATE rooms SET is_active = ? WHERE id = ?`, active, id)
	if err != nil {
		return models.Room{}, fmt.Errorf("set room active status: %w", err)
	}
	if n, e := result.RowsAffected(); e != nil {
		return models.Room{}, fmt.Errorf("check room status update: %w", e)
	} else if n == 0 {
		if _, e := r.Get(ctx, id); e != nil {
			return models.Room{}, e
		}
	}
	return r.Get(ctx, id)
}

func (r *RoomRepository) attachAmenities(ctx context.Context, rooms []models.Room, ids []int64) error {
	if len(ids) == 0 {
		return nil
	}
	marks := strings.TrimRight(strings.Repeat("?,", len(ids)), ",")
	args := make([]any, len(ids))
	for i, id := range ids {
		args[i] = id
	}
	query := `SELECT ra.room_id, a.id, a.name, a.description, a.created_at FROM room_amenities ra JOIN amenities a ON a.id = ra.amenity_id WHERE ra.room_id IN (` + marks + `) ORDER BY a.name`
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("list amenities for rooms: %w", err)
	}
	defer rows.Close()
	byID := make(map[int64]int, len(rooms))
	for i := range rooms {
		byID[rooms[i].ID] = i
	}
	for rows.Next() {
		var roomID int64
		var a models.Amenity
		if err := rows.Scan(&roomID, &a.ID, &a.Name, &a.Description, &a.CreatedAt); err != nil {
			return fmt.Errorf("scan room amenity: %w", err)
		}
		if i, ok := byID[roomID]; ok {
			rooms[i].Amenities = append(rooms[i].Amenities, a)
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate room amenities: %w", err)
	}
	return nil
}

var ErrRoomNotFound = errors.New("room not found")
