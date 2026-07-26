package auth

import (
	"errors"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
)

// InvitationToken represents an invitation sent to create a new account.
type InvitationToken struct {
	base.Model `bun:"schema:auth,table:invitation_tokens"`
	base.TenantModel

	Email            string     `bun:"email,notnull" json:"email"`
	Token            string     `bun:"token,notnull" json:"token"`
	RoleID           int64      `bun:"role_id,notnull" json:"role_id"`
	CreatedBy        *int64     `bun:"created_by,nullzero" json:"created_by,omitempty"`
	ExpiresAt        time.Time  `bun:"expires_at,notnull" json:"expires_at"`
	UsedAt           *time.Time `bun:"used_at,nullzero" json:"used_at,omitempty"`
	FirstName        *string    `bun:"first_name,nullzero" json:"first_name,omitempty"`
	LastName         *string    `bun:"last_name,nullzero" json:"last_name,omitempty"`
	Position         *string    `bun:"position,nullzero" json:"position,omitempty"`
	CaregiverEnabled bool       `bun:"caregiver_enabled,notnull,default:false" json:"caregiver_enabled"`
	EmailSentAt      *time.Time `bun:"email_sent_at,nullzero" json:"email_sent_at,omitempty"`
	EmailError       *string    `bun:"email_error,nullzero" json:"email_error,omitempty"`
	EmailRetryCount  int        `bun:"email_retry_count,notnull,default:0" json:"email_retry_count"`

	// Relations
	Role    *Role    `bun:"rel:belongs-to,join:role_id=id" json:"role,omitempty"`
	Creator *Account `bun:"rel:belongs-to,join:created_by=id" json:"creator,omitempty"`
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
	if t.CreatedBy != nil && *t.CreatedBy <= 0 {
		return errors.New("created_by must be positive when set")
	}
	if t.ExpiresAt.IsZero() {
		return errors.New("expires_at is required")
	}
	return nil
}

// IsUsed returns true if the invitation was already accepted or revoked. This
// is a pure field accessor (UsedAt != nil); the wall-clock expiry decision
// lives in the auth service (services/auth.InvitationTokenExpired), per issue
// #586 (Rule 12).
func (t *InvitationToken) IsUsed() bool {
	return t.UsedAt != nil
}

// SetExpiry assigns a duration from now as the expiry.
func (t *InvitationToken) SetExpiry(duration time.Duration) {
	t.ExpiresAt = time.Now().Add(duration)
}
