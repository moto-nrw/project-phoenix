package httpintegration_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/api"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/securityruntime"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/modules/timetable/compose"
	fixtures "github.com/moto-nrw/project-phoenix/modules/timetable/fixturetest"
	timetablePorts "github.com/moto-nrw/project-phoenix/modules/timetable/ports"
	"github.com/moto-nrw/project-phoenix/realtime"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// The instance lifecycle suites (#3424 slice S1) compose the Timetable
// owner's lifecycle over the real repositories, the way the composition root
// does, so each test can swap one collaborator: a recording broadcaster, a
// fault-injecting recovery or presence, the clock policy or the guardian
// notice publisher.

// newInstancePresence builds the Student Presence owner the instance
// lifecycle drives.
func newInstancePresence(t *testing.T, db *bun.DB) studentpresence.Capability {
	t.Helper()
	return api.NewStudentPresenceForTests(testpkg.NewDatabase(t, db))
}

// lifecycleSetup bundles the wired lifecycle + common fixtures. Rows belong
// to the test's own tenant clone; nothing is cleaned up explicitly.
type lifecycleSetup struct {
	presence        timetablePorts.LifecyclePresence
	svc             fixtures.InstanceLifecycle
	instance        timetable.InstanceLifecycleCapability
	timetableData   compose.Planning
	active          studentpresence.Presence
	materialization timetable.MaterializationCapability
	careDay         careplan.CareDayQuery
	autoStart       timetable.InstanceAutoStart
	careOffering    careplan.CareOfferingCapability
	statusDays      studentpresence.StatusDays
	repos           *api.PresenceSuiteRepositories
	reposPeople     *api.PeopleRepositorySuiteFactory
	reposSchedules  *api.CareScheduleSuiteRepositories
	reposCare       *api.RepositoryTestFactory
	// timetable are the retained Timetable adapters over the same owners.
	timetable api.TimetableRepositories
	db        *bun.DB
	ctx       context.Context
	roomID    int64
	staffID   int64
	student1  int64
	student2  int64
	tmplID    int64
	period    *testpkg.CalendarPeriod
}

// buildLifecycle prepares the minimum scaffold for instance-lifecycle tests:
// one tenant, one room, one staff, two students, one template (is_template).
func buildLifecycle(t *testing.T) *lifecycleSetup {
	t.Helper()
	db := testpkg.SetupTestDB(t)

	repoFactory := api.NewRepositoryTestFactory(testpkg.NewDatabase(t, db))
	repoFactoryPeople := api.NewPeopleRepositorySuiteFactory(testpkg.NewDatabase(t, db))
	repoFactorySchedules := api.NewCareScheduleSuiteRepositories(testpkg.NewDatabase(t, db))
	repoFactoryPresence := api.NewPresenceSuiteRepositories(testpkg.NewDatabase(t, db))
	ttLifecycle, ttPlanning, ttMaterialization, ttActive, ttSchedules, ttCareOfferings, ttStatusDays, err := api.NewTestTimetable(testpkg.NewDatabase(t, db), slog.Default())
	require.NoError(t, err)

	// Own tenant per caller, subtests included: every subtest builds its own
	// lifecycle and then asserts tenant-wide (loadLifecycleExceptions counts
	// every exception of the tenant), so sharing the parent's tenant would
	// make each subtest see its predecessors' rows (#2419).
	ctx := testpkg.OwnCtx(t)
	suffix := time.Now().UnixNano()

	room := testpkg.CreateTestRoom(t, db, fmt.Sprintf("LC-Room-%d", suffix))
	staff := testpkg.CreateTestStaff(t, db, "LC", fmt.Sprintf("Super-%d", suffix))
	student1 := testpkg.CreateTestStudent(t, db, "LC-Alice", fmt.Sprintf("One-%d", suffix), "3a")
	student2 := testpkg.CreateTestStudent(t, db, "LC-Bob", fmt.Sprintf("Two-%d", suffix), "3a")

	// A template-ish activity group. IsTemplate flag is immaterial for the
	// lifecycle path; we only need a non-nil FK target for the spontaneous=
	// false branch to fire.
	templateRow := testpkg.CreateTestActivityGroup(t, db, fmt.Sprintf("LC-Tmpl-%d", suffix))
	period := testpkg.CreateTestCalendarPeriod(t, db, fmt.Sprintf("LC-Period-%d", suffix),
		calendar.NewDate(2000, 1, 1), calendar.NewDate(2100, 1, 1))
	testpkg.SetCalendarPeriodActive(t, db, period, true)

	setup := &lifecycleSetup{
		presence:        newInstancePresence(t, db),
		instance:        fixtures.FromInstanceLifecycle(ttLifecycle.Instances),
		timetableData:   ttPlanning.Planning,
		active:          ttActive,
		materialization: ttMaterialization,
		careDay:         ttSchedules.CareDay,
		autoStart:       ttLifecycle.AutoStart,
		careOffering:    ttCareOfferings,
		statusDays:      ttStatusDays,
		repos:           repoFactoryPresence,
		reposPeople:     repoFactoryPeople,
		reposSchedules:  repoFactorySchedules,
		reposCare:       repoFactory,
		timetable:       api.NewUnobservedTimetableRepositories(testpkg.MustDatabase(db)),
		db:              db,
		ctx:             ctx,
		roomID:          room.ID,
		staffID:         staff.ID,
		student1:        student1.ID,
		student2:        student2.ID,
		tmplID:          templateRow.ID,
		period:          period,
	}
	// These state-machine fixtures use fixed wall-clock windows. Time-policy
	// boundaries have dedicated clock-injected tests; keep this suite focused on
	// lifecycle persistence and bridge behavior.
	setup.svc = instanceServiceWithBroadcaster(t, setup, nil)
	return setup
}

