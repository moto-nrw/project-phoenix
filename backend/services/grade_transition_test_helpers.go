package services

import (
	"context"
	"log/slog"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	auditSvc "github.com/moto-nrw/project-phoenix/services/audit"
	"github.com/moto-nrw/project-phoenix/services/schedule"
	"github.com/moto-nrw/project-phoenix/workflows/gradetransition"
	gradetransitioncompose "github.com/moto-nrw/project-phoenix/workflows/gradetransition/compose"
	"github.com/uptrace/bun"
)

// GradeTransitionTestModule composes the grade transition workflow (#2711)
// the way the production root does, over the real People Directory, School
// Membership, School Structure, Student Presence and the Timetable roster
// reconciliation.
type GradeTransitionTestModule struct {
	GradeTransition *gradetransition.Workflow
}

// OfferingResync is the offering-roster resync the workflow runs after the
// class rewrite. A nil resync is a no-op: the HTTP tests exercise the
// transition contract, and the resync has its own hermetic coverage with the
// real enrollment decision service.
type OfferingResync func(ctx context.Context, effectiveFrom timezone.Date) error

func NewGradeTransitionTestModule(db *bun.DB, resync OfferingResync, clocks ...func() time.Time) (GradeTransitionTestModule, error) {
	command, err := auditSvc.NewCommand(repositories.NewTestAuditStore(db), func(auditSvc.AppendObservation) {})
	if err != nil {
		return GradeTransitionTestModule{}, err
	}
	tt, err := repositories.NewTimetableTestRepositories(db, clocks...)
	if err != nil {
		return GradeTransitionTestModule{}, err
	}
	people, err := repositories.NewPeopleDirectory(db)
	if err != nil {
		return GradeTransitionTestModule{}, err
	}
	membership, err := repositories.NewSchoolMembership(db)
	if err != nil {
		return GradeTransitionTestModule{}, err
	}
	if resync == nil {
		resync = func(context.Context, timezone.Date) error { return nil }
	}
	clock := optionalClock(clocks)
	workflow, err := gradetransitioncompose.New(gradetransitioncompose.Dependencies{
		DB: db, Directory: people, Membership: membership,
		Rosters:               schedule.NewRosterReconciler(tt.ActivityInstance, tt.InstanceStudent, tt.StudentEnrollment, slog.Default(), clock),
		LockRecurrenceWrites:  func(ctx context.Context) error { return schedule.LockTenantRecurrenceWrites(ctx, db) },
		ResyncOfferingRosters: resync,
		Audit:                 command, Clock: clock,
	})
	if err != nil {
		return GradeTransitionTestModule{}, err
	}
	return GradeTransitionTestModule{GradeTransition: workflow}, nil
}
