package audit

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/users"
)

// StudentDeletion is an append-only, data-minimal tombstone for a permanently
// deleted child. It keeps the internal student ID for accountability, but no
// name, birthday, contact information or snapshot of the erased data.
type StudentDeletion struct {
	ID int64 `bun:"id,pk,autoincrement" json:"id"`
	base.TenantModel
	StudentID      int64                       `bun:"student_id,notnull" json:"student_id"`
	ActorAccountID int64                       `bun:"actor_account_id,notnull" json:"actor_account_id"`
	Reason         string                      `bun:"reason,notnull" json:"reason"`
	Counts         users.StudentDeletionCounts `bun:"counts,type:jsonb,notnull" json:"counts"`
	DeletedAt      time.Time                   `bun:"deleted_at,notnull,default:now()" json:"deleted_at"`
}

type StudentDeletionRepository interface {
	Create(ctx context.Context, event *StudentDeletion) error
}
