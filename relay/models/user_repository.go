package relay_models

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrUserNotFound      = errors.New("user not found")
	ErrUsernameTaken     = errors.New("username already taken")
	ErrInvalidCredentials = errors.New("invalid credentials")
)

type UserRepository struct {
	db *pgxpool.Pool
}

func NewUserRepository(db *pgxpool.Pool) *UserRepository {
	return &UserRepository{db: db}
}

func (r *UserRepository) Create(ctx context.Context, username, password string) (*User, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}

	user := &User{}
	err = r.db.QueryRow(ctx, `
		INSERT INTO users (username, password)
		VALUES ($1, $2)
		RETURNING id, username, password, active, created_at, updated_at
	`, username, string(hash)).Scan(
		&user.ID,
		&user.Username,
		&user.Password,
		&user.Active,
		&user.CreatedAt,
		&user.UpdatedAt,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, ErrUsernameTaken
		}
		return nil, err
	}
	return user, nil
}

func (r *UserRepository) List(ctx context.Context) ([]User, error) {
	rows, err := r.db.Query(ctx, `
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
		var user User
		if err := rows.Scan(&user.ID, &user.Username, &user.Password, &user.Active, &user.CreatedAt, &user.UpdatedAt); err != nil {
			return nil, err
		}
		users = append(users, user)
	}
	return users, rows.Err()
}

func (r *UserRepository) GetByID(ctx context.Context, id string) (*User, error) {
	user := &User{}
	err := r.db.QueryRow(ctx, `
		SELECT id, username, password, active, created_at, updated_at
		FROM users
		WHERE id = $1
	`, id).Scan(&user.ID, &user.Username, &user.Password, &user.Active, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	return user, nil
}

func (r *UserRepository) GetByUsername(ctx context.Context, username string) (*User, error) {
	user := &User{}
	err := r.db.QueryRow(ctx, `
		SELECT id, username, password, active, created_at, updated_at
		FROM users
		WHERE username = $1
	`, username).Scan(&user.ID, &user.Username, &user.Password, &user.Active, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	return user, nil
}

func (r *UserRepository) Update(ctx context.Context, id string, password *string, active *bool) (*User, error) {
	setParts := make([]string, 0, 3)
	args := []any{id}
	argIdx := 2

	if password != nil {
		hash, err := bcrypt.GenerateFromPassword([]byte(*password), bcrypt.DefaultCost)
		if err != nil {
			return nil, fmt.Errorf("hash password: %w", err)
		}
		setParts = append(setParts, fmt.Sprintf("password = $%d", argIdx))
		args = append(args, string(hash))
		argIdx++
	}
	if active != nil {
		setParts = append(setParts, fmt.Sprintf("active = $%d", argIdx))
		args = append(args, *active)
		argIdx++
	}

	setParts = append(setParts, fmt.Sprintf("updated_at = $%d", argIdx))
	args = append(args, time.Now().UTC())

	user := &User{}
	query := fmt.Sprintf(`
		UPDATE users
		SET %s
		WHERE id = $1
		RETURNING id, username, password, active, created_at, updated_at
	`, strings.Join(setParts, ", "))

	err := r.db.QueryRow(ctx, query, args...).Scan(
		&user.ID,
		&user.Username,
		&user.Password,
		&user.Active,
		&user.CreatedAt,
		&user.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	return user, nil
}

func (r *UserRepository) Delete(ctx context.Context, id string) error {
	tag, err := r.db.Exec(ctx, `DELETE FROM users WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
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
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
