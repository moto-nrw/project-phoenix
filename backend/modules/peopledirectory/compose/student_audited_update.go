package compose

import (
	"context"

	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/application"
)

// AuditedStudentUpdates is the audited write of a retained child row the
// root's Care Plan adapters reach: the row, then its change history, in the
// caller's transaction (see application.AuditedStudentUpdates).
type AuditedStudentUpdates interface {
	UpdateStudentAudited(ctx context.Context, before, after *userModels.Student, actorAccountID int64) (bool, error)
	WidenAccompaniedDays(ctx context.Context, companion *userModels.Student, days []string, actorAccountID int64) error
}

// NewAuditedStudentUpdates binds the command to the retained row write and
// the change recorder; audit may be nil.
func NewAuditedStudentUpdates(students application.StudentRowUpdate, audit application.StudentChangeActorRecorder) AuditedStudentUpdates {
	return application.NewAuditedStudentUpdates(students, audit)
}
