package platform

import (
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/uptrace/bun"
)

// OperatorMFAMethod constants accepted by the v1 implementation.
const (
	OperatorMFAMethodEmail = "email"
)

// OperatorMFACredential records that a moto-Operator has enrolled in MFA.
// Mirror of auth.MFACredential against platform.operator_mfa_credentials.
type OperatorMFACredential struct {
	base.Model `bun:"schema:platform,table:operator_mfa_credentials"`
	OperatorID int64      `bun:"operator_id,notnull" json:"operator_id"`
	Method     string     `bun:"method,notnull" json:"method"`
	EnrolledAt time.Time  `bun:"enrolled_at,notnull,default:current_timestamp" json:"enrolled_at"`
	LastUsedAt *time.Time `bun:"last_used_at" json:"last_used_at,omitempty"`
}

// BeforeAppendModel keeps the schema-qualified table name on UPDATE/DELETE.
func (c *OperatorMFACredential) BeforeAppendModel(query any) error {
	if q, ok := query.(*bun.UpdateQuery); ok {
		q.ModelTableExpr(`platform.operator_mfa_credentials AS "operator_mfa_credential"`)
	}
	if q, ok := query.(*bun.DeleteQuery); ok {
		q.ModelTableExpr(`platform.operator_mfa_credentials AS "operator_mfa_credential"`)
	}
	return nil
}

// Validate ensures the credential references a valid method.
func (c *OperatorMFACredential) Validate() error {
	if c.OperatorID == 0 {
		return errors.New("operator_id is required")
	}
	if c.Method != OperatorMFAMethodEmail {
		return errors.New("unsupported MFA method")
	}
	return nil
}
