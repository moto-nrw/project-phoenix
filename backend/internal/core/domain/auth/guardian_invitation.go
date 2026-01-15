package auth

import (
	"errors"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/core/domain/base"
	"github.com/uptrace/bun"
)

// GuardianInvitation represents an invitation sent to create a guardian account
type GuardianInvitation struct {
	base.Model `bun:"schema:auth,table:guardian_invitations"`

	Token             string     `bun:"token,notnull,unique" json:"token"`
	GuardianProfileID int64      `bun:"guardian_profile_id,notnull" json:"guardian_profile_id"`
	CreatedBy         int64      `bun:"created_by,notnull" json:"created_by"`
	ExpiresAt         time.Time  `bun:"expires_at,notnull" json:"expires_at"`
	AcceptedAt        *time.Time `bun:"accepted_at" json:"accepted_at,omitempty"`
	EmailSentAt       *time.Time `bun:"email_sent_at" json:"email_sent_at,omitempty"`
	EmailError        *string    `bun:"email_error" json:"email_error,omitempty"`
	EmailRetryCount   int        `bun:"email_retry_count,default:0" json:"email_retry_count"`

	// Relations (not stored in database)
	Creator *Account `bun:"rel:belongs-to,join:created_by=id" json:"creator,omitempty"`
}

// TableName returns the fully-qualified table name
func (i *GuardianInvitation) TableName() string {
	return "auth.guardian_invitations"
}

// BeforeAppendModel ensures the schema-qualified table expression is used with an alias
func (i *GuardianInvitation) BeforeAppendModel(query any) error {
	const tableExpr = `auth.guardian_invitations AS "guardian_invitation"`

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

// Validate ensures core fields are present and sensible
func (i *GuardianInvitation) Validate() error {
	if strings.TrimSpace(i.Token) == "" {
		return errors.New("token is required")
	}
	if i.GuardianProfileID <= 0 {
		return errors.New("guardian profile ID is required")
	}
	if i.CreatedBy <= 0 {
		return errors.New("created_by is required")
	}
	if i.ExpiresAt.IsZero() {
		return errors.New("expires_at is required")
	}
	if time.Now().After(i.ExpiresAt) {
		return errors.New("invitation expiry must be in the future")
	}
	return nil
}

// IsExpired returns true if the invitation has passed its expiry
func (i *GuardianInvitation) IsExpired() bool {
	return time.Now().After(i.ExpiresAt)
}

// IsAccepted returns true if the invitation was already accepted
func (i *GuardianInvitation) IsAccepted() bool {
	return i.AcceptedAt != nil
}

// IsValid checks whether the invitation can still be consumed
func (i *GuardianInvitation) IsValid() bool {
	return !i.IsExpired() && !i.IsAccepted()
}

// MarkAsAccepted sets the AcceptedAt timestamp to now
func (i *GuardianInvitation) MarkAsAccepted() {
	now := time.Now()
	i.AcceptedAt = &now
}

// SetExpiry assigns a duration from now as the expiry
func (i *GuardianInvitation) SetExpiry(duration time.Duration) {
	i.ExpiresAt = time.Now().Add(duration)
}

// GetID returns the primary key
func (i *GuardianInvitation) GetID() interface{} {
	return i.ID
}

// GetCreatedAt returns the creation timestamp
func (i *GuardianInvitation) GetCreatedAt() time.Time {
	return i.CreatedAt
}

// GetUpdatedAt returns the last update timestamp
func (i *GuardianInvitation) GetUpdatedAt() time.Time {
	return i.UpdatedAt
}
