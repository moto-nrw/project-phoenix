package platform

import (
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
)

// OperatorEmailChangeToken represents a pending email change verification token
type OperatorEmailChangeToken struct {
	base.Model      `bun:"schema:platform,table:operator_email_change_tokens"`
	OperatorID      int64      `bun:"operator_id,notnull" json:"operator_id"`
	NewEmail        string     `bun:"new_email,notnull" json:"new_email"`
	Token           string     `bun:"token,notnull" json:"token"`
	Expiry          time.Time  `bun:"expiry,notnull" json:"expiry"`
	Used            bool       `bun:"used,notnull,default:false" json:"used"`
	EmailSentAt     *time.Time `bun:"email_sent_at,nullzero" json:"email_sent_at,omitempty"`
	EmailError      *string    `bun:"email_error,nullzero" json:"email_error,omitempty"`
	EmailRetryCount int        `bun:"email_retry_count,notnull,default:0" json:"email_retry_count"`

	// Relations
}

// Validate ensures the token data is valid
func (t *OperatorEmailChangeToken) Validate() error {
	if t.OperatorID <= 0 {
		return errors.New("operator ID is required")
	}
	if t.Token == "" {
		return errors.New("token value is required")
	}
	if t.NewEmail == "" {
		return errors.New("new email is required")
	}
	// Creation-time data-integrity guard: never persist an already-expired
	// token. This is distinct from the usage-time IsExpired decision (which
	// lives in the service); the repository Create path relies on it.
	if t.Expiry.Before(time.Now()) {
		return errors.New("token has already expired")
	}
	if t.Used {
		return errors.New("token has already been used")
	}
	return nil
}
