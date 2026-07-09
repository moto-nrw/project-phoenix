package platform

import (
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
)

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
	if t.UsedAt != nil {
		return errors.New("token has already been used")
	}
	return nil
}

// IsUsed checks if the token has been used. This is a pure field accessor
// (UsedAt != nil); the wall-clock expiry/validity decision lives in the service
// layer (services/platform.OperatorInvitationTokenExpired /
// OperatorInvitationTokenValid) and the repository's valid-token finders, per
// issue #586 (Rule 12).
func (t *OperatorInvitationToken) IsUsed() bool {
	return t.UsedAt != nil
}
