package platform

import (
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/uptrace/bun"
)

// OperatorMFARecoveryCode is a single Argon2id-hashed single-use recovery
// code for a moto-Operator. Mirror of auth.MFARecoveryCode against
// platform.operator_mfa_recovery_codes.
type OperatorMFARecoveryCode struct {
	base.Model `bun:"schema:platform,table:operator_mfa_recovery_codes"`
	OperatorID int64      `bun:"operator_id,notnull" json:"operator_id"`
	CodeHash   string     `bun:"code_hash,notnull" json:"-"`
	UsedAt     *time.Time `bun:"used_at" json:"used_at,omitempty"`
}

func (c *OperatorMFARecoveryCode) BeforeAppendModel(query any) error {
	if q, ok := query.(*bun.UpdateQuery); ok {
		q.ModelTableExpr(`platform.operator_mfa_recovery_codes AS "operator_mfa_recovery_code"`)
	}
	if q, ok := query.(*bun.DeleteQuery); ok {
		q.ModelTableExpr(`platform.operator_mfa_recovery_codes AS "operator_mfa_recovery_code"`)
	}
	return nil
}

func (c *OperatorMFARecoveryCode) TableName() string {
	return "platform.operator_mfa_recovery_codes"
}

// IsUsed returns true once a code has been redeemed.
func (c *OperatorMFARecoveryCode) IsUsed() bool { return c.UsedAt != nil }

func (c *OperatorMFARecoveryCode) GetID() interface{}      { return c.ID }
func (c *OperatorMFARecoveryCode) GetCreatedAt() time.Time { return c.CreatedAt }
func (c *OperatorMFARecoveryCode) GetUpdatedAt() time.Time { return c.UpdatedAt }