func instanceServiceWithBroadcaster(t *testing.T, s *lifecycleSetup, broadcaster realtime.Broadcaster, recovery ...fixtures.ActivityRecoveryRecords) fixtures.InstanceLifecycle {
	t.Helper()
	return newLifecycle(t, lifecycleDependencies(s, broadcaster, recovery...))
}

func newLifecycle(t *testing.T, deps timetablePorts.InstanceLifecycleDependencies) fixtures.InstanceLifecycle {
	t.Helper()
	svc, err := compose.NewInstanceLifecycle(deps)
	require.NoError(t, err)
	return fixtures.FromInstanceLifecycle(svc)
}

// newLifecycleAutoEnd composes the scheduler's auto end over the lifecycle,
// as the composition root does.
func newLifecycleAutoEnd(t *testing.T, instances fixtures.ActivityInstanceRecords, lifecycle timetable.InstanceLifecycle) timetable.InstanceAutoEnd {
	t.Helper()
	autoEnd, err := compose.NewInstanceAutoEnd(fixtures.IntoActivityInstanceRecords(instances), lifecycle)
	require.NoError(t, err)
	return autoEnd
}

// lifecycleDependencies is the dependency set instanceServiceWithBroadcaster
// wires, for tests that add an optional collaborator before construction.
func lifecycleDependencies(s *lifecycleSetup, broadcaster realtime.Broadcaster, recovery ...fixtures.ActivityRecoveryRecords) timetablePorts.InstanceLifecycleDependencies {
	recoveryRepo := fixtures.FromActivityRecoveryRecords(compose.NewRetainedActivityRecoveryRepository(s.db, fixtures.IntoInstanceStudentRecords(fixtures.FromInstanceStudentRecords(s.timetable.InstanceStudent))))
	if len(recovery) > 0 {
		recoveryRepo = recovery[0]
	}
	// The deviation protocol's Audit trail writes through an Audit command,
	// as the test service graph routes it.
	audits, err := api.NewRoutedTestAuditPersistence(testpkg.MustDatabase(s.db))
	if err != nil {
		panic(err)
	}
	rooms, err := api.NewFacilities(testpkg.MustDatabase(s.db))
	if err != nil {
		panic(err)
	}
	deps := timetablePorts.InstanceLifecycleDependencies{
		StartConflicts:      lifecycleStartConflicts(s),
		SubstituteConflicts: s.timetableData.Deviations,
		Presence:            s.presence,
		InstanceRepo:        fixtures.IntoActivityInstanceRecords(fixtures.FromActivityInstanceRecords(s.timetable.ActivityInstance)),
		IdempotencyRepo:     fixtures.IntoInstanceIdempotencyRecords(fixtures.FromInstanceIdempotencyRecords(s.timetable.InstanceIdempotency)),
		InstanceStaffRepo:   fixtures.IntoInstanceStaffRecords(fixtures.FromInstanceStaffRecords(s.timetable.InstanceStaff)),
		InstanceStudents:    fixtures.IntoInstanceStudentRecords(fixtures.FromInstanceStudentRecords(s.timetable.InstanceStudent)),
		ExceptionRepo:       fixtures.IntoActivityExceptionRecords(fixtures.FromActivityExceptionRecords(s.timetable.ActivityException)),
		ActiveGroupRepo:     s.repos.ActiveGroup,
		SupervisorRepo:      s.repos.GroupSupervisor,
		Rooms:               api.NewTimetableRoomNames(rooms),
		ActivityGroupRepo:   s.timetable.ActivityGroup,
		StaffRepo:           api.NewTimetableStaff(s.reposPeople.Staff),
		StudentRepo:         api.NewTimetableLifecycleStudents(api.NewTimetableRosterStudents(s.reposPeople.Student)),
		CalendarPeriodRepo:  fixtures.IntoCalendarPeriodRecords(fixtures.FromCalendarPeriodRecords(s.reposSchedules.CalendarPeriod)),
		ActiveService:       s.active,
		Materialization:     s.materialization,
		RecurrenceLock:      s.timetableData.RecurrenceLock,
		CareDays:            lifecycleCareDays{query: s.careDay},
		CareDayLocks:        s.reposCare.CarePlan(),
		Protocol:            api.TimetableDeviationProtocol(audits.DeviationEvent),
		ContentHash:         securityruntime.Fingerprint,
		Database:            compose.NewTenantDatabase(s.db),
		Logger:              slog.Default(),
		RecoveryRepo:        fixtures.IntoActivityRecoveryRecords(recoveryRepo),
	}
	if broadcaster != nil {
		deps.Broadcaster = lifecycleRealtime{broadcaster: broadcaster}
	}
	return deps
}

