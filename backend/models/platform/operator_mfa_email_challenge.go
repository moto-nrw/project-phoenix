package platform

import (
	"net"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/uptrace/bun"
)

// OperatorMFAEmailChallenge represents a single 6-digit code emailed to a
// moto-Operator. Mirror of auth.MFAEmailChallenge against
// platform.operator_mfa_email_challenges.
type OperatorMFAEmailChallenge struct {
	base.Model `bun:"schema:platform,table:operator_mfa_email_challenges"`
	OperatorID int64      `bun:"operator_id,notnull" json:"operator_id"`
	CodeHash   string     `bun:"code_hash,notnull" json:"-"`
	ExpiresAt  time.Time  `bun:"expires_at,notnull" json:"expires_at"`
	ConsumedAt *time.Time `bun:"consumed_at" json:"consumed_at,omitempty"`
	IPAddress  net.IP     `bun:"ip_address,type:inet,nullzero" json:"ip_address,omitempty"`
}

func (c *OperatorMFAEmailChallenge) BeforeAppendModel(query any) error {
	if q, ok := query.(*bun.UpdateQuery); ok {
		q.ModelTableExpr(`platform.operator_mfa_email_challenges AS "operator_mfa_email_challenge"`)
	}
	if q, ok := query.(*bun.DeleteQuery); ok {
		q.ModelTableExpr(`platform.operator_mfa_email_challenges AS "operator_mfa_email_challenge"`)
	}
	return nil
}

func (c *OperatorMFAEmailChallenge) TableName() string {
	return "platform.operator_mfa_email_challenges"
}

// IsConsumed returns true once the code has been redeemed. This is a pure field
// accessor (ConsumedAt != nil); the wall-clock TTL/expiry decision lives in the
// service layer (services/platform.OperatorMFAEmailChallengeExpired) and the
// repository's active-challenge finder, per issue #586 (Rule 12).
func (c *OperatorMFAEmailChallenge) IsConsumed() bool { return c.ConsumedAt != nil }

// GetID, GetCreatedAt, GetUpdatedAt — base.Entity contract.
func (c *OperatorMFAEmailChallenge) GetID() interface{}      { return c.ID }
func (c *OperatorMFAEmailChallenge) GetCreatedAt() time.Time { return c.CreatedAt }
func (c *OperatorMFAEmailChallenge) GetUpdatedAt() time.Time { return c.UpdatedAt }
