package relay_models

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrUserPatNotFound = errors.New("user pat not found")

type UserPatRepository struct {
	db *pgxpool.Pool
}

func NewUserPatRepository(db *pgxpool.Pool) *UserPatRepository {
	return &UserPatRepository{db: db}
}

func (r *UserPatRepository) Create(ctx context.Context, userID, tokenHash string, expiresAt time.Time) (*UserPat, error) {
	pat := &UserPat{}
	err := r.db.QueryRow(ctx, `
		INSERT INTO user_pats (user_id, token_hash, expires_at)
		VALUES ($1, $2, $3)
		RETURNING id, user_id, token_hash, created_at, expires_at, last_used_at
	`, userID, tokenHash, expiresAt).Scan(
		&pat.ID,
		&pat.UserID,
		&pat.TokenHash,
		&pat.CreatedAt,
		&pat.ExpiresAt,
		&pat.LastUsedAt,
	)
	if err != nil {
		return nil, err
	}
	return pat, nil
}

func (r *UserPatRepository) GetByID(ctx context.Context, id string) (*UserPat, error) {
	pat := &UserPat{}
	err := r.db.QueryRow(ctx, `
		SELECT id, user_id, token_hash, created_at, expires_at, last_used_at
		FROM user_pats
		WHERE id = $1
	`, id).Scan(
		&pat.ID,
		&pat.UserID,
		&pat.TokenHash,
		&pat.CreatedAt,
		&pat.ExpiresAt,
		&pat.LastUsedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUserPatNotFound
		}
		return nil, err
	}
	return pat, nil
}

func (r *UserPatRepository) GetByTokenHash(ctx context.Context, tokenHash string) (*UserPat, error) {
	pat := &UserPat{}
	err := r.db.QueryRow(ctx, `
		SELECT id, user_id, token_hash, created_at, expires_at, last_used_at
		FROM user_pats
		WHERE token_hash = $1
	`, tokenHash).Scan(
		&pat.ID,
		&pat.UserID,
		&pat.TokenHash,
		&pat.CreatedAt,
		&pat.ExpiresAt,
		&pat.LastUsedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUserPatNotFound
		}
		return nil, err
	}
	return pat, nil
}

func (r *UserPatRepository) ListByUserID(ctx context.Context, userID string) ([]UserPat, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, user_id, token_hash, created_at, expires_at, last_used_at
		FROM user_pats
		WHERE user_id = $1
		ORDER BY created_at DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	pats := []UserPat{}
	for rows.Next() {
		var pat UserPat
		if err := rows.Scan(
			&pat.ID,
			&pat.UserID,
			&pat.TokenHash,
			&pat.CreatedAt,
			&pat.ExpiresAt,
			&pat.LastUsedAt,
		); err != nil {
			return nil, err
		}
		pats = append(pats, pat)
	}
	return pats, rows.Err()
}

func (r *UserPatRepository) GetByIDForUser(ctx context.Context, id, userID string) (*UserPat, error) {
	pat := &UserPat{}
	err := r.db.QueryRow(ctx, `
		SELECT id, user_id, token_hash, created_at, expires_at, last_used_at
		FROM user_pats
		WHERE id = $1 AND user_id = $2
	`, id, userID).Scan(
		&pat.ID,
		&pat.UserID,
		&pat.TokenHash,
		&pat.CreatedAt,
		&pat.ExpiresAt,
		&pat.LastUsedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUserPatNotFound
		}
		return nil, err
	}
	return pat, nil
}

func (r *UserPatRepository) DeleteForUser(ctx context.Context, id, userID string) error {
	tag, err := r.db.Exec(ctx, `DELETE FROM user_pats WHERE id = $1 AND user_id = $2`, id, userID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrUserPatNotFound
	}
	return nil
}

func (r *UserPatRepository) TouchLastUsed(ctx context.Context, id string) error {
	now := time.Now().UTC()
	tag, err := r.db.Exec(ctx, `
		UPDATE user_pats
		SET last_used_at = $2
		WHERE id = $1
	`, id, now)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrUserPatNotFound
	}
	return nil
}

func (r *UserPatRepository) Delete(ctx context.Context, id string) error {
	tag, err := r.db.Exec(ctx, `DELETE FROM user_pats WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrUserPatNotFound
	}
	return nil
}

func (r *UserPatRepository) DeleteByUserID(ctx context.Context, userID string) error {
	_, err := r.db.Exec(ctx, `DELETE FROM user_pats WHERE user_id = $1`, userID)
	return err
}