// lifecycleStartConflicts composes the Timetable owner's conflict detection
// over the lifecycle's repositories and its (possibly fault-injecting)
// presence; the lifecycle only consumes its start check.
func lifecycleStartConflicts(s *lifecycleSetup) timetable.StartConflictQuery {
	detection, err := api.NewTimetableConflictDetection(timetablePorts.ConflictDetectionDependencies{
		Instances: fixtures.IntoConflictInstances(fixtures.FromActivityInstanceRecords(s.timetable.ActivityInstance)), InstanceStaff: fixtures.IntoConflictInstanceStaff(fixtures.FromInstanceStaffRecords(s.timetable.InstanceStaff)), InstanceStudents: fixtures.IntoConflictInstanceStudents(fixtures.FromInstanceStudentRecords(s.timetable.InstanceStudent)),
		Exceptions: fixtures.IntoConflictExceptions(fixtures.FromActivityExceptionRecords(s.timetable.ActivityException)), Schedules: s.timetable.ActivitySchedule, Staff: api.NewTimetableStaff(s.reposPeople.Staff),
		CalendarPeriods: fixtures.IntoConflictCalendarPeriods(fixtures.FromCalendarPeriodRecords(s.reposSchedules.CalendarPeriod)), ArrivalExceptions: fixtures.IntoConflictArrivalExceptions(fixtures.FromStudentArrivalExceptionRecords(s.reposSchedules.StudentArrivalException)),
		Sessions: s.repos.ActiveGroup, Shifts: fixtures.IntoConflictShifts(unusedShiftRows{}), Presence: s.presence,
	}, nil)
	if err != nil {
		panic(err)
	}
	return detection
}

// unusedShiftRows stands in for the Dienstplan rows the start check never
// reads.
type unusedShiftRows struct{}

var errShiftRowsNotUsed = errors.New("the start check reads no Dienstplan rows")

func (unusedShiftRows) FindByDateRange(context.Context, calendar.Date, calendar.Date) ([]*fixtures.StaffShift, error) {
	return nil, errShiftRowsNotUsed
}

func (unusedShiftRows) FindByStaffIDsAndDates(context.Context, []int64, []calendar.Date) ([]*fixtures.StaffShift, error) {
	return nil, errShiftRowsNotUsed
}

