package audit

import (
	"context"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/base"
)

const (
	SubstitutionAssigned = "assigned"
	SubstitutionEnded    = "ended"
)

// SubstitutionChange is the append-only trail for responsibility changes.
// It intentionally stores identifiers only; names and staff records remain
// outside the audit boundary.
type SubstitutionChange struct {
	base.Model `bun:"schema:audit,table:substitution_changes"`
	base.TenantModel
	SubstitutionID int64         `bun:"substitution_id,notnull"`
	TargetType     string        `bun:"target_type,notnull"`
	Action         string        `bun:"action,notnull"`
	GroupID        int64         `bun:"group_id,notnull"`
	TargetStaffID  int64         `bun:"target_staff_id,notnull"`
	ActorAccountID int64         `bun:"actor_account_id,notnull"`
	StartDate      timezone.Date `bun:"start_date,notnull"`
	EndDate        timezone.Date `bun:"end_date,notnull"`
}

type SubstitutionChangeCreator interface {
	Create(ctx context.Context, change *SubstitutionChange) error
}
