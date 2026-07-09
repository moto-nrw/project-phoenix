package auth

import (
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
)

// PasswordResetToken represents a token used for password reset operations
type PasswordResetToken struct {
	base.Model      `bun:"schema:auth,table:password_reset_tokens"`
	AccountID       int64      `bun:"account_id,notnull" json:"account_id"`
	Token           string     `bun:"token,notnull" json:"token"`
	Expiry          time.Time  `bun:"expiry,notnull" json:"expiry"`
	Used            bool       `bun:"used,notnull,default:false" json:"used"`
	EmailSentAt     *time.Time `bun:"email_sent_at,nullzero" json:"email_sent_at,omitempty"`
	EmailError      *string    `bun:"email_error,nullzero" json:"email_error,omitempty"`
	EmailRetryCount int        `bun:"email_retry_count,notnull,default:0" json:"email_retry_count"`

	// Relations
	Account *Account `bun:"rel:belongs-to,join:account_id=id" json:"account,omitempty"`
}

// Validate ensures password reset token data is valid. It performs pure field
// validation only. Expiry/used consumability is wall-clock policy owned by the
// repository's FindValidByToken, per issue #586 (Rule 12: models hold data, not
// decisions).
func (t *PasswordResetToken) Validate() error {
	if t.AccountID <= 0 {
		return errors.New("account ID is required")
	}

	if t.Token == "" {
		return errors.New("token value is required")
	}

	return nil
}

// SetExpiry sets the token expiry time to a specified duration from now
func (t *PasswordResetToken) SetExpiry(duration time.Duration) {
	t.Expiry = time.Now().Add(duration)
}
