package domain

import "time"

// MaxDisplayNameLength bounds the admin-supplied screen name.
const MaxDisplayNameLength = 100

// Display is one registered info-point screen. Only the SHA-256 hash of its
// access token is ever stored.
type Display struct {
	ID        int64
	TenantID  int64
	CreatedAt time.Time
	UpdatedAt time.Time
	Name      string
	IsActive  bool
	TokenHash string
}

// CreateDisplay registers a screen with an already hashed token.
type CreateDisplay struct {
	Name      string
	TokenHash string
}

// UpdateDisplay carries the columns an admin may change. Nil fields stay put.
type UpdateDisplay struct {
	ID        int64
	Name      *string
	IsActive  *bool
	TokenHash *string
}