func (unusedShiftRows) FindUsedCalendarWeeks(context.Context, calendar.Date, calendar.Date) ([]calendar.Date, error) {
	return nil, errShiftRowsNotUsed
}

// lifecycleCareDays binds Care Plan's care-day verdict to the lifecycle's
// port, as the composition root does: the verdict and the rules that read
// it stay Care Plan's.
type lifecycleCareDays struct {
	query careplan.CareDayQuery
}

func (c lifecycleCareDays) ResolveForDate(ctx context.Context, studentIDs []int64, date calendar.Date) (map[int64]timetable.CareDayStatus, error) {
	resolved, err := c.query.ResolveForDate(ctx, studentIDs, date)
	if err != nil {
		return nil, err
	}
	result := make(map[int64]timetable.CareDayStatus, len(resolved))
	for studentID, status := range resolved {
		result[studentID] = timetable.CareDayStatus(status)
	}
	return result, nil
}

func (c lifecycleCareDays) ResolveForRange(ctx context.Context, studentIDs []int64, from, to calendar.Date) (map[int64]map[calendar.Date]timetable.CareDayStatus, error) {
	resolved, err := c.query.ResolveForRange(ctx, studentIDs, from, to)
	if err != nil {
		return nil, err
	}
	result := make(map[int64]map[calendar.Date]timetable.CareDayStatus, len(resolved))
	for studentID, days := range resolved {
		byDate := make(map[calendar.Date]timetable.CareDayStatus, len(days))
		for date, status := range days {
			byDate[date] = timetable.CareDayStatus(status)
		}
		result[studentID] = byDate
	}
	return result, nil
}

func (lifecycleCareDays) AttendanceRowCareDay(instanceCompleted bool, row timetable.CareDayAttendance, planVerdict timetable.CareDayStatus) timetable.CareDayStatus {
	return timetable.CareDayStatus(careplan.AttendanceRowCareDay(instanceCompleted, &careplan.CareDayAttendance{
		Expected:         row.Expected,
		NotScheduled:     row.NotScheduled,
		ManuallyDecided:  row.ManuallyDecided,
		PlanOwnedAbsence: row.PlanOwnedAbsence,
	}, careplan.CareDayStatus(planVerdict)))
}

func (lifecycleCareDays) Expected(status timetable.CareDayStatus) bool {
	return careplan.CareDayStatus(status).Expected()
}

func (lifecycleCareDays) ExemptFromAbsence(status timetable.CareDayStatus) bool {
	return careplan.CareDayStatus(status).ExemptFromAbsence()
}

// lifecycleRealtime delivers the lifecycle's events to a realtime
// broadcaster with their types unchanged, as the composition root does, so
// the suites assert on the shared test.RecordingBroadcaster.
type lifecycleRealtime struct {
	broadcaster realtime.Broadcaster
}

func lifecycleRealtimeEvent(event timetablePorts.LifecycleEvent) realtime.Event {
	return realtime.NewEvent(realtime.EventType(event.Type), event.ActiveGroupID, realtime.EventData{
		InstanceID:        event.InstanceID,
		InstanceDate:      event.InstanceDate,
		InstanceStartTime: event.InstanceStartTime,
		RoomID:            event.RoomID,
		RoomName:          event.RoomName,
		ActivityName:      event.ActivityName,
		SupervisorIDs:     event.SupervisorIDs,
		StudentIDs:        event.StudentIDs,
		GroupIDs:          event.GroupIDs,
		Reason:            event.Reason,
		Source:            event.Source,
	})
}

func (b lifecycleRealtime) BroadcastToGroup(tenantID int64, topic string, event timetablePorts.LifecycleEvent) error {
	return b.broadcaster.BroadcastToGroup(tenantID, topic, lifecycleRealtimeEvent(event))
}

func (b lifecycleRealtime) BroadcastToTenant(tenantID int64, event timetablePorts.LifecycleEvent) error {
	return b.broadcaster.BroadcastToTenant(tenantID, lifecycleRealtimeEvent(event))
}

