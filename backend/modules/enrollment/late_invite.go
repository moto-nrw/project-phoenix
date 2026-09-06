package enrollment

import (
	"errors"
	"time"
)

// ErrLateInviteNotFound distinguishes an unusable token from a failed lookup.
var ErrLateInviteNotFound = errors.New("late invite not found")

// LateInvite grants one family an exception to a phase's public intake window.
type LateInvite struct {
	ID                int64      `json:"id"`
	TenantID          int64      `json:"tenant_id"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
	PhaseID           int64      `json:"phase_id"`
	TokenHash         string     `json:"token_hash"`
	GuardianEmail     string     `json:"guardian_email"`
	GuardianFirstName *string    `json:"guardian_first_name,omitempty"`
	GuardianLastName  *string    `json:"guardian_last_name,omitempty"`
	ExpiresAt         time.Time  `json:"expires_at"`
	UsedAt            *time.Time `json:"used_at,omitempty"`
	UsedRequestID     *int64     `json:"used_request_id,omitempty"`
	CreatedBy         int64      `json:"created_by"`
	Reason            *string    `json:"reason,omitempty"`
}
