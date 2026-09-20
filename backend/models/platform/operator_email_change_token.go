package platform

import "time"

// OperatorEmailChangeToken represents a pending email change verification token
type OperatorEmailChangeToken struct {
	ID              int64      `bun:"id,pk,autoincrement" json:"id"`
	CreatedAt       time.Time  `bun:"created_at,nullzero,notnull,default:current_timestamp" json:"created_at"`
	UpdatedAt       time.Time  `bun:"updated_at,nullzero,notnull,default:current_timestamp" json:"updated_at"`
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
