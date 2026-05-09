package auth

import (
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/uptrace/bun"
)

// MFARecoveryCode represents one of the (typically 10) Argon2id-hashed,
// single-use recovery codes a user can redeem when their email channel is
// unavailable. The plaintext is shown to the user exactly once at generation;
// only the hash is persisted.
type MFARecoveryCode struct {
	base.Model `bun:"schema:auth,table:mfa_recovery_codes"`
	AccountID  int64      `bun:"account_id,notnull" json:"account_id"`
	CodeHash   string     `bun:"code_hash,notnull" json:"-"`
	UsedAt     *time.Time `bun:"used_at" json:"used_at,omitempty"`
}

// BeforeAppendModel keeps the schema-qualified table name on UPDATE/DELETE.
func (c *MFARecoveryCode) BeforeAppendModel(query any) error {
	if q, ok := query.(*bun.UpdateQuery); ok {
		q.ModelTableExpr(`auth.mfa_recovery_codes AS "mfa_recovery_code"`)
	}
	if q, ok := query.(*bun.DeleteQuery); ok {
		q.ModelTableExpr(`auth.mfa_recovery_codes AS "mfa_recovery_code"`)
	}
	return nil
}

// TableName returns the database table name.
func (c *MFARecoveryCode) TableName() string { return "auth.mfa_recovery_codes" }

// IsUsed returns true once a code has been redeemed.
func (c *MFARecoveryCode) IsUsed() bool { return c.UsedAt != nil }
