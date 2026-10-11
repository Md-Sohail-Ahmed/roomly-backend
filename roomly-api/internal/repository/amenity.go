package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	mysql "github.com/go-sql-driver/mysql"
	"roomly-api/internal/models"
)

var (
	ErrAmenityNotFound       = errors.New("amenity not found")
	ErrAmenityExists         = errors.New("amenity already exists")
	ErrAssociationExists     = errors.New("amenity is already assigned to this room")
	ErrAssociationNotFound   = errors.New("room amenity association not found")
	ErrRoomOrAmenityNotFound = errors.New("room or amenity not found")
)

type AmenityRepository struct{ db *sql.DB }

func NewAmenityRepository(db *sql.DB) *AmenityRepository { return &AmenityRepository{db: db} }

func (r *AmenityRepository) List(ctx context.Context, limit, offset int) ([]models.Amenity, int, error) {
	var total int
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM amenities`).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count amenities: %w", err)
	}
	rows, err := r.db.QueryContext(ctx, `SELECT id, name, description, created_at FROM amenities ORDER BY name LIMIT ? OFFSET ?`, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list amenities: %w", err)
	}
	defer rows.Close()
	items := make([]models.Amenity, 0)
	for rows.Next() {
		var a models.Amenity
		if err := rows.Scan(&a.ID, &a.Name, &a.Description, &a.CreatedAt); err != nil {
			return nil, 0, fmt.Errorf("scan amenity: %w", err)
		}
		items = append(items, a)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate amenities: %w", err)
	}
	return items, total, nil
}

func (r *AmenityRepository) Get(ctx context.Context, id int64) (models.Amenity, error) {
	var a models.Amenity
	err := r.db.QueryRowContext(ctx, `SELECT id, name, description, created_at FROM amenities WHERE id = ?`, id).Scan(&a.ID, &a.Name, &a.Description, &a.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return models.Amenity{}, ErrAmenityNotFound
	}
	if err != nil {
		return models.Amenity{}, fmt.Errorf("get amenity: %w", err)
	}
	return a, nil
}

func (r *AmenityRepository) Create(ctx context.Context, a models.Amenity) (models.Amenity, error) {
	result, err := r.db.ExecContext(ctx, `INSERT INTO amenities (name, description) VALUES (?, ?)`, a.Name, a.Description)
	if err != nil {
		return models.Amenity{}, translateAmenityError(err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return models.Amenity{}, fmt.Errorf("get created amenity ID: %w", err)
	}
	return r.Get(ctx, id)
}

func (r *AmenityRepository) Update(ctx context.Context, id int64, a models.Amenity) (models.Amenity, error) {
	result, err := r.db.ExecContext(ctx, `UPDATE amenities SET name = ?, description = ? WHERE id = ?`, a.Name, a.Description, id)
	if err != nil {
		return models.Amenity{}, translateAmenityError(err)
	}
	if count, e := result.RowsAffected(); e != nil {
		return models.Amenity{}, fmt.Errorf("check amenity update: %w", e)
	} else if count == 0 {
		if _, e := r.Get(ctx, id); e != nil {
			return models.Amenity{}, e
		}
	}
	return r.Get(ctx, id)
}

func (r *AmenityRepository) Delete(ctx context.Context, id int64) error {
	result, err := r.db.ExecContext(ctx, `DELETE FROM amenities WHERE id = ?`, id)
	if err != nil {
		return translateAmenityError(err)
	}
	count, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check amenity deletion: %w", err)
	}
	if count == 0 {
		return ErrAmenityNotFound
	}
	return nil
}

func (r *AmenityRepository) ListForRoom(ctx context.Context, roomID int64) ([]models.Amenity, error) {
	var exists int
	err := r.db.QueryRowContext(ctx, `SELECT 1 FROM rooms WHERE id = ?`, roomID).Scan(&exists)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrRoomNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("check room for amenities: %w", err)
	}
	rows, err := r.db.QueryContext(ctx, `SELECT a.id, a.name, a.description, a.created_at FROM amenities a JOIN room_amenities ra ON ra.amenity_id = a.id WHERE ra.room_id = ? ORDER BY a.name`, roomID)
	if err != nil {
		return nil, fmt.Errorf("list room amenities: %w", err)
	}
	defer rows.Close()
	items := make([]models.Amenity, 0)
	for rows.Next() {
		var a models.Amenity
		if err := rows.Scan(&a.ID, &a.Name, &a.Description, &a.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan room amenity: %w", err)
		}
		items = append(items, a)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate room amenities: %w", err)
	}
	return items, nil
}

func (r *AmenityRepository) AddToRoom(ctx context.Context, roomID, amenityID int64) error {
	_, err := r.db.ExecContext(ctx, `INSERT INTO room_amenities (room_id, amenity_id) VALUES (?, ?)`, roomID, amenityID)
	if err == nil {
		return nil
	}
	var mysqlErr *mysql.MySQLError
	if errors.As(err, &mysqlErr) {
		switch mysqlErr.Number {
		case 1062:
			return ErrAssociationExists
		case 1452:
			return ErrRoomOrAmenityNotFound
		}
	}
	return fmt.Errorf("associate amenity with room: %w", err)
}

func (r *AmenityRepository) RemoveFromRoom(ctx context.Context, roomID, amenityID int64) error {
	result, err := r.db.ExecContext(ctx, `DELETE FROM room_amenities WHERE room_id = ? AND amenity_id = ?`, roomID, amenityID)
	if err != nil {
		return fmt.Errorf("remove room amenity: %w", err)
	}
	count, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check room amenity removal: %w", err)
	}
	if count == 0 {
		return ErrAssociationNotFound
	}
	return nil
}

func translateAmenityError(err error) error {
	var mysqlErr *mysql.MySQLError
	if errors.As(err, &mysqlErr) {
		switch mysqlErr.Number {
		case 1062:
			return ErrAmenityExists
		case 1451:
			return fmt.Errorf("amenity is still referenced: %w", err)
		}
	}
	return fmt.Errorf("amenity database operation: %w", err)
}
