package httpintegration_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/securityruntime"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/modules/timetable/compose"
	"github.com/moto-nrw/project-phoenix/realtime"
	"github.com/moto-nrw/project-phoenix/services"
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
	return repositories.NewStudentPresenceForTests(db)
}

// lifecycleSetup bundles the wired lifecycle + common fixtures. Rows belong
// to the test's own tenant clone; nothing is cleaned up explicitly.
type lifecycleSetup struct {
	presence compose.LifecyclePresence
	svc      *compose.InstanceLifecycleService
	factory  *services.Factory
	repos    *repositories.Factory
	db       *bun.DB
	ctx      context.Context
	roomID   int64
	staffID  int64
	student1 int64
	student2 int64
	tmplID   int64
	period   *scheduleModels.CalendarPeriod
}

// buildLifecycle prepares the minimum scaffold for instance-lifecycle tests:
// one tenant, one room, one staff, two students, one template (is_template).
func buildLifecycle(t *testing.T) *lifecycleSetup {
	t.Helper()
	db := testpkg.SetupTestDB(t)

	repoFactory := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db))
	serviceFactory, err := services.NewFactoryForTests(repoFactory, db, slog.Default())
	require.NoError(t, err)
	require.NoError(t, serviceFactory.SetTenantRuntime(testpkg.TenantRuntime(t, db)))

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
		presence: newInstancePresence(t, db),
		factory:  serviceFactory,
		repos:    repoFactory,
		db:       db,
		ctx:      ctx,
		roomID:   room.ID,
		staffID:  staff.ID,
		student1: student1.ID,
		student2: student2.ID,
		tmplID:   templateRow.ID,
		period:   period,
	}
	// These state-machine fixtures use fixed wall-clock windows. Time-policy
	// boundaries have dedicated clock-injected tests; keep this suite focused on
	// lifecycle persistence and bridge behavior.
	setup.svc = instanceServiceWithBroadcaster(t, setup, nil)
	return setup
}

func instanceServiceWithBroadcaster(t *testing.T, s *lifecycleSetup, broadcaster realtime.Broadcaster, recovery ...scheduleModels.ActivityRecoveryRepository) *compose.InstanceLifecycleService {
	t.Helper()
	return newLifecycle(t, lifecycleDependencies(s, broadcaster, recovery...))
}

func newLifecycle(t *testing.T, deps compose.InstanceLifecycleDependencies) *compose.InstanceLifecycleService {
	t.Helper()
	svc, err := compose.NewInstanceLifecycle(deps)
	require.NoError(t, err)
	return svc
}

// newLifecycleAutoEnd composes the scheduler's auto end over the lifecycle,
// as the composition root does.
func newLifecycleAutoEnd(t *testing.T, instances scheduleModels.ActivityInstanceRepository, lifecycle timetable.InstanceLifecycle) timetable.InstanceAutoEnd {
	t.Helper()
	autoEnd, err := compose.NewInstanceAutoEnd(instances, lifecycle)
	require.NoError(t, err)
	return autoEnd
}

