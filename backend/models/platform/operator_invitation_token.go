package platform

import (
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
