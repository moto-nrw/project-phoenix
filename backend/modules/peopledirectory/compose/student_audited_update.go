package compose

import (
	"context"
	"time"

	peopleModule "github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/application"
)

// AuditedStudentUpdates are the retained child row writes the root's Care Plan
// adapters reach, in the caller's transaction: the audited write, the locked
// companion widening and the live absence flags (application.AuditedStudentUpdates).
type AuditedStudentUpdates interface {
	UpdateStudentAudited(ctx context.Context, before, after *peopleModule.StudentState, actorAccountID int64) (bool, error)
	WidenAccompaniedDaysByID(ctx context.Context, studentID int64, days []string, actorAccountID int64) (bool, error)
	SetLiveAbsenceFlags(ctx context.Context, studentID int64, flags peopleModule.LiveAbsenceFlags, at time.Time) error
}

// NewAuditedStudentUpdates binds the commands to the retained row lock and
// write and the change recorder; audit may be nil.
func NewAuditedStudentUpdates(students application.StudentRowLockUpdate, audit application.StudentChangeActorRecorder) AuditedStudentUpdates {
	return application.NewLockedStudentUpdates(students, audit)
}
