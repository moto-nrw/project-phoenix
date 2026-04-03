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
	DisplayName     *string    `bun:"display_name,nullzero" json:"display_name,omitempty"`
	Token           string     `bun:"token,notnull" json:"token"`
	Expiry          time.Time  `bun:"expiry,notnull" json:"expiry"`
	Used            bool       `bun:"used,notnull,default:false" json:"used"`
	InvitedBy       int64      `bun:"invited_by,notnull" json:"invited_by"`
	EmailSentAt     *time.Time `bun:"email_sent_at,nullzero" json:"email_sent_at,omitempty"`
	EmailError      *string    `bun:"email_error,nullzero" json:"email_error,omitempty"`
	EmailRetryCount int        `bun:"email_retry_count,notnull,default:0" json:"email_retry_count"`

	// Relations
	Inviter *Operator `bun:"rel:belongs-to,join:invited_by=id" json:"inviter,omitempty"`
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

// Validate ensures the invitation token data is valid
func (t *OperatorInvitationToken) Validate() error {
	if t.Email == "" {
		return errors.New("email is required")
	}
	if t.Token == "" {
		return errors.New("token value is required")
	}
	if t.InvitedBy <= 0 {
		return errors.New("invited_by operator ID is required")
	}
	if t.Expiry.Before(time.Now()) {
		return errors.New("token has already expired")
	}
	if t.Used {
		return errors.New("token has already been used")
	}
	return nil
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
