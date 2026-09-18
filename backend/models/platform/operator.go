package platform

import (
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
)

// Operator represents a platform operator (moto DevOps team member)
type Operator struct {
	base.Model   `bun:"schema:platform,table:operators"`
	Email        string     `bun:"email,notnull,unique" json:"email"`
	DisplayName  string     `bun:"display_name,notnull" json:"display_name"`
	PasswordHash string     `bun:"password_hash,notnull" json:"-"`
	Active       bool       `bun:"active,notnull,default:true" json:"active"`
	LastLogin    *time.Time `bun:"last_login" json:"last_login,omitempty"`

	// MFA-Lockout fields (operator MFA is hardcoded mandatory; lockout mirrors auth.accounts).
	// The lockout decision (IsMFALocked) and the counter mutations
	// (Increment/Reset) live in the operator MFA service / atomic repository,
	// not on the model — see services/platform/operator_mfa_service.go and the
	// OperatorRepository (issue #586, Rule 12).
	MFAAttempts    int        `bun:"mfa_attempts,default:0" json:"-"`
	MFALockedUntil *time.Time `bun:"mfa_locked_until" json:"-"`
}