// seedInstance inserts one planned activity_instance plus optional staff
// and student rows. Returns the instance for the caller to act on.
func seedInstance(t *testing.T, s *lifecycleSetup, withStaff bool, withStudents bool) *testpkg.ActivityInstance {
	t.Helper()
	ai := &testpkg.ActivityInstance{
		Date:            testpkg.Date(2026, 4, 20),
		ActivityGroupID: &s.tmplID,
		Title:           "Lifecycle-Test",
		StartTime:       time.Date(1, 1, 1, 14, 0, 0, 0, time.UTC),
		EndTime:         time.Date(1, 1, 1, 15, 0, 0, 0, time.UTC),
		RoomID:          s.roomID,
		Status:          fixtures.InstanceStatusPlanned(),
		IsSpontaneous:   false,
	}
	ai.TenantID = testpkg.Tenant(t)
	testpkg.InsertActivityInstanceRow(t, s.ctx, s.db, ai)

	if withStaff {
		row := fixtures.NewInstanceStaff(func(fixtureRow *fixtures.InstanceStaff) {
			fixtureRow.InstanceID = ai.ID
			fixtureRow.StaffID = s.staffID
			fixtureRow.IsPrimary = true
		})
		row.SetTenantID(testpkg.Tenant(t))
		_, err := s.db.NewInsert().Model(row).ModelTableExpr(`schedule.instance_staff`).Exec(s.ctx)
		require.NoError(t, err)
	}
	if withStudents {
		for _, sid := range []int64{s.student1, s.student2} {
			row := &testpkg.InstanceStudent{InstanceID: ai.ID, StudentID: sid, Status: fixtures.AttendanceStatusExpected()}
			row.TenantID = testpkg.Tenant(t)
			testpkg.InsertInstanceStudentRow(t, s.ctx, s.db, row)
		}
	}
	return ai
}

func seedSpontaneousInstance(t *testing.T, s *lifecycleSetup, withStaff bool) *testpkg.ActivityInstance {
	t.Helper()
	ai := &testpkg.ActivityInstance{
		Date:          testpkg.Date(2026, 4, 20),
		Title:         "Lifecycle-Test-Spontaneous",
		StartTime:     time.Date(1, 1, 1, 14, 0, 0, 0, time.UTC),
		EndTime:       time.Date(1, 1, 1, 15, 0, 0, 0, time.UTC),
		RoomID:        s.roomID,
		Status:        fixtures.InstanceStatusPlanned(),
		IsSpontaneous: true,
	}
	ai.TenantID = testpkg.Tenant(t)
	testpkg.InsertActivityInstanceRow(t, s.ctx, s.db, ai)
	if withStaff {
		row := fixtures.NewInstanceStaff(func(fixtureRow *fixtures.InstanceStaff) {
			fixtureRow.InstanceID = ai.ID
			fixtureRow.StaffID = s.staffID
			fixtureRow.IsPrimary = true
		})
		row.SetTenantID(testpkg.Tenant(t))
		_, err := s.db.NewInsert().Model(row).ModelTableExpr(`schedule.instance_staff`).Exec(s.ctx)
		require.NoError(t, err)
	}
	return ai
}

// forceSetInstanceStatus moves an instance into a lifecycle state directly,
// through the plan and its Student Presence session. Used by 409-path tests
// to reach a state that can't be reached via the public API (e.g.
// cancelled-from-scratch).
func forceSetInstanceStatus(t *testing.T, s *lifecycleSetup, id int64, status string) {
	t.Helper()
	testpkg.SetActivityInstanceLifecycle(t, s.ctx, s.db, id, status)
}

// reloadInstance reads the stored row of a block, for the columns the
// lifecycle's public view does not carry.
func reloadInstance(t *testing.T, s *lifecycleSetup, id int64) *fixtures.ActivityInstance {
	t.Helper()
	row, err := fixtures.FromActivityInstanceRecords(s.timetable.ActivityInstance).FindByID(s.ctx, id)
	require.NoError(t, err)
	require.NotNil(t, row)
	return row
}

func insertInstance(t *testing.T, s *lifecycleSetup, date calendar.Date, status string, spontaneous bool) int64 {
	t.Helper()
	return insertInstanceAt(t, s, date, status, spontaneous, 14)
}

