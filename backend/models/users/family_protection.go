package users

import (
	"context"

	"github.com/moto-nrw/project-phoenix/models/base"
)

// FamilyProtectionEvent is one immutable change to a child's privacy rule.
// The newest event is the current state; older events remain the audit trail.
type FamilyProtectionEvent struct {
	base.Model `bun:"schema:users,table:student_family_protection_events"`
	base.TenantModel
	StudentID      int64  `bun:"student_id,notnull" json:"student_id"`
	Enabled        bool   `bun:"enabled,notnull" json:"enabled"`
	Reason         string `bun:"reason,notnull" json:"reason"`
	ActorAccountID int64  `bun:"actor_account_id,notnull" json:"actor_account_id"`
}

type FamilyProtectionEventRepository interface {
	Create(ctx context.Context, event *FamilyProtectionEvent) error
	CurrentForStudents(ctx context.Context, studentIDs []int64) (map[int64]*FamilyProtectionEvent, error)
}
