package compose_test

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
	workforceCompose "github.com/moto-nrw/project-phoenix/modules/workforce/compose"
	"github.com/moto-nrw/project-phoenix/realtime"
	"github.com/moto-nrw/project-phoenix/services"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/moto-nrw/project-phoenix/workflows/shiftplansync"
	"github.com/moto-nrw/project-phoenix/workflows/shiftplansync/compose"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// planSyncEnv composes the workflow against the real owners in a real
// database: Workforce serves the Dienstplan rows, the shift write that
// rebuilds a cancelled shift's covers and the per-staff write lock, while the
// retained timetable planning services serve the Betreuungsplan staffing.
type planSyncEnv struct {
	db        *bun.DB
	repos     repositories.TimetableTestRepositories
	timetable services.TimetableTestModule
	shifts    workforce.Capability
	planning  workforce.StaffShiftPlanning
	lock      func(context.Context, int64) error
	// cascade and substitution are the workflow's two halves, bound the way
	// the composition root binds them.
	cascade      shiftplansync.SickCascade
	substitution shiftplansync.ScheduleSubstitution
}

// newPlanSyncEnv builds the workflow. cascadeBroadcaster replaces the
// timetable module's hub as the workflow's SSE sink, so a recording
// broadcaster sees exactly the workflow's own events and not the shift
// writes'. now fixes the clock the past-day guards compare against.
func newPlanSyncEnv(t *testing.T, cascadeBroadcaster realtime.Broadcaster, now func() time.Time) planSyncEnv {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	logger := slog.Default()
	var clocks []func() time.Time
	if now != nil {
		clocks = append(clocks, now)
	}
	today := timezone.CalendarDateClock(clocks...)

	repos, err := repositories.NewTimetableTestRepositories(db, clocks...)
	require.NoError(t, err)
	timetable, err := services.NewTimetableTestModule(db, testpkg.TenantRuntime(t, db), clocks...)
	require.NoError(t, err)
	membership, err := repositories.NewSchoolMembership(db)
	require.NoError(t, err)
	capability, err := repositories.NewWorkforce(db, membership)
	require.NoError(t, err)

	shiftPlanning, err := workforceCompose.NewShiftPlanning(workforceCompose.ShiftPlanningDependencies{
		Workforce:       capability,
		Staff:           repos.Staff,
		CalendarPeriods: repos.CalendarPeriod,
		DeviationEvents: repos.DeviationEvent,
		Instances:       repos.ActivityInstance,
		InstanceStaff:   repos.InstanceStaff,
		Rooms:           repos.Room,
		ActivityGroups:  repos.ActivityGroup,
		DB:              db,
		Broadcaster:     timetable.RealtimeHub,
		Logger:          logger,
		Today:           today,
	})
	require.NoError(t, err)
	// planExport is nil: no suite here renders the printable plan.
	planning := shiftPlanning.Planning(nil)

	broadcaster := cascadeBroadcaster
	if broadcaster == nil {
		broadcaster = timetable.RealtimeHub
	}
	lock := workforceCompose.NewStaffShiftLock(db)
	cascade, err := compose.NewSickCascade(compose.SickCascadeDependencies{
		Planning:        planning,
		Workforce:       capability,
		LockStaffShifts: lock,
		Instances:       timetable.Instance,
		TimetableData:   services.NewSickCascadeTimetableRows(repos.OwnerRows(), db),
		InstanceStaff:   repos.InstanceStaff,
		Broadcaster:     broadcaster,
		Logger:          logger,
		Today:           today,
	})
	require.NoError(t, err)
	substitution, err := compose.NewSubstitution(compose.SubstitutionDependencies{
		Instances:         timetable.Instance,
		ActivityInstances: repos.ActivityInstance,
		InstanceStaff:     repos.InstanceStaff,
		Staff:             repos.Staff,
		Broadcaster:       broadcaster,
		Logger:            logger,
	})
	require.NoError(t, err)

	return planSyncEnv{
		db:           db,
		repos:        repos,
		timetable:    timetable,
		shifts:       capability,
		planning:     planning,
		lock:         lock,
		cascade:      cascade,
		substitution: substitution,
	}
}
