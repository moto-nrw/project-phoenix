package auth

import (
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
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
