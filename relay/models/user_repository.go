package relay_models

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"
)

var (
	ErrUserNotFound       = errors.New("user not found")
	ErrUsernameTaken      = errors.New("username already taken")
	ErrInvalidCredentials = errors.New("invalid credentials")
)

type UserRepository struct {
	db *sql.DB
}

func NewUserRepository(db *sql.DB) *UserRepository {
	return &UserRepository{db: db}
}

func scanUser(row interface{ Scan(dest ...any) error }) (*User, error) {
	user := &User{}
	var createdAt, updatedAt int64
	if err := row.Scan(
		&user.ID,
		&user.Username,
		&user.Password,
		&user.Active,
		&createdAt,
		&updatedAt,
	); err != nil {
		return nil, err
	}
	user.CreatedAt = timeFromEpoch(createdAt)
	user.UpdatedAt = timeFromEpoch(updatedAt)
	return user, nil
}

func (r *UserRepository) Create(ctx context.Context, username, password string) (*User, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}

	id := uuid.New().String()
	now := unixNow()
	user, err := scanUser(r.db.QueryRowContext(ctx, `
		INSERT INTO users (id, username, password, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?)
		RETURNING id, username, password, active, created_at, updated_at
	`, id, username, string(hash), now, now))
	if err != nil {
		if isUniqueViolation(err) {
			return nil, ErrUsernameTaken
		}
		return nil, err
	}
	return user, nil
}

func (r *UserRepository) List(ctx context.Context) ([]User, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, username, password, active, created_at, updated_at
		FROM users
		ORDER BY created_at ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	users := []User{}
	for rows.Next() {
		user, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		users = append(users, *user)
	}
	return users, rows.Err()
}

func (r *UserRepository) GetByID(ctx context.Context, id string) (*User, error) {
	user, err := scanUser(r.db.QueryRowContext(ctx, `
		SELECT id, username, password, active, created_at, updated_at
		FROM users
		WHERE id = ?
	`, id))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	return user, nil
}

func (r *UserRepository) GetByUsername(ctx context.Context, username string) (*User, error) {
	user, err := scanUser(r.db.QueryRowContext(ctx, `
		SELECT id, username, password, active, created_at, updated_at
		FROM users
		WHERE username = ?
	`, username))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	return user, nil
}

func (r *UserRepository) Update(ctx context.Context, id string, password *string, active *bool) (*User, error) {
	setParts := make([]string, 0, 3)
	args := make([]any, 0, 4)

	if password != nil {
		hash, err := bcrypt.GenerateFromPassword([]byte(*password), bcrypt.DefaultCost)
		if err != nil {
			return nil, fmt.Errorf("hash password: %w", err)
		}
		setParts = append(setParts, "password = ?")
		args = append(args, string(hash))
	}
	if active != nil {
		setParts = append(setParts, "active = ?")
		args = append(args, *active)
	}

	setParts = append(setParts, "updated_at = ?")
	args = append(args, unixNow())
	args = append(args, id)

	query := fmt.Sprintf(`
		UPDATE users
		SET %s
		WHERE id = ?
		RETURNING id, username, password, active, created_at, updated_at
	`, strings.Join(setParts, ", "))

	user, err := scanUser(r.db.QueryRowContext(ctx, query, args...))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	return user, nil
}

func (r *UserRepository) Delete(ctx context.Context, id string) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM users WHERE id = ?`, id)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrUserNotFound
	}
	return nil
}

func (r *UserRepository) Authenticate(ctx context.Context, username, password string) (*User, error) {
	user, err := r.GetByUsername(ctx, username)
	if err != nil {
		if errors.Is(err, ErrUserNotFound) {
			return nil, ErrInvalidCredentials
		}
		return nil, err
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)); err != nil {
		return nil, ErrInvalidCredentials
	}
	if !user.Active {
		return nil, ErrInvalidCredentials
	}
	return user, nil
}

func isUniqueViolation(err error) bool {
	var se *sqlite.Error
	return errors.As(err, &se) && se.Code() == sqlite3.SQLITE_CONSTRAINT_UNIQUE
}
