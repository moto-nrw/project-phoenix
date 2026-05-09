package auth

import (
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/uptrace/bun"
)

// MFA method constants accepted by the v1 implementation.
const (
	MFAMethodEmail = "email"
)

// MFACredential records that a given account has enrolled in MFA via a specific
// method. v1 only supports `email`; the schema is forward-compatible if more
// methods are ever added.
type MFACredential struct {
	base.Model `bun:"schema:auth,table:mfa_credentials"`
	AccountID  int64      `bun:"account_id,notnull" json:"account_id"`
	Method     string     `bun:"method,notnull" json:"method"`
	EnrolledAt time.Time  `bun:"enrolled_at,notnull,default:current_timestamp" json:"enrolled_at"`
	LastUsedAt *time.Time `bun:"last_used_at" json:"last_used_at,omitempty"`
}

// BeforeAppendModel keeps the schema-qualified table name on UPDATE/DELETE.
func (c *MFACredential) BeforeAppendModel(query any) error {
	if q, ok := query.(*bun.UpdateQuery); ok {
		q.ModelTableExpr(`auth.mfa_credentials AS "mfa_credential"`)
	}
	if q, ok := query.(*bun.DeleteQuery); ok {
		q.ModelTableExpr(`auth.mfa_credentials AS "mfa_credential"`)
	}
	return nil
}

// TableName returns the database table name.
func (c *MFACredential) TableName() string { return "auth.mfa_credentials" }

// Validate ensures the credential references a valid method.
func (c *MFACredential) Validate() error {
	if c.AccountID == 0 {
		return errors.New("account_id is required")
	}
	if c.Method != MFAMethodEmail {
		return errors.New("unsupported MFA method")
	}
	return nil
}

// GetID returns the entity's primary key. Required by base.Entity.
func (c *MFACredential) GetID() interface{} { return c.ID }

// GetCreatedAt returns the row creation timestamp. Required by base.Entity.
func (c *MFACredential) GetCreatedAt() time.Time { return c.CreatedAt }

// GetUpdatedAt returns the last update timestamp. Required by base.Entity.
func (c *MFACredential) GetUpdatedAt() time.Time { return c.UpdatedAt }
