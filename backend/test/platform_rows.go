package test

import (
	"time"

	"github.com/uptrace/bun"
)

// Operator is one platform.operators row the fixtures write. Organisation &
// Tenancy's row mapping is internal to the owner (#2748), so test support
// maps the table itself.
type Operator struct {
	PasswordAttempts    int        `bun:"password_attempts,scanonly"`
	PasswordLockedUntil *time.Time `bun:"password_locked_until,scanonly"`
	bun.BaseModel       `bun:"table:platform.operators,alias:operator"`
	ID                  int64      `bun:"id,pk,autoincrement"`
	CreatedAt           time.Time  `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt           time.Time  `bun:"updated_at,nullzero,notnull,default:current_timestamp"`
	Email               string     `bun:"email,notnull,unique"`
	DisplayName         string     `bun:"display_name,notnull"`
	PasswordHash        string     `bun:"password_hash,notnull"`
	Active              bool       `bun:"active,notnull,default:true"`
	LastLogin           *time.Time `bun:"last_login"`
	MFAAttempts         int        `bun:"mfa_attempts,default:0"`
	MFALockedUntil      *time.Time `bun:"mfa_locked_until"`
	DeactivatedAt       *time.Time `bun:"deactivated_at"`
}

// Organization is one platform.organizations row the fixtures write.
type Organization struct {
	bun.BaseModel `bun:"table:platform.organizations,alias:organization"`
	ID            int64      `bun:"id,pk,autoincrement"`
	CreatedAt     time.Time  `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt     time.Time  `bun:"updated_at,nullzero,notnull,default:current_timestamp"`
	Name          string     `bun:"name,notnull"`
	Slug          string     `bun:"slug,notnull,unique"`
	Active        bool       `bun:"active,notnull,default:true"`
	DeletedAt     *time.Time `bun:"deleted_at"`
	Settings      string     `bun:"settings,default:'{}'"`
}
