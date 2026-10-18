package api

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	peoplePorts "github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	timezone "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/workflows/gradetransition"
	gradetransitioncompose "github.com/moto-nrw/project-phoenix/workflows/gradetransition/compose"
	"github.com/moto-nrw/project-phoenix/workflows/studentdeletion"
	studentdeletioncompose "github.com/moto-nrw/project-phoenix/workflows/studentdeletion/compose"
)

// buildGradeTransition binds the school-year rollover workflow (#2711),
// which re-reconciles the offering-sourced templates' Jahrgang-filtered
// rosters (#2137) through Care Plan's booking materialization.
func buildGradeTransition(
	db transactionruntime.Database, env RootEnvironment, persons peopledirectory.Capability, membership schoolmembership.Capability,
	owner TimetableOwner, resyncOfferingRosters func(context.Context, timezone.Date) error, audit AuditOwner,
) (*gradetransition.Workflow, error) {
	structure, presence, err := NewGradeTransitionOwners(db)
	if err != nil {
		return nil, fmt.Errorf("compose grade transition owners: %w", err)
	}
	workflow, err := gradetransitioncompose.New(gradetransitioncompose.Dependencies{
		DB: db.Pool(), Structure: structure, Presence: presence, Directory: persons, Membership: membership, Rosters: owner.RosterReconciler,
		LockRecurrenceWrites:  owner.RecurrenceLock.LockRecurrenceWrites,
		ResyncOfferingRosters: resyncOfferingRosters,
		Logger:                env.Logger.With("workflow", "grade_transition"), Audit: audit.Command, ClassListEntryRecord: GradeTransitionClassListRecord,
		Clock: env.Now,
	})
	if err != nil {
		return nil, fmt.Errorf("compose grade transition workflow: %w", err)
	}
	return workflow, nil
}

// buildStudentDeletion binds the permanent child deletion (#2710), which
// unlinks a deleted child's photo through the photo lifecycle.
func buildStudentDeletion(
	db transactionruntime.Database, env RootEnvironment, transport MailTransport, persons peopledirectory.Capability,
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
		Broadcaster:           transport.Hub, Logger: env.Logger.With("workflow", "student_deletion"), Audit: audit.Command,
	}, NewStudentActivityEnrollments())
	if err != nil {
		return nil, fmt.Errorf("compose student deletion workflow: %w", err)
	}
	return workflow, nil
}
