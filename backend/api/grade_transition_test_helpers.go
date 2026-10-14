package api

import (
	"context"
	"log/slog"
	"time"

	auditCompose "github.com/moto-nrw/project-phoenix/modules/auditlog/compose"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	"github.com/moto-nrw/project-phoenix/services"
	timezone "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/workflows/gradetransition"
	gradetransitioncompose "github.com/moto-nrw/project-phoenix/workflows/gradetransition/compose"
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

func NewGradeTransitionTestModule(db transactionruntime.Database, resync OfferingResync, clocks ...func() time.Time) (GradeTransitionTestModule, error) {
	command, err := auditCompose.NewCommand(NewTestAuditStore(db), func(auditCompose.AppendObservation) {})
	if err != nil {
		return GradeTransitionTestModule{}, err
	}
	tt, err := NewTimetableTestRepositories(db, clocks...)
	if err != nil {
		return GradeTransitionTestModule{}, err
	}
	people, err := NewPeopleDirectory(db)
	if err != nil {
		return GradeTransitionTestModule{}, err
	}
	membership, err := services.NewSchoolMembership(db.Pool())
	if err != nil {
		return GradeTransitionTestModule{}, err
	}
	if resync == nil {
		resync = func(context.Context, timezone.Date) error { return nil }
	}
	clock := services.OptionalClock(clocks)
	recurrenceLock, err := newTimetableTestRecurrenceLock(db)
	if err != nil {
		return GradeTransitionTestModule{}, err
	}
	workflow, err := gradetransitioncompose.New(gradetransitioncompose.Dependencies{
		DB: db.Pool(), Directory: people, Membership: membership,
		Rosters:               tt.RosterMaintenance(slog.Default(), clock),
		LockRecurrenceWrites:  recurrenceLock.LockRecurrenceWrites,
		ResyncOfferingRosters: resync,
		Audit:                 command, Clock: clock,
	})
	if err != nil {
		return GradeTransitionTestModule{}, err
	}
	return GradeTransitionTestModule{GradeTransition: workflow}, nil
}
