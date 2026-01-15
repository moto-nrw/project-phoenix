package auth

import (
	"errors"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/core/domain/base"
	"github.com/uptrace/bun"
)

// InvitationToken represents an invitation sent to create a new account.
type InvitationToken struct {
	base.Model `bun:"schema:auth,table:invitation_tokens"`

	Email           string     `bun:"email,notnull" json:"email"`
	Token           string     `bun:"token,notnull" json:"token"`
	RoleID          int64      `bun:"role_id,notnull" json:"role_id"`
	CreatedBy       int64      `bun:"created_by,notnull" json:"created_by"`
	ExpiresAt       time.Time  `bun:"expires_at,notnull" json:"expires_at"`
	UsedAt          *time.Time `bun:"used_at,nullzero" json:"used_at,omitempty"`
	FirstName       *string    `bun:"first_name,nullzero" json:"first_name,omitempty"`
	LastName        *string    `bun:"last_name,nullzero" json:"last_name,omitempty"`
	Position        *string    `bun:"position,nullzero" json:"position,omitempty"`
	EmailSentAt     *time.Time `bun:"email_sent_at,nullzero" json:"email_sent_at,omitempty"`
	EmailError      *string    `bun:"email_error,nullzero" json:"email_error,omitempty"`
	EmailRetryCount int        `bun:"email_retry_count,notnull,default:0" json:"email_retry_count"`

	// Relations
	Role    *Role    `bun:"rel:belongs-to,join:role_id=id" json:"role,omitempty"`
	Creator *Account `bun:"rel:belongs-to,join:created_by=id" json:"creator,omitempty"`
}

// TableName returns the fully-qualified table name.
func (t *InvitationToken) TableName() string {
	return "auth.invitation_tokens"
}

// BeforeAppendModel ensures the schema-qualified table expression is used with an alias.
func (t *InvitationToken) BeforeAppendModel(query any) error {
	const tableExpr = `auth.invitation_tokens AS "invitation_token"`

	switch q := query.(type) {
	case *bun.SelectQuery:
		q.ModelTableExpr(tableExpr)
	case *bun.InsertQuery:
		q.ModelTableExpr(tableExpr)
	case *bun.UpdateQuery:
		q.ModelTableExpr(tableExpr)
	case *bun.DeleteQuery:
		q.ModelTableExpr(tableExpr)
	}
	return nil
}

// Validate ensures core fields are present and sensible.
func (t *InvitationToken) Validate() error {
	if strings.TrimSpace(t.Email) == "" {
		return errors.New("email is required")
	}
	if strings.TrimSpace(t.Token) == "" {
		return errors.New("token is required")
	}
	if t.RoleID <= 0 {
		return errors.New("role id is required")
	}
	if t.CreatedBy <= 0 {
		return errors.New("created_by is required")
	}
	if t.ExpiresAt.IsZero() {
		return errors.New("expires_at is required")
	}
	if time.Now().After(t.ExpiresAt) {
		return errors.New("invitation expiry must be in the future")
	}
	return nil
}

// IsExpired returns true if the invitation has passed its expiry.
func (t *InvitationToken) IsExpired() bool {
	return time.Now().After(t.ExpiresAt)
}

// IsUsed returns true if the invitation was already accepted or revoked.
func (t *InvitationToken) IsUsed() bool {
	return t.UsedAt != nil
}

// IsValid checks whether the invitation can still be consumed.
func (t *InvitationToken) IsValid() bool {
	return !t.IsExpired() && !t.IsUsed()
}

// MarkAsUsed sets the UsedAt timestamp to now.
func (t *InvitationToken) MarkAsUsed() {
	now := time.Now()
	t.UsedAt = &now
}

// SetExpiry assigns a duration from now as the expiry.
func (t *InvitationToken) SetExpiry(duration time.Duration) {
	t.ExpiresAt = time.Now().Add(duration)
}

// GetID returns the primary key.
func (t *InvitationToken) GetID() interface{} {
	return t.ID
}

// GetCreatedAt returns the creation timestamp.
func (t *InvitationToken) GetCreatedAt() time.Time {
	return t.CreatedAt
}

// GetUpdatedAt returns the last update timestamp.
func (t *InvitationToken) GetUpdatedAt() time.Time {
	return t.UpdatedAt
}
