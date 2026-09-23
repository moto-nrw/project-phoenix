package test

import (
	"time"

	"github.com/uptrace/bun"
)

// AccountFixture holds database setup data, without authentication behavior.
type AccountFixture struct {
	bun.BaseModel     `bun:"table:auth.accounts,alias:account"`
	ID                int64      `bun:"id,pk,autoincrement" json:"id"`
	CreatedAt         time.Time  `bun:"created_at,nullzero,notnull,default:current_timestamp" json:"created_at"`
	UpdatedAt         time.Time  `bun:"updated_at,nullzero,notnull,default:current_timestamp" json:"updated_at"`
	Email             string     `bun:"email,notnull" json:"email"`
	Username          *string    `bun:"username,unique" json:"username,omitempty"`
	Avatar            string     `bun:"avatar" json:"avatar,omitempty"`
	Active            bool       `bun:"active,notnull,default:true" json:"active"`
	PasswordHash      *string    `bun:"password_hash" json:"-"`
	IsPasswordOTP     bool       `bun:"is_password_otp,default:false" json:"is_password_otp"`
	LastLogin         *time.Time `bun:"last_login" json:"last_login,omitempty"`
	PINHash           *string    `bun:"pin_hash" json:"-"`
	PINAttempts       int        `bun:"pin_attempts,default:0" json:"-"`
	PINLockedUntil    *time.Time `bun:"pin_locked_until" json:"-"`
	MFAAttempts       int        `bun:"mfa_attempts,default:0" json:"-"`
	MFALockedUntil    *time.Time `bun:"mfa_locked_until" json:"-"`
	CalendarFeedToken *string    `bun:"calendar_feed_token" json:"-"`
}

// RoleFixture holds a persisted role for test setup, not authorization policy.
type RoleFixture struct {
	bun.BaseModel `bun:"table:auth.roles,alias:role"`
	ID            int64     `bun:"id,pk,autoincrement"`
	CreatedAt     time.Time `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt     time.Time `bun:"updated_at,nullzero,notnull,default:current_timestamp"`
	TenantID      *int64    `bun:"tenant_id"`
	Name          string    `bun:"name,notnull"`
	Description   string    `bun:"description"`
	IsSystem      bool      `bun:"is_system,notnull,default:false"`
	BaseRole      *string   `bun:"base_role"`
}

// PermissionFixture holds persisted permission facts for test setup.
type PermissionFixture struct {
	bun.BaseModel `bun:"table:auth.permissions,alias:permission"`
	ID            int64     `bun:"id,pk,autoincrement"`
	CreatedAt     time.Time `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt     time.Time `bun:"updated_at,nullzero,notnull,default:current_timestamp"`
	Name          string    `bun:"name,notnull,unique"`
	Description   string    `bun:"description"`
	Resource      string    `bun:"resource,notnull"`
	Action        string    `bun:"action,notnull"`
}