func insertInstanceAt(t *testing.T, s *lifecycleSetup, date calendar.Date, status string, spontaneous bool, startHour int) int64 {
	t.Helper()
	endHour := startHour + 1
	row := &testpkg.ActivityInstance{
		Date:          testpkg.ScheduleDate(date),
		Title:         fmt.Sprintf("Row-%s-%d", status, time.Now().UnixNano()),
		StartTime:     time.Date(1, 1, 1, startHour, 0, 0, 0, time.UTC),
		EndTime:       time.Date(1, 1, 1, endHour, 0, 0, 0, time.UTC),
		RoomID:        s.roomID,
		Status:        status,
		IsSpontaneous: spontaneous,
	}
	if !spontaneous {
		row.ActivityGroupID = &s.tmplID
	}
	row.TenantID = testpkg.Tenant(t)
	testpkg.InsertActivityInstanceRow(t, s.ctx, s.db, row)
	return row.ID
}

func countRowsWhere(t *testing.T, s *lifecycleSetup, table, column string, value int64) int {
	t.Helper()
	var count int
	err := s.db.NewSelect().
		TableExpr(table).
		ColumnExpr("COUNT(*)").
		Where(column+" = ?", value).
		Where("tenant_id = ?", testpkg.Tenant(t)).
		Scan(s.ctx, &count)
	require.NoError(t, err)
	return count
}

func instanceExists(t *testing.T, s *lifecycleSetup, id int64) bool {
	t.Helper()
	var count int
	err := s.db.NewSelect().
		TableExpr(`schedule.activity_instances AS "ai"`).
		ColumnExpr("COUNT(*)").
		Where(`"ai".id = ?`, id).
		Where(`"ai".tenant_id = ?`, testpkg.Tenant(t)).
		Scan(s.ctx, &count)
	require.NoError(t, err)
	return count > 0
}

func loadLifecycleExceptions(t *testing.T, s *lifecycleSetup) []*fixtures.ActivityException {
	t.Helper()
	var rows []*fixtures.ActivityException
	err := s.db.NewSelect().
		Model(&rows).
		ModelTableExpr(`schedule.activity_exceptions AS "activity_exception"`).
		Where(`"activity_exception".tenant_id = ?`, testpkg.Tenant(t)).
		Order("exception_date ASC").
		Scan(s.ctx)
	require.NoError(t, err)
	return rows
}

// fetchAttendance loads a participant with its attendance by (instance_id,
// student_id). The tests use it to assert status after lifecycle transitions.
func fetchAttendance(t *testing.T, s *lifecycleSetup, instanceID, studentID int64) *testpkg.InstanceStudent {
	t.Helper()
	rows := testpkg.InstanceStudentsWhere(t, s.ctx, s.db,
		`"instance_student".instance_id = ? AND "instance_student".student_id = ? AND "instance_student".tenant_id = ?`,
		instanceID, studentID, testpkg.Tenant(t))
	require.Len(t, rows, 1)
	return rows[0]
}

// deviationEventRow is one stored audit.deviation_events row, read back
// directly: the Audit Platform owns the model, this suite only asserts what
// the lifecycle's protocol writes landed.
type deviationEventRow struct {
	bun.BaseModel   `bun:"table:audit.deviation_events,alias:deviation_event"`
	ID              int64           `bun:"id,pk"`
	TenantID        int64           `bun:"tenant_id"`
	ActivityGroupID *int64          `bun:"activity_group_id"`
	OccurrenceDate  calendar.Date   `bun:"occurrence_date,type:date"`
	InstanceID      *int64          `bun:"instance_id"`
	SubjectStaffID  *int64          `bun:"subject_staff_id"`
	ActorAccountID  *int64          `bun:"actor_account_id"`
	EventType       string          `bun:"event_type"`
	OldValue        json.RawMessage `bun:"old_value,type:jsonb"`
	NewValue        json.RawMessage `bun:"new_value,type:jsonb"`
}

// loadDeviationEvents returns every protocol row of one tenant, oldest first.
func loadDeviationEvents(t *testing.T, db *bun.DB, ctx context.Context, tenantID int64) []*deviationEventRow {
	t.Helper()
	var rows []*deviationEventRow
	err := db.NewSelect().
		Model(&rows).
		ModelTableExpr(`audit.deviation_events AS "deviation_event"`).
		Where(`"deviation_event".tenant_id = ?`, tenantID).
		Order("id ASC").
		Scan(ctx)
	require.NoError(t, err)
	return rows
}
