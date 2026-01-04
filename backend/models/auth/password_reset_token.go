package auth

import (
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/uptrace/bun"
)

// tableAuthPasswordResetTokens is the schema-qualified table name for password reset tokens
const tableAuthPasswordResetTokens = "auth.password_reset_tokens"

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

// TableName returns the database table name
func (t *PasswordResetToken) TableName() string {
	return tableAuthPasswordResetTokens
}

func (t *PasswordResetToken) BeforeAppendModel(query any) error {
	if q, ok := query.(*bun.SelectQuery); ok {
		q.ModelTableExpr(tableAuthPasswordResetTokens)
	}
	if q, ok := query.(*bun.UpdateQuery); ok {
		q.ModelTableExpr(tableAuthPasswordResetTokens)
	}
	if q, ok := query.(*bun.DeleteQuery); ok {
		q.ModelTableExpr(tableAuthPasswordResetTokens)
	}
	return nil
}

// Validate ensures password reset token data is valid
func (t *PasswordResetToken) Validate() error {
	if t.AccountID <= 0 {
		return errors.New("account ID is required")
	}

	if t.Token == "" {
		return errors.New("token value is required")
	}

	// Check if token has expired
	if t.Expiry.Before(time.Now()) {
		return errors.New("token has already expired")
	}

	// Check if token has been used
	if t.Used {
		return errors.New("token has already been used")
	}

	return nil
}

// IsExpired checks if the token has expired
func (t *PasswordResetToken) IsExpired() bool {
	return t.Expiry.Before(time.Now())
}

// IsValid checks if the token is valid (not expired and not used)
func (t *PasswordResetToken) IsValid() bool {
	return !t.IsExpired() && !t.Used
}

// MarkAsUsed marks the token as used
func (t *PasswordResetToken) MarkAsUsed() {
	t.Used = true
}

// SetExpiry sets the token expiry time to a specified duration from now
func (t *PasswordResetToken) SetExpiry(duration time.Duration) {
	t.Expiry = time.Now().Add(duration)
}

// GetID returns the entity's ID
func (m *PasswordResetToken) GetID() interface{} {
	return m.ID
}

// GetCreatedAt returns the creation timestamp
func (m *PasswordResetToken) GetCreatedAt() time.Time {
	return m.CreatedAt
}

// GetUpdatedAt returns the last update timestamp
func (m *PasswordResetToken) GetUpdatedAt() time.Time {
	return m.UpdatedAt
}
