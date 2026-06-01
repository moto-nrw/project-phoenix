package auth

import (
	"net"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/uptrace/bun"
)

// MFAEmailChallenge represents a single 6-digit email code issued to an account
// during MFA verification. The plaintext code is never stored — only its hash.
// `consumed_at` enforces single-use; the row stays around for audit until the
// cleanup job removes expired/consumed entries.
type MFAEmailChallenge struct {
	base.Model `bun:"schema:auth,table:mfa_email_challenges"`
	AccountID  int64      `bun:"account_id,notnull" json:"account_id"`
	CodeHash   string     `bun:"code_hash,notnull" json:"-"`
	ExpiresAt  time.Time  `bun:"expires_at,notnull" json:"expires_at"`
	ConsumedAt *time.Time `bun:"consumed_at" json:"consumed_at,omitempty"`
	IPAddress  net.IP     `bun:"ip_address,type:inet,nullzero" json:"ip_address,omitempty"`
}

// BeforeAppendModel keeps the schema-qualified table name on UPDATE/DELETE.
func (c *MFAEmailChallenge) BeforeAppendModel(query any) error {
	if q, ok := query.(*bun.UpdateQuery); ok {
		q.ModelTableExpr(`auth.mfa_email_challenges AS "mfa_email_challenge"`)
	}
	if q, ok := query.(*bun.DeleteQuery); ok {
		q.ModelTableExpr(`auth.mfa_email_challenges AS "mfa_email_challenge"`)
	}
	return nil
}

// TableName returns the database table name.
func (c *MFAEmailChallenge) TableName() string { return "auth.mfa_email_challenges" }

// IsExpired returns true once the TTL has passed.
func (c *MFAEmailChallenge) IsExpired() bool { return time.Now().After(c.ExpiresAt) }

// IsConsumed returns true once the code has been redeemed.
func (c *MFAEmailChallenge) IsConsumed() bool { return c.ConsumedAt != nil }

// GetID returns the entity's primary key. Required by base.Entity.
func (c *MFAEmailChallenge) GetID() interface{} { return c.ID }

// GetCreatedAt returns the row creation timestamp. Required by base.Entity.
func (c *MFAEmailChallenge) GetCreatedAt() time.Time { return c.CreatedAt }

// GetUpdatedAt returns the last update timestamp. Required by base.Entity.
func (c *MFAEmailChallenge) GetUpdatedAt() time.Time { return c.UpdatedAt }
