package platform

import (
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/uptrace/bun"
)

// tablePlatformOperatorInvitationTokens is the schema-qualified table name
const tablePlatformOperatorInvitationTokens = "platform.operator_invitation_tokens"

// OperatorInvitationToken represents a pending operator invitation
type OperatorInvitationToken struct {
	base.Model      `bun:"schema:platform,table:operator_invitation_tokens"`
	Email           string     `bun:"email,notnull" json:"email"`
	Token           string     `bun:"token,notnull" json:"token"`
	ExpiresAt       time.Time  `bun:"expires_at,notnull" json:"expires_at"`
	UsedAt          *time.Time `bun:"used_at,nullzero" json:"used_at,omitempty"`
	CreatedBy       int64      `bun:"created_by,notnull" json:"created_by"`
	DisplayName     *string    `bun:"display_name,nullzero" json:"display_name,omitempty"`
	EmailSentAt     *time.Time `bun:"email_sent_at,nullzero" json:"email_sent_at,omitempty"`
	EmailError      *string    `bun:"email_error,nullzero" json:"email_error,omitempty"`
	EmailRetryCount int        `bun:"email_retry_count,notnull,default:0" json:"email_retry_count"`

	// Relations
	Creator *Operator `bun:"rel:belongs-to,join:created_by=id" json:"creator,omitempty"`
}

// TableName returns the database table name
func (t *OperatorInvitationToken) TableName() string {
	return tablePlatformOperatorInvitationTokens
}

// BeforeAppendModel sets the schema-qualified table expression for UPDATE and DELETE queries.
// This is NOT inherited from base.Model and must be explicitly implemented.
func (t *OperatorInvitationToken) BeforeAppendModel(query any) error {
	if q, ok := query.(*bun.UpdateQuery); ok {
		q.ModelTableExpr(tablePlatformOperatorInvitationTokens)
	}
	if q, ok := query.(*bun.DeleteQuery); ok {
		q.ModelTableExpr(tablePlatformOperatorInvitationTokens)
	}
	return nil
}

// Validate ensures the token data is valid
func (t *OperatorInvitationToken) Validate() error {
	if t.Email == "" {
		return errors.New("email is required")
	}
	if t.Token == "" {
		return errors.New("token value is required")
	}
	if t.CreatedBy <= 0 {
		return errors.New("created_by operator ID is required")
	}
	if t.ExpiresAt.Before(time.Now()) {
		return errors.New("token has already expired")
	}
	if t.UsedAt != nil {
		return errors.New("token has already been used")
	}
	return nil
}

// IsExpired checks if the token has expired
func (t *OperatorInvitationToken) IsExpired() bool {
	return time.Now().After(t.ExpiresAt)
}

// IsUsed checks if the token has been used
func (t *OperatorInvitationToken) IsUsed() bool {
	return t.UsedAt != nil
}

// IsValid checks if the token is still usable
func (t *OperatorInvitationToken) IsValid() bool {
	return !t.IsExpired() && !t.IsUsed()
}

// GetID returns the entity's ID
func (t *OperatorInvitationToken) GetID() interface{} {
	return t.ID
}

// GetCreatedAt returns the creation timestamp
func (t *OperatorInvitationToken) GetCreatedAt() time.Time {
	return t.CreatedAt
}

// GetUpdatedAt returns the last update timestamp
func (t *OperatorInvitationToken) GetUpdatedAt() time.Time {
	return t.UpdatedAt
}
