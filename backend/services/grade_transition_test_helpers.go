package services

import (
	"context"
	"log/slog"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	auditSvc "github.com/moto-nrw/project-phoenix/services/audit"
	"github.com/moto-nrw/project-phoenix/services/education"
	"github.com/moto-nrw/project-phoenix/services/schedule"
	"github.com/moto-nrw/project-phoenix/workflows/gradetransition"
	gradetransitioncompose "github.com/moto-nrw/project-phoenix/workflows/gradetransition/compose"
	"github.com/uptrace/bun"
)

// GradeTransitionTestModule composes the grade transition workflow (#2711)
// the way the production root does, over the real People Directory, School
// Membership, School Structure, Student Presence and the Timetable roster
// reconciliation. The offering-roster resync is a no-op until
// BindOfferingResync installs the enrollment decision service, which the
// student test module constructs after this one.
type GradeTransitionTestModule struct {
	GradeTransition *gradetransition.Workflow
	resync          *offeringResyncSlot
}

type offeringResyncSlot struct {
	fn func(context.Context, timezone.Date) error
}

// BindOfferingResync routes the workflow's offering-roster resync through
// the given enrollment decision service.
func (m GradeTransitionTestModule) BindOfferingResync(resyncer education.OfferingSourceResyncer) {
	m.resync.fn = resyncer.ResyncOfferingSourcedTemplates
}

func NewGradeTransitionTestModule(db *bun.DB, clocks ...func() time.Time) (GradeTransitionTestModule, error) {
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
	clock := optionalClock(clocks)
	slot := &offeringResyncSlot{}
	workflow, err := gradetransitioncompose.New(gradetransitioncompose.Dependencies{
		DB: db, Directory: people, Membership: membership,
		Rosters:              schedule.NewRosterReconciler(tt.ActivityInstance, tt.InstanceStudent, tt.StudentEnrollment, slog.Default(), clock),
		LockRecurrenceWrites: func(ctx context.Context) error { return schedule.LockTenantRecurrenceWrites(ctx, db) },
		ResyncOfferingRosters: func(ctx context.Context, effectiveFrom timezone.Date) error {
			if slot.fn == nil {
				return nil
			}
			return slot.fn(ctx, effectiveFrom)
		},
		Audit: command, Clock: clock,
	})
	if err != nil {
		return GradeTransitionTestModule{}, err
	}
	return GradeTransitionTestModule{GradeTransition: workflow, resync: slot}, nil
}
