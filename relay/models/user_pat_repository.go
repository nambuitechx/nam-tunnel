package relay_models

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"
)

var ErrUserPatNotFound = errors.New("user pat not found")

type UserPatRepository struct {
	db *sql.DB
}

func NewUserPatRepository(db *sql.DB) *UserPatRepository {
	return &UserPatRepository{db: db}
}

func (r *UserPatRepository) scanPat(row interface{ Scan(dest ...any) error }) (*UserPat, error) {
	pat := &UserPat{}
	var createdAt, expiresAt int64
	var lastUsedAt sql.NullInt64
	if err := row.Scan(
		&pat.ID,
		&pat.UserID,
		&pat.TokenHash,
		&createdAt,
		&expiresAt,
		&lastUsedAt,
	); err != nil {
		return nil, err
	}
	pat.CreatedAt = timeFromEpoch(createdAt)
	pat.ExpiresAt = timeFromEpoch(expiresAt)
	pat.LastUsedAt = timeFromNullEpoch(lastUsedAt)
	return pat, nil
}

func (r *UserPatRepository) Create(ctx context.Context, userID, tokenHash string, expiresAt time.Time) (*UserPat, error) {
	id := uuid.New().String()
	now := unixNow()
	pat, err := r.scanPat(r.db.QueryRowContext(ctx, `
		INSERT INTO user_pats (id, user_id, token_hash, created_at, expires_at)
		VALUES (?, ?, ?, ?, ?)
		RETURNING id, user_id, token_hash, created_at, expires_at, last_used_at
	`, id, userID, tokenHash, now, expiresAt.UTC().Unix()))
	if err != nil {
		return nil, err
	}
	return pat, nil
}

func (r *UserPatRepository) GetByID(ctx context.Context, id string) (*UserPat, error) {
	pat, err := r.scanPat(r.db.QueryRowContext(ctx, `
		SELECT id, user_id, token_hash, created_at, expires_at, last_used_at
		FROM user_pats
		WHERE id = ?
	`, id))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrUserPatNotFound
		}
		return nil, err
	}
	return pat, nil
}

func (r *UserPatRepository) GetByTokenHash(ctx context.Context, tokenHash string) (*UserPat, error) {
	pat, err := r.scanPat(r.db.QueryRowContext(ctx, `
		SELECT id, user_id, token_hash, created_at, expires_at, last_used_at
		FROM user_pats
		WHERE token_hash = ?
	`, tokenHash))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrUserPatNotFound
		}
		return nil, err
	}
	return pat, nil
}

func (r *UserPatRepository) ListByUserID(ctx context.Context, userID string) ([]UserPat, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, user_id, token_hash, created_at, expires_at, last_used_at
		FROM user_pats
		WHERE user_id = ?
		ORDER BY created_at DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	pats := []UserPat{}
	for rows.Next() {
		pat, err := r.scanPat(rows)
		if err != nil {
			return nil, err
		}
		pats = append(pats, *pat)
	}
	return pats, rows.Err()
}

func (r *UserPatRepository) GetByIDForUser(ctx context.Context, id, userID string) (*UserPat, error) {
	pat, err := r.scanPat(r.db.QueryRowContext(ctx, `
		SELECT id, user_id, token_hash, created_at, expires_at, last_used_at
		FROM user_pats
		WHERE id = ? AND user_id = ?
	`, id, userID))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrUserPatNotFound
		}
		return nil, err
	}
	return pat, nil
}

func (r *UserPatRepository) DeleteForUser(ctx context.Context, id, userID string) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM user_pats WHERE id = ? AND user_id = ?`, id, userID)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrUserPatNotFound
	}
	return nil
}

func (r *UserPatRepository) TouchLastUsed(ctx context.Context, id string) error {
	res, err := r.db.ExecContext(ctx, `
		UPDATE user_pats
		SET last_used_at = ?
		WHERE id = ?
	`, unixNow(), id)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrUserPatNotFound
	}
	return nil
}

func (r *UserPatRepository) Delete(ctx context.Context, id string) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM user_pats WHERE id = ?`, id)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrUserPatNotFound
	}
	return nil
}

func (r *UserPatRepository) DeleteByUserID(ctx context.Context, userID string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM user_pats WHERE user_id = ?`, userID)
	return err
}
