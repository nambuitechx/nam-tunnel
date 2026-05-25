package relay_models

import "time"

type UserPat struct {
	ID         string     `json:"id"`
	UserID     string     `json:"user_id"`
	TokenHash  string     `json:"-"`
	CreatedAt  time.Time  `json:"created_at"`
	ExpiresAt  time.Time  `json:"expires_at"`
	LastUsedAt *time.Time `json:"last_used_at"`
}
