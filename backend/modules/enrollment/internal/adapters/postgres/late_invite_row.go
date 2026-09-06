package postgres

import (
	"time"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/uptrace/bun"
)

type lateInviteRow struct {
	bun.BaseModel     `bun:"table:enrollment.late_invites,alias:late_invite"`
	ID                int64      `bun:"id,pk,autoincrement"`
	TenantID          int64      `bun:"tenant_id"`
	CreatedAt         time.Time  `bun:"created_at"`
	UpdatedAt         time.Time  `bun:"updated_at"`
	PhaseID           int64      `bun:"phase_id"`
	TokenHash         string     `bun:"token_hash"`
	GuardianEmail     string     `bun:"guardian_email"`
	GuardianFirstName *string    `bun:"guardian_first_name"`
	GuardianLastName  *string    `bun:"guardian_last_name"`
	ExpiresAt         time.Time  `bun:"expires_at"`
	UsedAt            *time.Time `bun:"used_at"`
	UsedRequestID     *int64     `bun:"used_request_id"`
	CreatedBy         int64      `bun:"created_by"`
	Reason            *string    `bun:"reason"`
}

func (r lateInviteRow) value() *enrollment.LateInvite {
	return &enrollment.LateInvite{ID: r.ID, TenantID: r.TenantID, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt, PhaseID: r.PhaseID, TokenHash: r.TokenHash, GuardianEmail: r.GuardianEmail, GuardianFirstName: r.GuardianFirstName, GuardianLastName: r.GuardianLastName, ExpiresAt: r.ExpiresAt, UsedAt: r.UsedAt, UsedRequestID: r.UsedRequestID, CreatedBy: r.CreatedBy, Reason: r.Reason}
}
func lateInviteStorage(r *enrollment.LateInvite) *lateInviteRow {
	return &lateInviteRow{ID: r.ID, TenantID: r.TenantID, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt, PhaseID: r.PhaseID, TokenHash: r.TokenHash, GuardianEmail: r.GuardianEmail, GuardianFirstName: r.GuardianFirstName, GuardianLastName: r.GuardianLastName, ExpiresAt: r.ExpiresAt, UsedAt: r.UsedAt, UsedRequestID: r.UsedRequestID, CreatedBy: r.CreatedBy, Reason: r.Reason}
}
