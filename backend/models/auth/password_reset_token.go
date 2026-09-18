package auth

import (
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
