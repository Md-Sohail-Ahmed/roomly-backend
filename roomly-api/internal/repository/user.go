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
var ErrLastActiveAdmin = errors.New("cannot deactivate or demote the last active admin")
var ErrUserNotFound = errors.New("user not found")

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
		INSERT INTO users (name, email, password_hash, role)
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
		SELECT id, name, email, password_hash, role,
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
		SELECT id, name, email, password_hash, role,
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

func (r *UserRepository) AuthorizationState(ctx context.Context, id int64) (bool, string, error) {
	var active bool
	var role string
	err := r.db.QueryRowContext(ctx, `SELECT is_active, role FROM users WHERE id = ?`, id).Scan(&active, &role)
	if errors.Is(err, sql.ErrNoRows) {
		return false, "", sql.ErrNoRows
	}
	if err != nil {
		return false, "", fmt.Errorf("check user authorization state: %w", err)
	}
	return active, role, nil
}

func (r *UserRepository) List(ctx context.Context, limit, offset int) ([]models.User, int, error) {
	var total int
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users`).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count users: %w", err)
	}
	rows, err := r.db.QueryContext(ctx, `SELECT id, name, email, role, is_active, created_at, updated_at FROM users ORDER BY id LIMIT ? OFFSET ?`, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list users: %w", err)
	}
	defer rows.Close()
	users := make([]models.User, 0)
	for rows.Next() {
		var user models.User
		if err := rows.Scan(&user.ID, &user.Name, &user.Email, &user.Role, &user.IsActive, &user.CreatedAt, &user.UpdatedAt); err != nil {
			return nil, 0, fmt.Errorf("scan user: %w", err)
		}
		users = append(users, user)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate users: %w", err)
	}
	return users, total, nil
}

func (r *UserRepository) FindByID(ctx context.Context, id int64) (models.User, error) {
	var user models.User
	err := r.db.QueryRowContext(ctx, `SELECT id, name, email, password_hash, role, is_active, created_at, updated_at FROM users WHERE id = ?`, id).Scan(&user.ID, &user.Name, &user.Email, &user.PasswordHash, &user.Role, &user.IsActive, &user.CreatedAt, &user.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return models.User{}, ErrUserNotFound
	}
	if err != nil {
		return models.User{}, fmt.Errorf("find user by ID: %w", err)
	}
	return user, nil
}

func (r *UserRepository) SetActive(ctx context.Context, id int64, active bool) (models.User, error) {
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return models.User{}, fmt.Errorf("begin user status transaction: %w", err)
	}
	defer tx.Rollback()
	adminCount, err := lockActiveAdmins(ctx, tx)
	if err != nil {
		return models.User{}, err
	}
	var role string
	var currentActive bool
	err = tx.QueryRowContext(ctx, `SELECT role, is_active FROM users WHERE id = ? FOR UPDATE`, id).Scan(&role, &currentActive)
	if errors.Is(err, sql.ErrNoRows) {
		return models.User{}, ErrUserNotFound
	}
	if err != nil {
		return models.User{}, fmt.Errorf("lock target user: %w", err)
	}
	if role == "admin" && currentActive && !active && adminCount <= 1 {
		return models.User{}, ErrLastActiveAdmin
	}
	if _, err := tx.ExecContext(ctx, `UPDATE users SET is_active = ? WHERE id = ?`, active, id); err != nil {
		return models.User{}, fmt.Errorf("update user status: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return models.User{}, fmt.Errorf("commit user status transaction: %w", err)
	}
	return r.FindByID(ctx, id)
}

func (r *UserRepository) SetRole(ctx context.Context, id int64, role string) (models.User, error) {
	if role != "employee" && role != "admin" {
		return models.User{}, ErrInvalidRole
	}
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return models.User{}, fmt.Errorf("begin user role transaction: %w", err)
	}
	defer tx.Rollback()
	adminCount, err := lockActiveAdmins(ctx, tx)
	if err != nil {
		return models.User{}, err
	}
	var currentRole string
	var active bool
	err = tx.QueryRowContext(ctx, `SELECT role, is_active FROM users WHERE id = ? FOR UPDATE`, id).Scan(&currentRole, &active)
	if errors.Is(err, sql.ErrNoRows) {
		return models.User{}, ErrUserNotFound
	}
	if err != nil {
		return models.User{}, fmt.Errorf("lock target user: %w", err)
	}
	if currentRole == "admin" && role != "admin" && active && adminCount <= 1 {
		return models.User{}, ErrLastActiveAdmin
	}
	if _, err := tx.ExecContext(ctx, `UPDATE users SET role = ? WHERE id = ?`, role, id); err != nil {
		return models.User{}, fmt.Errorf("update user role: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return models.User{}, fmt.Errorf("commit user role transaction: %w", err)
	}
	return r.FindByID(ctx, id)
}

var ErrInvalidRole = errors.New("role must be employee or admin")
var ErrAdminBootstrapComplete = errors.New("an admin account already exists")
var ErrBootstrapUserNotFound = errors.New("bootstrap user not found")
var ErrBootstrapUserInactive = errors.New("bootstrap user must be active")

// PromoteFirstAdmin promotes an already registered employee only when no
// admin exists. A MySQL named lock serializes concurrent bootstrap commands.
func (r *UserRepository) PromoteFirstAdmin(ctx context.Context, email string) error {
	conn, err := r.db.Conn(ctx)
	if err != nil {
		return fmt.Errorf("open bootstrap connection: %w", err)
	}
	defer conn.Close()
	var acquired sql.NullInt64
	if err := conn.QueryRowContext(ctx, `SELECT GET_LOCK('roomly_admin_bootstrap', 10)`).Scan(&acquired); err != nil {
		return fmt.Errorf("acquire admin bootstrap lock: %w", err)
	}
	if !acquired.Valid || acquired.Int64 != 1 {
		return errors.New("could not acquire admin bootstrap lock")
	}
	defer func() { _, _ = conn.ExecContext(context.Background(), `SELECT RELEASE_LOCK('roomly_admin_bootstrap')`) }()
	tx, err := conn.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return fmt.Errorf("begin admin bootstrap transaction: %w", err)
	}
	defer tx.Rollback()
	var count int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM users WHERE role = 'admin'`).Scan(&count); err != nil {
		return fmt.Errorf("check existing admins: %w", err)
	}
	if count != 0 {
		return ErrAdminBootstrapComplete
	}
	var id int64
	var role string
	var active bool
	err = tx.QueryRowContext(ctx, `SELECT id, role, is_active FROM users WHERE email = ? FOR UPDATE`, email).Scan(&id, &role, &active)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrBootstrapUserNotFound
	}
	if err != nil {
		return fmt.Errorf("lock bootstrap user: %w", err)
	}
	if !active {
		return ErrBootstrapUserInactive
	}
	if role != "employee" {
		return ErrAdminBootstrapComplete
	}
	if _, err := tx.ExecContext(ctx, `UPDATE users SET role = 'admin' WHERE id = ?`, id); err != nil {
		return fmt.Errorf("promote bootstrap user: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit admin bootstrap: %w", err)
	}
	return nil
}

func lockActiveAdmins(ctx context.Context, tx *sql.Tx) (int, error) {
	// Lock active admin rows in a stable order before locking the target user.
	// Concurrent privilege changes then serialize around the same rows, preventing
	// two requests from both demoting what each believes is not the final admin.
	rows, err := tx.QueryContext(ctx, `SELECT id FROM users WHERE role = 'admin' AND is_active = TRUE ORDER BY id FOR UPDATE`)
	if err != nil {
		return 0, fmt.Errorf("lock active admins: %w", err)
	}
	defer rows.Close()
	count := 0
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return 0, fmt.Errorf("scan active admin: %w", err)
		}
		count++
	}
	if err := rows.Err(); err != nil {
		return 0, fmt.Errorf("iterate active admins: %w", err)
	}
	return count, nil
}
