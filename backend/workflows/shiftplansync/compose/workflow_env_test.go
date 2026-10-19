package compose_test

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/api"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
	workforceCompose "github.com/moto-nrw/project-phoenix/modules/workforce/compose"
	"github.com/moto-nrw/project-phoenix/realtime"
	owners "github.com/moto-nrw/project-phoenix/roots/owners"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
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
	db    *bun.DB
	repos owners.TimetableTestRepositories
	// deviations is the Änderungsprotokoll the suites read back.
	deviations api.TimetableDeviationAudit
	timetable  api.TimetableTestModule
	shifts     workforce.Capability
	Planning   workforce.StaffShiftPlanning
	Lock       func(context.Context, int64) error
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
	today := calendar.CalendarDateClock(clocks...)

	repos, err := api.NewTimetableTestRepositories(testpkg.NewDatabase(t, db), clocks...)
	require.NoError(t, err)
	members := owners.MustNewMembershipTestRepositories(testpkg.NewDatabase(t, db))
	rooms, err := owners.NewTimetableRoomRows(testpkg.NewDatabase(t, db))
	require.NoError(t, err)
	schoolCalendar, err := api.NewTimetableSchoolCalendar(testpkg.NewDatabase(t, db))
	require.NoError(t, err)
	deviations := api.NewTimetableDeviationAudit(testpkg.NewDatabase(t, db))
	timetable, _, realtimeHub, err := api.NewTimetableTestModule(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db), clocks...)
	require.NoError(t, err)
	membership, err := owners.NewSchoolMembership(testpkg.NewDatabase(t, db))
	require.NoError(t, err)
	capability, err := owners.NewWorkforce(testpkg.NewDatabase(t, db), membership)
	require.NoError(t, err)

	shiftPlanning, err := workforceCompose.NewShiftPlanning(workforceCompose.ShiftPlanningDependencies{
		Workforce:       capability,
		Staff:           owners.NewPlanStaffDirectory(members.Staff),
		CalendarPeriods: schoolCalendar.SchoolCalendar(),
		ShiftMoveAudit:  owners.NewShiftMoveAudit(deviations.DeviationEvent),
		Instances:       repos.InstanceReads(),
		InstanceStaff:   repos.InstanceStaffReads(),
		Rooms:           owners.NewTimetableRoomNames(rooms.Facilities),
		ActivityGroups:  repos.GroupReads(),
		DB:              db,
		Broadcaster:     realtimeHub,
		Logger:          logger,
		Today:           today,
	})
	require.NoError(t, err)
	// planExport is nil: no suite here renders the printable plan.
	planning := shiftPlanning.Planning(nil)

	broadcaster := cascadeBroadcaster
	if broadcaster == nil {
		broadcaster = realtimeHub
	}
	lock := workforceCompose.NewStaffShiftLock(db)
	cascade, err := compose.NewSickCascade(compose.SickCascadeDependencies{
		Planning:        planning,
		Workforce:       capability,
		LockStaffShifts: lock,
		Deviations:      timetable.TimetableData.Deviations,
		TimetableData:   api.NewSickCascadeTimetableRows(repos.OwnerRows(), testpkg.NewDatabase(t, db)),
		InstanceStaff:   repos.InstanceStaffReads(),
		Broadcaster:     broadcaster,
		Logger:          logger,
		Today:           today,
	})
	require.NoError(t, err)
	substitution, err := compose.NewSubstitution(compose.SubstitutionDependencies{
		Deviations:        timetable.TimetableData.Deviations,
		ActivityInstances: repos.InstanceReads(),
		InstanceStaff:     repos.InstanceStaffReads(),
		Staff:             owners.NewPlanStaffDirectory(members.Staff),
		Broadcaster:       broadcaster,
		Logger:            logger,
	})
	require.NoError(t, err)

	return planSyncEnv{
		db:           db,
		repos:        repos,
		deviations:   deviations,
		timetable:    timetable,
		shifts:       capability,
		Planning:     planning,
		Lock:         lock,
		cascade:      cascade,
		substitution: substitution,
	}
}
