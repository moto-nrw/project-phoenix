package owners

import (
	"context"
	"fmt"

	realtime "github.com/moto-nrw/project-phoenix/realtime"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	peoplePorts "github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	"github.com/moto-nrw/project-phoenix/workflows/studentdeletion"
	studentdeletioncompose "github.com/moto-nrw/project-phoenix/workflows/studentdeletion/compose"
)

// buildStudentDeletion binds the permanent child deletion (#2710), which
// unlinks a deleted child's photo through the photo lifecycle.
func BuildStudentDeletion(
	db transactionruntime.Database, env RootEnvironment, hub *realtime.Hub, persons peopledirectory.Capability,
	carePlan careplan.Capability, timetableOwner timetable.Capability, owner TimetableOwner, feedback studentdeletioncompose.Feedback,
	verifiedStaff func(context.Context) (bool, error), photos peoplePorts.StudentPhotoService, audit AuditOwner,
) (*studentdeletion.Workflow, error) {
	owners, err := NewStudentDeletionOwners(db)
	if err != nil {
		return nil, fmt.Errorf("compose student deletion owners: %w", err)
	}
	workflow, err := studentdeletioncompose.New(studentdeletioncompose.Dependencies{
		DB: db.Pool(), Owners: owners, Directory: persons, CarePlan: carePlan, Timetable: timetableOwner,
		Feedback: feedback, IsVerifiedStaff: verifiedStaff,
		LockCareBookingWrites: owner.RecurrenceLock.LockRecurrenceWrites,
		UnlinkPhoto:           photos.ScheduleUnlinkAfterCommit,
		Broadcaster:           hub, Logger: env.Logger.With("workflow", "student_deletion"), Audit: audit.Command,
	}, NewStudentActivityEnrollments())
	if err != nil {
		return nil, fmt.Errorf("compose student deletion workflow: %w", err)
	}
	return workflow, nil
}
