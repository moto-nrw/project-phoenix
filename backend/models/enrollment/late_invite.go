package enrollment

import (
	"context"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
)

// ErrLateInviteNotFound reports that no usable late invite matches the token
// — unknown, already used, expired, or minted for a different phase. It is the
// repository's contract for "this token grants nothing", and callers MUST
// distinguish it from every other error the lookup can return: a database or
// driver failure means the invite status is unknown, not that the invite is
// invalid, and must surface as a server error instead of silently downgrading
// a valid recipient to the anonymous public gate (#1663).
var ErrLateInviteNotFound = errors.New("late invite not found")

// LateInvite is a one-family exception token that lets parents submit a
// request for a closed phase without reopening the phase globally.
type LateInvite struct {
	base.Model `bun:"schema:enrollment,table:late_invites"`
	base.TenantModel
	PhaseID           int64      `bun:"phase_id,notnull" json:"phase_id"`
	TokenHash         string     `bun:"token_hash,notnull" json:"token_hash"`
	GuardianEmail     string     `bun:"guardian_email,notnull" json:"guardian_email"`
	GuardianFirstName *string    `bun:"guardian_first_name" json:"guardian_first_name,omitempty"`
	GuardianLastName  *string    `bun:"guardian_last_name" json:"guardian_last_name,omitempty"`
	ExpiresAt         time.Time  `bun:"expires_at,notnull" json:"expires_at"`
	UsedAt            *time.Time `bun:"used_at" json:"used_at,omitempty"`
	UsedRequestID     *int64     `bun:"used_request_id" json:"used_request_id,omitempty"`
	CreatedBy         int64      `bun:"created_by,notnull" json:"created_by"`
	Reason            *string    `bun:"reason" json:"reason,omitempty"`
}

type LateInviteRepository interface {
	Create(ctx context.Context, invite *LateInvite) error
	FindUsableByTokenHash(ctx context.Context, tokenHash string, phaseID int64, now time.Time) (*LateInvite, error)
	FindUsableByTokenHashForUpdate(ctx context.Context, tokenHash string, phaseID int64, now time.Time) (*LateInvite, error)
	MarkUsed(ctx context.Context, inviteID, requestID int64, usedAt time.Time) error
	DeleteByUsedRequestID(ctx context.Context, requestID int64) (int64, error)
}