// lifecycleDependencies is the dependency set instanceServiceWithBroadcaster
// wires, for tests that add an optional collaborator before construction.
func lifecycleDependencies(s *lifecycleSetup, broadcaster realtime.Broadcaster, recovery ...scheduleModels.ActivityRecoveryRepository) compose.InstanceLifecycleDependencies {
	recoveryRepo := repositories.NewActivityRecoveryRepository(s.db, s.repos.InstanceStudent)
	if len(recovery) > 0 {
		recoveryRepo = recovery[0]
	}
	deps := compose.InstanceLifecycleDependencies{
		StartConflicts:      lifecycleStartConflicts(s),
		SubstituteConflicts: s.factory.TimetableData.Deviations,
		Presence:            s.presence,
		InstanceRepo:        s.repos.ActivityInstance,
		IdempotencyRepo:     s.repos.InstanceIdempotency,
		InstanceStaffRepo:   s.repos.InstanceStaff,
		InstanceStudents:    s.repos.InstanceStudent,
		ExceptionRepo:       s.repos.ActivityException,
		ActiveGroupRepo:     s.repos.ActiveGroup,
		SupervisorRepo:      s.repos.GroupSupervisor,
		Rooms:               repositories.TimetableOwnerRows{Rooms: s.repos.Room}.LifecycleRooms(),
		ActivityGroupRepo:   s.repos.ActivityGroup,
		StaffRepo:           s.repos.Staff,
		StudentRepo:         s.repos.Student,
		CalendarPeriodRepo:  s.repos.CalendarPeriod,
		ActiveService:       s.factory.Active,
		Materialization:     s.factory.Materialization,
		RecurrenceLock:      s.factory.TimetableData.RecurrenceLock,
		CareDays:            lifecycleCareDays{query: s.factory.CareDay},
		CareDayLocks:        s.repos.CarePlan(),
		Protocol:            repositories.TimetableDeviationProtocol(s.repos.DeviationEvent),
		ContentHash:         securityruntime.Fingerprint,
		DB:                  s.db,
		Logger:              slog.Default(),
		RecoveryRepo:        recoveryRepo,
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
	detection, err := services.NewTimetableConflictDetection(services.TimetableConflictReaders{
		Instances: s.repos.ActivityInstance, InstanceStaff: s.repos.InstanceStaff, InstanceStudents: s.repos.InstanceStudent,
		Exceptions: s.repos.ActivityException, Schedules: s.repos.ActivitySchedule, Staff: s.repos.Staff,
		CalendarPeriods: s.repos.CalendarPeriod, ArrivalExceptions: s.repos.StudentArrivalException,
		Sessions: s.repos.ActiveGroup, Shifts: unusedShiftRows{}, Presence: s.presence,
	})
	if err != nil {
		panic(err)
	}
	return detection
}

// unusedShiftRows stands in for the Dienstplan rows the start check never
// reads.
type unusedShiftRows struct{}

var errShiftRowsNotUsed = errors.New("the start check reads no Dienstplan rows")

func (unusedShiftRows) FindByDateRange(context.Context, scheduleModels.Date, scheduleModels.Date) ([]*scheduleModels.StaffShift, error) {
	return nil, errShiftRowsNotUsed
}

func (unusedShiftRows) FindByStaffIDsAndDates(context.Context, []int64, []scheduleModels.Date) ([]*scheduleModels.StaffShift, error) {
	return nil, errShiftRowsNotUsed
}

func (unusedShiftRows) FindUsedCalendarWeeks(context.Context, scheduleModels.Date, scheduleModels.Date) ([]scheduleModels.Date, error) {
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

func lifecycleRealtimeEvent(event compose.LifecycleEvent) realtime.Event {
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

func (b lifecycleRealtime) BroadcastToGroup(tenantID int64, topic string, event compose.LifecycleEvent) error {
	return b.broadcaster.BroadcastToGroup(tenantID, topic, lifecycleRealtimeEvent(event))
}

func (b lifecycleRealtime) BroadcastToTenant(tenantID int64, event compose.LifecycleEvent) error {
	return b.broadcaster.BroadcastToTenant(tenantID, lifecycleRealtimeEvent(event))
}

// seedInstance inserts one planned activity_instance plus optional staff
// and student rows. Returns the instance for the caller to act on.
func seedInstance(t *testing.T, s *lifecycleSetup, withStaff bool, withStudents bool) *scheduleModels.ActivityInstance {
	t.Helper()
	ai := &scheduleModels.ActivityInstance{
		Date:            scheduleModels.NewDate(2026, 4, 20),
		ActivityGroupID: &s.tmplID,
		Title:           "Lifecycle-Test",
		StartTime:       time.Date(1, 1, 1, 14, 0, 0, 0, time.UTC),
		EndTime:         time.Date(1, 1, 1, 15, 0, 0, 0, time.UTC),
		RoomID:          s.roomID,
		Status:          scheduleModels.InstanceStatusPlanned,
		IsSpontaneous:   false,
	}
	ai.SetTenantID(testpkg.Tenant(t))
	_, err := s.db.NewInsert().Model(ai).ModelTableExpr(`schedule.activity_instances`).Exec(s.ctx)
	require.NoError(t, err)

	if withStaff {
		row := &scheduleModels.InstanceStaff{InstanceID: ai.ID, StaffID: s.staffID, IsPrimary: true}
		row.SetTenantID(testpkg.Tenant(t))
		_, err = s.db.NewInsert().Model(row).ModelTableExpr(`schedule.instance_staff`).Exec(s.ctx)
		require.NoError(t, err)
	}
	if withStudents {
		for _, sid := range []int64{s.student1, s.student2} {
			row := &scheduleModels.InstanceStudent{InstanceID: ai.ID, StudentID: sid, Status: scheduleModels.AttendanceStatusExpected}
			row.SetTenantID(testpkg.Tenant(t))
			_, err = s.db.NewInsert().Model(row).ModelTableExpr(`schedule.instance_students`).Exec(s.ctx)
			require.NoError(t, err)
		}
	}
	return ai
}

func seedSpontaneousInstance(t *testing.T, s *lifecycleSetup, withStaff bool) *scheduleModels.ActivityInstance {
	t.Helper()
	ai := &scheduleModels.ActivityInstance{
		Date:          scheduleModels.NewDate(2026, 4, 20),
		Title:         "Lifecycle-Test-Spontaneous",
		StartTime:     time.Date(1, 1, 1, 14, 0, 0, 0, time.UTC),
		EndTime:       time.Date(1, 1, 1, 15, 0, 0, 0, time.UTC),
		RoomID:        s.roomID,
		Status:        scheduleModels.InstanceStatusPlanned,
		IsSpontaneous: true,
	}
	ai.SetTenantID(testpkg.Tenant(t))
	_, err := s.db.NewInsert().Model(ai).ModelTableExpr(`schedule.activity_instances`).Exec(s.ctx)
	require.NoError(t, err)
	if withStaff {
		row := &scheduleModels.InstanceStaff{InstanceID: ai.ID, StaffID: s.staffID, IsPrimary: true}
		row.SetTenantID(testpkg.Tenant(t))
		_, err = s.db.NewInsert().Model(row).ModelTableExpr(`schedule.instance_staff`).Exec(s.ctx)
		require.NoError(t, err)
	}
	return ai
}

// forceSetInstanceStatus rewrites the status column directly. Used by 409-path
// tests to move an instance into a state that can't be reached via the public
// API (e.g. cancelled-from-scratch).
func forceSetInstanceStatus(t *testing.T, s *lifecycleSetup, id int64, status string) {
	t.Helper()
	_, err := s.db.NewUpdate().
		Model((*scheduleModels.ActivityInstance)(nil)).
		ModelTableExpr(`schedule.activity_instances AS "activity_instance"`).
		Set("status = ?", status).
		Where(`"activity_instance".id = ?`, id).
		Where("tenant_id = ?", testpkg.Tenant(t)).
		Exec(s.ctx)
	require.NoError(t, err)
}

// reloadInstance reads the stored row of a block, for the columns the
// lifecycle's public view does not carry.
func reloadInstance(t *testing.T, s *lifecycleSetup, id int64) *scheduleModels.ActivityInstance {
	t.Helper()
	row, err := s.repos.ActivityInstance.FindByID(s.ctx, id)
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
	row := &scheduleModels.ActivityInstance{
		Date:          scheduleModels.Date(date),
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
	row.SetTenantID(testpkg.Tenant(t))
	_, err := s.db.NewInsert().Model(row).ModelTableExpr(`schedule.activity_instances`).Exec(s.ctx)
	require.NoError(t, err)
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

func loadLifecycleExceptions(t *testing.T, s *lifecycleSetup) []*scheduleModels.ActivityException {
	t.Helper()
	var rows []*scheduleModels.ActivityException
	err := s.db.NewSelect().
		Model(&rows).
		ModelTableExpr(`schedule.activity_exceptions AS "activity_exception"`).
		Where(`"activity_exception".tenant_id = ?`, testpkg.Tenant(t)).
		Order("exception_date ASC").
		Scan(s.ctx)
	require.NoError(t, err)
	return rows
}

// fetchAttendance loads an instance_student row by (instance_id, student_id).
// The tests use it to assert status after lifecycle transitions.
func fetchAttendance(t *testing.T, s *lifecycleSetup, instanceID, studentID int64) *scheduleModels.InstanceStudent {
	t.Helper()
	var row scheduleModels.InstanceStudent
	err := s.db.NewSelect().
		Model(&row).
		ModelTableExpr(`schedule.instance_students AS "instance_student"`).
		Where(`"instance_student".instance_id = ?`, instanceID).
		Where(`"instance_student".student_id = ?`, studentID).
		Where(`"instance_student".tenant_id = ?`, testpkg.Tenant(t)).
		Scan(s.ctx)
	require.NoError(t, err)
	return &row
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
