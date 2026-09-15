package education_test

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	auditSvc "github.com/moto-nrw/project-phoenix/services/audit"
	scheduleSvc "github.com/moto-nrw/project-phoenix/services/schedule"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/moto-nrw/project-phoenix/workflows/gradetransition"
	gradetransitioncompose "github.com/moto-nrw/project-phoenix/workflows/gradetransition/compose"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// transitionFixture assembles the grade transition workflow (#2711) over the
// real owner compositions the production root uses: People Directory,
// School Membership, School Structure, Student Presence, the Timetable
// roster reconciliation and the audit command. Only the HTTP principal is
// replaced by a fixed actor; every port stays swappable through deps so a
// test can wrap one owner command or record the resync.
type transitionFixture struct {
	db         *bun.DB
	people     peopledirectory.Capability
	membership schoolmembership.Capability
	timetable  repositories.TimetableTestRepositories
	deps       gradetransition.Dependencies
	actorID    int64
	now        time.Time
	// resyncCalls records every offering-roster resync the workflow triggers.
	resyncCalls []timezone.Date
}

// newTransitionFixture composes the workflow with the clock pinned to a fixed
// Berlin instant; f.setNow moves it.
func newTransitionFixture(t *testing.T, db *bun.DB) *transitionFixture {
	t.Helper()
	f := &transitionFixture{db: db, now: time.Date(2026, 8, 24, 12, 0, 0, 0, timezone.Berlin)}
	clock := func() time.Time { return f.now }
	timetable, err := repositories.NewTimetableTestRepositories(db, clock)
	require.NoError(t, err)
	f.timetable = timetable
	f.people, err = repositories.NewPeopleDirectory(db)
	require.NoError(t, err)
	f.membership, err = repositories.NewSchoolMembership(db)
	require.NoError(t, err)
	command, err := auditSvc.NewCommand(repositories.NewTestAuditStore(db), func(auditSvc.AppendObservation) {})
	require.NoError(t, err)
	actor := testpkg.CreateTestAccount(t, db, "grade-transition-actor@example.com")
	f.actorID = actor.ID
	deps, err := gradetransitioncompose.Assemble(gradetransitioncompose.Dependencies{
		DB: db, Directory: f.people, Membership: f.membership,
		Rosters:              scheduleSvc.NewRosterReconciler(timetable.ActivityInstance, timetable.InstanceStudent, timetable.StudentEnrollment, slog.Default(), clock),
		LockRecurrenceWrites: func(ctx context.Context) error { return scheduleSvc.LockTenantRecurrenceWrites(ctx, db) },
		ResyncOfferingRosters: func(_ context.Context, effectiveFrom timezone.Date) error {
			f.resyncCalls = append(f.resyncCalls, effectiveFrom)
			return nil
		},
		Audit: command, Clock: clock,
	})
	require.NoError(t, err)
	deps.Authorize = func(ctx context.Context, _ string) (gradetransition.Actor, error) {
		return gradetransition.Actor{TenantID: tenant.FromContext(ctx), AccountID: f.actorID}, nil
	}
	f.deps = deps
	return f
}

// setNow moves the fixture clock: Today, Now and the roster reconciliation
// all read it.
func (f *transitionFixture) setNow(now time.Time) { f.now = now }

// today is the calendar day the workflow sees.
func (f *transitionFixture) today() timezone.Date { return timezone.DateFromTime(f.now) }

// workflow constructs the workflow over the current deps.
func (f *transitionFixture) workflow(t *testing.T) *gradetransition.Workflow {
	t.Helper()
	workflow, err := gradetransition.New(f.deps)
	require.NoError(t, err)
	return workflow
}

// createDraft stores a draft with the given mappings through the workflow.
func (f *transitionFixture) createDraft(t *testing.T, ctx context.Context, academicYear string, mappings ...gradetransition.Mapping) int64 {
	t.Helper()
	transition, err := f.workflow(t).Create(ctx, gradetransition.Draft{AcademicYear: academicYear, Mappings: mappings})
	require.NoError(t, err)
	return transition.ID
}

// promote and graduate build mapping inputs.
func promote(from, to string) gradetransition.Mapping {
	return gradetransition.Mapping{FromClass: from, ToClass: &to}
}

func graduate(from string) gradetransition.Mapping {
	return gradetransition.Mapping{FromClass: from}
}
