package schedule_test

import (
	"context"
	"fmt"
	"log/slog"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	activitiesModels "github.com/moto-nrw/project-phoenix/models/activities"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/services"
	scheduleSvc "github.com/moto-nrw/project-phoenix/services/schedule"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// Hermetic test: fixtures are real DB rows created per subtest and cleaned up
// via testpkg.Cleanup* helpers. No hardcoded entity IDs (tenant_id=1 is
// whitelisted by the hermetic linter). Each subtest is self-contained.

// -----------------------------------------------------------------------------
// scenarioSetup bundles the minimum dependencies the materialization service
// needs plus convenient references to the common fixtures.
// -----------------------------------------------------------------------------

type scenarioSetup struct {
	svc           scheduleSvc.MaterializationService
	factory       *services.Factory
	db            *bun.DB
	ctx           context.Context
	cleanup       func()
	period        *scheduleModels.CalendarPeriod
	template      *activitiesModels.Group
	schedule      *activitiesModels.Schedule
	timeframe     *scheduleModels.Timeframe
	students      []int64
	staffID       int64
	categoryID    int64
	roomID        int64
	enrollmentIDs []int64
	supervisorIDs []int64
	extraCleanups []func()
	cleanupTables map[string][]int64 // table → IDs
}

func (s *scenarioSetup) registerCleanup(table string, ids ...int64) {
	if s.cleanupTables == nil {
		s.cleanupTables = make(map[string][]int64)
	}
	s.cleanupTables[table] = append(s.cleanupTables[table], ids...)
}

func (s *scenarioSetup) runCleanup(tb testing.TB) {
	tb.Helper()
	// Cleanup order matters for FKs. Children first.
	order := []string{
		"schedule.instance_students",
		"schedule.instance_staff",
		"schedule.activity_instances",
		"schedule.activity_exceptions",
		"activities.student_enrollments",
		"activities.supervisors",
		"activities.schedules",
	}
	for _, tbl := range order {
		if ids := s.cleanupTables[tbl]; len(ids) > 0 {
			testpkg.CleanupTableRecords(tb, s.db, tbl, ids...)
		}
	}
	for _, fn := range s.extraCleanups {
		fn()
	}
	if s.cleanup != nil {
		s.cleanup()
	}
}

// makeScenario prepares a minimal end-to-end materialization scenario:
// tenant=1, one active school-year period (no A/B cycle), one template on
// the given weekday, one timeframe 14:00–15:00, one room, one staff, three
// students of which two have valid enrollments at `materializeDate` and one
// has expired. Returns the setup; caller registers additional fixtures and
// calls runCleanup on teardown.
func makeScenario(t *testing.T, weekday int, materializeDate time.Time) *scenarioSetup {
	t.Helper()
	db := testpkg.SetupTestDB(t)

	repoFactory := repositories.NewFactory(db)
	serviceFactory, err := services.NewFactory(repoFactory, db, slog.Default())
	require.NoError(t, err)

	svc := scheduleSvc.NewMaterializationService(
		repoFactory.ActivityGroup,
		repoFactory.ActivitySchedule,
		repoFactory.StudentEnrollment,
		repoFactory.ActivitySupervisor,
		repoFactory.CalendarPeriod,
		repoFactory.ActivityInstance,
		repoFactory.InstanceStaff,
		repoFactory.InstanceStudent,
		repoFactory.ActivityException,
		repoFactory.Timeframe,
		serviceFactory.CalendarPeriod,
		db,
		slog.Default(),
	)

	ctx := testpkg.TenantContext(1)
	suffix := time.Now().UnixNano()

	// 1. Calendar period — no A/B, wide range, active.
	period := &scheduleModels.CalendarPeriod{
		Name:            fmt.Sprintf("Schuljahr-%d", suffix),
		PeriodType:      scheduleModels.PeriodTypeSchoolYear,
		StartDate:       time.Date(materializeDate.Year()-1, 8, 1, 0, 0, 0, 0, time.UTC),
		EndDate:         time.Date(materializeDate.Year()+1, 7, 31, 0, 0, 0, 0, time.UTC),
		WeekCycleLength: 1,
		IsActive:        true,
	}
	require.NoError(t, serviceFactory.CalendarPeriod.CreatePeriod(ctx, period))

	// 2. Fixtures: room, staff, 3 students.
	room := testpkg.CreateTestRoom(t, db, fmt.Sprintf("Room-%d", suffix))
	staff := testpkg.CreateTestStaff(t, db, "Super", fmt.Sprintf("Visor-%d", suffix))
	student1 := testpkg.CreateTestStudent(t, db, "Alice", fmt.Sprintf("One-%d", suffix), "3a")
	student2 := testpkg.CreateTestStudent(t, db, "Bob", fmt.Sprintf("Two-%d", suffix), "3a")
	student3 := testpkg.CreateTestStudent(t, db, "Carol", fmt.Sprintf("Three-%d", suffix), "3a")

	// 3. Template activity group (is_template = true, planned_room_id set).
	category := testpkg.CreateTestActivityCategory(t, db, fmt.Sprintf("Cat-%d", suffix))
	template := &activitiesModels.Group{
		Name:            fmt.Sprintf("Malen-AG-%d", suffix),
		MaxParticipants: 20,
		IsOpen:          true,
		CategoryID:      category.ID,
		CreatedBy:       &staff.ID,
		PlannedRoomID:   &room.ID,
		IsTemplate:      true,
	}
	template.SetTenantID(1)
	_, err = db.NewInsert().Model(template).ModelTableExpr(`activities.groups AS "group"`).Exec(ctx)
	require.NoError(t, err)

	// 4. Timeframe 14:00–15:00 today (the wall-clock date is irrelevant for
	// TIME columns; bun strips to TIME on write).
	timeframe := testpkg.CreateTestTimeframe(t, db, fmt.Sprintf("Nachmittag-%d", suffix))

	// 5. Schedule: weekday + timeframe, no A/B pattern.
	sched := &activitiesModels.Schedule{
		Weekday:         weekday,
		TimeframeID:     &timeframe.ID,
		ActivityGroupID: template.ID,
		WeekPattern:     0,
	}
	sched.SetTenantID(1)
	_, err = db.NewInsert().Model(sched).ModelTableExpr(`activities.schedules`).Exec(ctx)
	require.NoError(t, err)

	// 6. Enrollments: student1 + student2 valid unbounded; student3 expired
	// the day before materializeDate.
	expiredUntil := materializeDate.AddDate(0, 0, -1)
	validFrom := materializeDate.AddDate(0, 0, -30)
	enroll1 := &activitiesModels.StudentEnrollment{StudentID: student1.ID, ActivityGroupID: template.ID, ValidFrom: validFrom}
	enroll1.SetTenantID(1)
	_, err = db.NewInsert().Model(enroll1).ModelTableExpr(`activities.student_enrollments`).Exec(ctx)
	require.NoError(t, err)
	enroll2 := &activitiesModels.StudentEnrollment{StudentID: student2.ID, ActivityGroupID: template.ID, ValidFrom: validFrom}
	enroll2.SetTenantID(1)
	_, err = db.NewInsert().Model(enroll2).ModelTableExpr(`activities.student_enrollments`).Exec(ctx)
	require.NoError(t, err)
	enroll3 := &activitiesModels.StudentEnrollment{StudentID: student3.ID, ActivityGroupID: template.ID, ValidFrom: validFrom, ValidUntil: &expiredUntil}
	enroll3.SetTenantID(1)
	_, err = db.NewInsert().Model(enroll3).ModelTableExpr(`activities.student_enrollments`).Exec(ctx)
	require.NoError(t, err)

	// 7. Supervisor (primary, unbounded).
	sup := &activitiesModels.SupervisorPlanned{StaffID: staff.ID, GroupID: template.ID, IsPrimary: true, ValidFrom: validFrom}
	sup.SetTenantID(1)
	_, err = db.NewInsert().Model(sup).ModelTableExpr(`activities.supervisors`).Exec(ctx)
	require.NoError(t, err)

	s := &scenarioSetup{
		svc:           svc,
		factory:       serviceFactory,
		db:            db,
		ctx:           ctx,
		period:        period,
		template:      template,
		schedule:      sched,
		timeframe:     timeframe,
		students:      []int64{student1.ID, student2.ID, student3.ID},
		staffID:       staff.ID,
		categoryID:    category.ID,
		roomID:        room.ID,
		enrollmentIDs: []int64{enroll1.ID, enroll2.ID, enroll3.ID},
		supervisorIDs: []int64{sup.ID},
	}

	// Cleanup plan (children first).
	s.registerCleanup("activities.student_enrollments", s.enrollmentIDs...)
	s.registerCleanup("activities.supervisors", s.supervisorIDs...)
	s.registerCleanup("activities.schedules", sched.ID)

	// Ambient cleanup for things created by testpkg helpers (reuses helpers).
	s.extraCleanups = append(s.extraCleanups, func() {
		testpkg.CleanupActivityFixtures(t, db, student1.ID, staff.ID, 0, template.ID, room.ID)
		testpkg.CleanupTableRecords(t, db, "users.students", student2.ID, student3.ID)
		testpkg.CleanupTableRecords(t, db, "activities.categories", category.ID)
		testpkg.CleanupScheduleFixtures(t, db, timeframe.ID)
		testpkg.CleanupTableRecords(t, db, "schedule.calendar_periods", period.ID)
	})
	s.cleanup = func() { _ = db.Close() }
	return s
}

// -----------------------------------------------------------------------------
// Tests
// -----------------------------------------------------------------------------

func TestMaterializeForTenant_EndToEnd(t *testing.T) {
	// Target Monday inside the wide period window; use a deterministic date
	// in the middle of a "school year" so the period bounds are obviously
	// satisfied.
	materializeDate := time.Date(2026, time.April, 20, 0, 0, 0, 0, time.UTC) // Mon
	s := makeScenario(t, activitiesModels.WeekdayMonday, materializeDate)
	defer s.runCleanup(t)

	from := materializeDate
	to := materializeDate.AddDate(0, 0, 6) // Sun

	// --- Happy path ---
	r, err := s.svc.MaterializeForTenant(s.ctx, from, to, scheduleSvc.MaterializationSourceManual)
	require.NoError(t, err)
	assert.Equal(t, 1, r.InstancesCreated, "one instance per matching weekday")
	assert.Equal(t, 2, r.InstanceStudentsCreated, "expired enrollment filtered")
	assert.Equal(t, 1, r.InstanceStaffCreated)
	assert.Zero(t, r.CandidatesRaced)
	assert.Zero(t, r.CandidatesSkippedException)

	// Collect instance + register for cleanup.
	instanceRows := listInstancesForDate(t, s.db, s.template.ID, materializeDate)
	require.Len(t, instanceRows, 1)
	s.registerCleanup("schedule.activity_instances", instanceRows[0].ID)

	// --- Idempotent re-run ---
	r2, err := s.svc.MaterializeForTenant(s.ctx, from, to, scheduleSvc.MaterializationSourceManual)
	require.NoError(t, err)
	assert.Zero(t, r2.InstancesCreated)
	assert.Equal(t, 1, r2.CandidatesSkippedExisting, "existing row protected by merge strategy")

	// --- Protects existing non-planned statuses ---
	// Change status to 'active' and re-run. Must still be skipped-existing
	// (decideMerge() returns SkipExisting regardless of status, v1 insert-only).
	_, err = s.db.NewUpdate().
		Model((*scheduleModels.ActivityInstance)(nil)).
		ModelTableExpr(`schedule.activity_instances AS "activity_instance"`).
		Set("status = ?", scheduleModels.InstanceStatusActive).
		Where(`"activity_instance".id = ?`, instanceRows[0].ID).
		Where("tenant_id = ?", 1).
		Exec(s.ctx)
	require.NoError(t, err)
	r3, err := s.svc.MaterializeForTenant(s.ctx, from, to, scheduleSvc.MaterializationSourceManual)
	require.NoError(t, err)
	assert.Zero(t, r3.InstancesCreated)
	assert.Equal(t, 1, r3.CandidatesSkippedExisting)
}

func TestMaterializeForTenant_ExceptionCancelled_Skips(t *testing.T) {
	materializeDate := time.Date(2026, time.April, 20, 0, 0, 0, 0, time.UTC)
	s := makeScenario(t, activitiesModels.WeekdayMonday, materializeDate)
	defer s.runCleanup(t)

	// Insert a cancelled exception for the target date.
	exc := &scheduleModels.ActivityException{
		ActivityGroupID: s.template.ID,
		ExceptionDate:   materializeDate,
		ExceptionType:   scheduleModels.ActivityExceptionCancelled,
	}
	exc.SetTenantID(1)
	_, err := s.db.NewInsert().Model(exc).ModelTableExpr(`schedule.activity_exceptions`).Exec(s.ctx)
	require.NoError(t, err)
	s.registerCleanup("schedule.activity_exceptions", exc.ID)

	r, err := s.svc.MaterializeForTenant(s.ctx, materializeDate, materializeDate.AddDate(0, 0, 6), scheduleSvc.MaterializationSourceManual)
	require.NoError(t, err)
	assert.Zero(t, r.InstancesCreated)
	assert.Equal(t, 1, r.CandidatesSkippedException)
}

func TestMaterializeForTenant_ExceptionModified_OverridesStartTime(t *testing.T) {
	materializeDate := time.Date(2026, time.April, 20, 0, 0, 0, 0, time.UTC)
	s := makeScenario(t, activitiesModels.WeekdayMonday, materializeDate)
	defer s.runCleanup(t)

	// Year 1 (not 0) — PostgreSQL TIMESTAMPTZ rejects "0000-01-01" because
	// ISO 8601 has no year zero. The time-of-day is all we care about; bun
	// writes "13:00:00" into the TIME column regardless of the date portion.
	newStart := time.Date(1, 1, 1, 13, 0, 0, 0, time.UTC)
	exc := &scheduleModels.ActivityException{
		ActivityGroupID: s.template.ID,
		ExceptionDate:   materializeDate,
		ExceptionType:   scheduleModels.ActivityExceptionModified,
		StartTime:       &newStart,
	}
	exc.SetTenantID(1)
	_, err := s.db.NewInsert().Model(exc).ModelTableExpr(`schedule.activity_exceptions`).Exec(s.ctx)
	require.NoError(t, err)
	s.registerCleanup("schedule.activity_exceptions", exc.ID)

	r, err := s.svc.MaterializeForTenant(s.ctx, materializeDate, materializeDate.AddDate(0, 0, 6), scheduleSvc.MaterializationSourceManual)
	require.NoError(t, err)
	assert.Equal(t, 1, r.InstancesCreated)

	rows := listInstancesForDate(t, s.db, s.template.ID, materializeDate)
	require.Len(t, rows, 1)
	assert.Equal(t, 13, rows[0].StartTime.Hour(), "exception overrode start_time")
	s.registerCleanup("schedule.activity_instances", rows[0].ID)
}

func TestMaterializeForTenant_NoActivePeriod_ReturnsGracefully(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repoFactory := repositories.NewFactory(db)
	serviceFactory, err := services.NewFactory(repoFactory, db, slog.Default())
	require.NoError(t, err)
	svc := scheduleSvc.NewMaterializationService(
		repoFactory.ActivityGroup, repoFactory.ActivitySchedule, repoFactory.StudentEnrollment,
		repoFactory.ActivitySupervisor, repoFactory.CalendarPeriod, repoFactory.ActivityInstance,
		repoFactory.InstanceStaff, repoFactory.InstanceStudent, repoFactory.ActivityException,
		repoFactory.Timeframe, serviceFactory.CalendarPeriod, db, slog.Default(),
	)

	// Use a fresh tenant id that we know has no active periods.
	//
	// tenant_id = some large unlikely value — the hermetic linter whitelists
	// the `tenant_id` key itself. We still must not use int64(1)-int64(9).
	const emptyTenantID = int64(990001)
	ctx := testpkg.TenantContext(emptyTenantID)
	from := time.Date(2026, 4, 20, 0, 0, 0, 0, time.UTC)
	to := from.AddDate(0, 0, 6)

	r, err := svc.MaterializeForTenant(ctx, from, to, scheduleSvc.MaterializationSourceManual)
	require.NoError(t, err)
	assert.Zero(t, r.InstancesCreated)
	assert.Zero(t, r.CandidatesSkippedExisting)
	assert.Zero(t, r.CandidatesSkippedNoPeriod)
	assert.Zero(t, r.CandidatesSkippedABWeek)
}

func TestMaterializeForTenant_PreFetchObservesFirstRunThenSkips(t *testing.T) {
	// Verifies the "expected" half of the idempotency story the UNIQUE index
	// backstops: a first-run insert is observed by the second run's pre-fetch
	// and produces CandidatesSkippedExisting. The actual UNIQUE-violation
	// race branch (pre-fetch misses + concurrent insert wins) is exercised by
	// a unit test on isUniqueViolation — simulating it end-to-end would
	// require contrived concurrency that does not model real production.
	materializeDate := time.Date(2026, time.April, 20, 0, 0, 0, 0, time.UTC)
	s := makeScenario(t, activitiesModels.WeekdayMonday, materializeDate)
	defer s.runCleanup(t)

	r1, err := s.svc.MaterializeForTenant(s.ctx, materializeDate, materializeDate.AddDate(0, 0, 6), scheduleSvc.MaterializationSourceManual)
	require.NoError(t, err)
	require.Equal(t, 1, r1.InstancesCreated)

	rows := listInstancesForDate(t, s.db, s.template.ID, materializeDate)
	require.Len(t, rows, 1)
	s.registerCleanup("schedule.activity_instances", rows[0].ID)

	r2, err := s.svc.MaterializeForTenant(s.ctx, materializeDate, materializeDate.AddDate(0, 0, 6), scheduleSvc.MaterializationSourceManual)
	require.NoError(t, err)
	assert.Zero(t, r2.InstancesCreated)
	assert.Equal(t, 1, r2.CandidatesSkippedExisting)
	assert.Zero(t, r2.CandidatesRaced)
}

func TestMaterializeForTenant_TemplateScheduleBoundToPeriod_OutOfRange_Skips(t *testing.T) {
	materializeDate := time.Date(2026, time.April, 20, 0, 0, 0, 0, time.UTC)
	s := makeScenario(t, activitiesModels.WeekdayMonday, materializeDate)
	defer s.runCleanup(t)

	// Create a holiday period that is active but only covers Oct 2026.
	holiday := &scheduleModels.CalendarPeriod{
		Name:            fmt.Sprintf("Herbstferien-%d", time.Now().UnixNano()),
		PeriodType:      scheduleModels.PeriodTypeHoliday,
		StartDate:       time.Date(2026, 10, 14, 0, 0, 0, 0, time.UTC),
		EndDate:         time.Date(2026, 10, 25, 0, 0, 0, 0, time.UTC),
		WeekCycleLength: 1,
		IsActive:        true,
	}
	require.NoError(t, s.factory.CalendarPeriod.CreatePeriod(s.ctx, holiday))
	s.extraCleanups = append(s.extraCleanups, func() {
		testpkg.CleanupTableRecords(t, s.db, "schedule.calendar_periods", holiday.ID)
	})

	// Pin the schedule to the holiday period. The materialize window
	// (April 20+) is outside the holiday period → no instance should be
	// created.
	_, err := s.db.NewUpdate().
		Model((*activitiesModels.Schedule)(nil)).
		ModelTableExpr(`activities.schedules`).
		Set("calendar_period_id = ?", holiday.ID).
		Where("id = ?", s.schedule.ID).
		Where("tenant_id = ?", 1).
		Exec(s.ctx)
	require.NoError(t, err)

	r, err := s.svc.MaterializeForTenant(s.ctx, materializeDate, materializeDate.AddDate(0, 0, 6), scheduleSvc.MaterializationSourceManual)
	require.NoError(t, err)
	assert.Zero(t, r.InstancesCreated)
	assert.Equal(t, 1, r.CandidatesSkippedNoPeriod,
		"schedule pinned to a period outside the window must be skipped")
}

func TestMaterializeForTenant_ABWeekSmoke(t *testing.T) {
	materializeDate := time.Date(2026, time.April, 20, 0, 0, 0, 0, time.UTC) // Mon, "Week A"
	s := makeScenario(t, activitiesModels.WeekdayMonday, materializeDate)
	defer s.runCleanup(t)

	// Flip the period to A/B cycle with anchor = materializeDate. Anchor week
	// resolves to Week A (pattern 1).
	anchor := materializeDate
	_, err := s.db.NewUpdate().
		Model((*scheduleModels.CalendarPeriod)(nil)).
		ModelTableExpr(`schedule.calendar_periods AS "calendar_period"`).
		Set("week_cycle_length = ?", 2).
		Set("week_cycle_anchor = ?", anchor).
		Where(`"calendar_period".id = ?`, s.period.ID).
		Where("tenant_id = ?", 1).
		Exec(s.ctx)
	require.NoError(t, err)

	// Existing schedule has week_pattern=0 (every week) — set it to 2 (Week B).
	_, err = s.db.NewUpdate().
		Model((*activitiesModels.Schedule)(nil)).
		ModelTableExpr(`activities.schedules`).
		Set("week_pattern = ?", 2).
		Where("id = ?", s.schedule.ID).
		Where("tenant_id = ?", 1).
		Exec(s.ctx)
	require.NoError(t, err)

	// With schedule as Week B and anchor week as Week A, materializeDate must
	// be skipped by the A/B filter.
	r, err := s.svc.MaterializeForTenant(s.ctx, materializeDate, materializeDate.AddDate(0, 0, 6), scheduleSvc.MaterializationSourceManual)
	require.NoError(t, err)
	assert.Zero(t, r.InstancesCreated, "Week-B schedule should not materialize in anchor week (Week A)")
	assert.Equal(t, 1, r.CandidatesSkippedABWeek)

	// Next week is Week B → same schedule should now materialize.
	nextMon := materializeDate.AddDate(0, 0, 7)
	r2, err := s.svc.MaterializeForTenant(s.ctx, nextMon, nextMon.AddDate(0, 0, 6), scheduleSvc.MaterializationSourceManual)
	require.NoError(t, err)
	assert.Equal(t, 1, r2.InstancesCreated, "Week-B schedule must materialize in a Week-B calendar week")

	rows := listInstancesForDate(t, s.db, s.template.ID, nextMon)
	if len(rows) > 0 {
		s.registerCleanup("schedule.activity_instances", rows[0].ID)
	}
}

// -----------------------------------------------------------------------------
// Helpers
// -----------------------------------------------------------------------------

func listInstancesForDate(tb testing.TB, db *bun.DB, templateID int64, date time.Time) []*scheduleModels.ActivityInstance {
	tb.Helper()
	var rows []*scheduleModels.ActivityInstance
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err := db.NewSelect().
		Model(&rows).
		ModelTableExpr(`schedule.activity_instances AS "activity_instance"`).
		Where(`"activity_instance".activity_group_id = ?`, templateID).
		Where(`"activity_instance".date = ?`, date).
		Order("start_time ASC").
		Scan(ctx)
	require.NoError(tb, err)
	return rows
}
