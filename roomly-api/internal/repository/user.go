package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	mysql "github.com/go-sql-driver/mysql"

	"roomly-api/internal/models"
)

var ErrEmailAlreadyExists = errors.New("email already exists")

type UserRepository struct {
	db *sql.DB
}

func NewUserRepository(db *sql.DB) *UserRepository {
	return &UserRepository{db: db}
}

func (r *UserRepository) Create(
	ctx context.Context,
	name string,
	email string,
	passwordHash string,
) (models.User, error) {
	query := `
		INSERT INTO users (name, email, password, role)
		VALUES (?, ?, ?, 'employee')
	`

	result, err := r.db.ExecContext(
		ctx,
		query,
		name,
		email,
		passwordHash,
	)
	if err != nil {
		var mysqlErr *mysql.MySQLError
		if errors.As(err, &mysqlErr) && mysqlErr.Number == 1062 {
			return models.User{}, ErrEmailAlreadyExists
		}

		return models.User{}, fmt.Errorf("insert user: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return models.User{}, fmt.Errorf("get inserted user ID: %w", err)
	}

	var user models.User

	selectQuery := `
		SELECT id, name, email, password, role,
		       is_active, created_at, updated_at
		FROM users
		WHERE id = ?
	`

	err = r.db.QueryRowContext(ctx, selectQuery, id).Scan(
		&user.ID,
		&user.Name,
		&user.Email,
		&user.PasswordHash,
		&user.Role,
		&user.IsActive,
		&user.CreatedAt,
		&user.UpdatedAt,
	)
	if err != nil {
		return models.User{}, fmt.Errorf("fetch created user: %w", err)
	}

	return user, nil
}

func (r *UserRepository) FindByEmail(
	ctx context.Context,
	email string,
) (models.User, error) {
	query := `
		SELECT id, name, email, password, role,
		       is_active, created_at, updated_at
		FROM users
		WHERE email = ?
		LIMIT 1
	`

	var user models.User

	err := r.db.QueryRowContext(ctx, query, email).Scan(
		&user.ID,
		&user.Name,
		&user.Email,
		&user.PasswordHash,
		&user.Role,
		&user.IsActive,
		&user.CreatedAt,
		&user.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return models.User{}, sql.ErrNoRows
		}

		return models.User{}, fmt.Errorf("find user by email: %w", err)
	}

	return user, nil
}

func (r *UserRepository) IsActive(ctx context.Context, id int64) (bool, error) {
	var active bool
	err := r.db.QueryRowContext(ctx, `SELECT is_active FROM users WHERE id = ?`, id).Scan(&active)
	if errors.Is(err, sql.ErrNoRows) {
		return false, sql.ErrNoRows
	}
	if err != nil {
		return false, fmt.Errorf("check user status: %w", err)
	}
	return active, nil
}
