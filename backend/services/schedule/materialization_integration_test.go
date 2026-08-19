package schedule_test

import (
	"context"
	"fmt"
	"log/slog"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	activitiesRepo "github.com/moto-nrw/project-phoenix/database/repositories/activities"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activitiesModels "github.com/moto-nrw/project-phoenix/models/activities"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/services"
	scheduleSvc "github.com/moto-nrw/project-phoenix/services/schedule"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// Hermetic test: fixtures are real DB rows created per subtest under a unique
// tenant and cleaned up via testpkg.Cleanup* helpers. Each subtest is
// self-contained.

// -----------------------------------------------------------------------------
// scenarioSetup bundles the minimum dependencies the materialization service
// needs plus convenient references to the common fixtures.
// -----------------------------------------------------------------------------

type scenarioSetup struct {
	svc           scheduleSvc.MaterializationService
	factory       *services.Factory
	db            *bun.DB
	ctx           context.Context
	tenantID      int64
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
// one active school-year period (no A/B cycle), one template on
// the given weekday, one timeframe 14:00–15:00, one room, one staff, three
// students of which two have valid enrollments at `materializeDate` and one
// has expired. Returns the setup; caller registers additional fixtures and
// calls runCleanup on teardown.
func makeScenario(t *testing.T, weekday int, materializeDate timezone.Date) *scenarioSetup {
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
		nil,
		slog.Default(),
	)

	tenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, tenantID)
	ctx := testpkg.TenantContext(tenantID)
	suffix := time.Now().UnixNano()

	// 1. Calendar period — no A/B, wide range, active.
	period := &scheduleModels.CalendarPeriod{
		Name:            fmt.Sprintf("Schuljahr-%d", suffix),
		PeriodType:      scheduleModels.PeriodTypeSchoolYear,
		StartDate:       timezone.NewDate(materializeDate.Year-1, 8, 1),
		EndDate:         timezone.NewDate(materializeDate.Year+1, 7, 31),
		WeekCycleLength: 1,
		IsActive:        true,
	}
	require.NoError(t, serviceFactory.CalendarPeriod.CreatePeriod(ctx, period))

	// 2. Fixtures: room, staff, 3 students.
	room := testpkg.CreateTestRoomForTenant(t, db, tenantID, fmt.Sprintf("Room-%d", suffix))
	staff := testpkg.CreateTestStaffForTenant(t, db, tenantID, "Super", fmt.Sprintf("Visor-%d", suffix))
	student1 := testpkg.CreateTestStudentForTenant(t, db, tenantID, "Alice", fmt.Sprintf("One-%d", suffix), "3a")
	student2 := testpkg.CreateTestStudentForTenant(t, db, tenantID, "Bob", fmt.Sprintf("Two-%d", suffix), "3a")
	student3 := testpkg.CreateTestStudentForTenant(t, db, tenantID, "Carol", fmt.Sprintf("Three-%d", suffix), "3a")

	// 3. Template activity group (is_template = true, planned_room_id set).
	category := testpkg.CreateTestActivityCategoryForTenant(t, db, tenantID, fmt.Sprintf("Cat-%d", suffix))
	template := &activitiesModels.Group{
		Name:            fmt.Sprintf("Malen-AG-%d", suffix),
		MaxParticipants: 20,
		IsOpen:          true,
		CategoryID:      category.ID,
		CreatedBy:       &staff.ID,
		PlannedRoomID:   &room.ID,
		IsTemplate:      true,
	}
	template.SetTenantID(tenantID)
	_, err = db.NewInsert().Model(template).ModelTableExpr(`activities.groups AS "group"`).Exec(ctx)
	require.NoError(t, err)

	// 4. Timeframe 14:00–15:00 today (the wall-clock date is irrelevant for
	// TIME columns; bun strips to TIME on write).
	timeframe := testpkg.CreateTestTimeframeForTenant(t, db, tenantID, fmt.Sprintf("Nachmittag-%d", suffix))

	// 5. Schedule: weekday + timeframe, no A/B pattern.
	sched := &activitiesModels.Schedule{
		Weekday:         weekday,
		TimeframeID:     &timeframe.ID,
		ActivityGroupID: template.ID,
		WeekPattern:     0,
	}
	sched.SetTenantID(tenantID)
	_, err = db.NewInsert().Model(sched).ModelTableExpr(`activities.schedules`).Exec(ctx)
	require.NoError(t, err)

	// 6. Enrollments: student1 + student2 valid unbounded; student3 expired
	// the day before materializeDate.
	expiredUntil := materializeDate.AddDays(-1)
	validFrom := materializeDate.AddDays(-30)
	enroll1 := &activitiesModels.StudentEnrollment{StudentID: student1.ID, ActivityGroupID: template.ID, ValidFrom: validFrom}
	enroll1.SetTenantID(tenantID)
	_, err = db.NewInsert().Model(enroll1).ModelTableExpr(`activities.student_enrollments`).ExcludeColumn("selected_weekdays").Exec(ctx)
	require.NoError(t, err)
	enroll2 := &activitiesModels.StudentEnrollment{StudentID: student2.ID, ActivityGroupID: template.ID, ValidFrom: validFrom}
	enroll2.SetTenantID(tenantID)
	_, err = db.NewInsert().Model(enroll2).ModelTableExpr(`activities.student_enrollments`).ExcludeColumn("selected_weekdays").Exec(ctx)
	require.NoError(t, err)
	enroll3 := &activitiesModels.StudentEnrollment{StudentID: student3.ID, ActivityGroupID: template.ID, ValidFrom: validFrom, ValidUntil: &expiredUntil}
	enroll3.SetTenantID(tenantID)
	_, err = db.NewInsert().Model(enroll3).ModelTableExpr(`activities.student_enrollments`).ExcludeColumn("selected_weekdays").Exec(ctx)
	require.NoError(t, err)

	// 7. Supervisor (primary, unbounded).
	sup := &activitiesModels.SupervisorPlanned{StaffID: staff.ID, GroupID: template.ID, IsPrimary: true, ValidFrom: validFrom}
	sup.SetTenantID(tenantID)
	_, err = db.NewInsert().Model(sup).ModelTableExpr(`activities.supervisors`).Exec(ctx)
	require.NoError(t, err)

	s := &scenarioSetup{
		svc:           svc,
		factory:       serviceFactory,
		db:            db,
		ctx:           ctx,
		tenantID:      tenantID,
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
		testpkg.CleanupActivityFixturesForTenant(t, db, tenantID, student1.ID, staff.ID, 0, template.ID, room.ID)
		testpkg.CleanupTableRecords(t, db, "users.students", student2.ID, student3.ID)
		testpkg.CleanupTableRecords(t, db, "activities.categories", category.ID)
		testpkg.CleanupScheduleFixtures(t, db, timeframe.ID)
		testpkg.CleanupTableRecords(t, db, "schedule.calendar_periods", period.ID)
	})
	s.cleanup = func() {}
	return s
}

// -----------------------------------------------------------------------------
// Tests
// -----------------------------------------------------------------------------

func TestMaterializeForTenant_EndToEnd(t *testing.T) {
	// Target Monday inside the wide period window; use a deterministic date
	// in the middle of a "school year" so the period bounds are obviously
	// satisfied.
	materializeDate := timezone.NewDate(2026, time.April, 20) // Mon
	s := makeScenario(t, activitiesModels.WeekdayMonday, materializeDate)
	defer s.runCleanup(t)

	from := materializeDate
	to := materializeDate.AddDays(6) // Sun

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
	// (the merge strategy skips every existing row regardless of status,
	// v1 insert-only).
	_, err = s.db.NewUpdate().
		Model((*scheduleModels.ActivityInstance)(nil)).
		ModelTableExpr(`schedule.activity_instances AS "activity_instance"`).
		Set("status = ?", scheduleModels.InstanceStatusActive).
		Where(`"activity_instance".id = ?`, instanceRows[0].ID).
		Where("tenant_id = ?", s.tenantID).
		Exec(s.ctx)
	require.NoError(t, err)
	r3, err := s.svc.MaterializeForTenant(s.ctx, from, to, scheduleSvc.MaterializationSourceManual)
	require.NoError(t, err)
	assert.Zero(t, r3.InstancesCreated)
	assert.Equal(t, 1, r3.CandidatesSkippedExisting)
}

// TestMaterializeForTenant_ExcludesGraduatedStudents covers the #405 fix:
// a graduated (alumnus) student's enrollment row survives for transition
// reverts, but future materialization must never copy them into a new
// instance_students row — otherwise upcoming cards, staffing ratios and
// slot-list exports keep counting a departed child.
func TestMaterializeForTenant_ExcludesGraduatedStudents(t *testing.T) {
	materializeDate := timezone.NewDate(2026, time.April, 20) // Mon
	s := makeScenario(t, activitiesModels.WeekdayMonday, materializeDate)
	defer s.runCleanup(t)

	// Graduate one of the two students with a valid enrollment. Their enrollment
	// row is intentionally left in place (soft delete).
	_, err := s.db.NewUpdate().
		TableExpr(`users.students`).
		Set("status = ?", string(usersModels.StudentStatusAlumnus)).
		Where("id = ?", s.students[0]).
		Where("tenant_id = ?", s.tenantID).
		Exec(s.ctx)
	require.NoError(t, err)

	r, err := s.svc.MaterializeForTenant(s.ctx, materializeDate, materializeDate.AddDays(6), scheduleSvc.MaterializationSourceManual)
	require.NoError(t, err)
	assert.Equal(t, 1, r.InstancesCreated)
	assert.Equal(t, 1, r.InstanceStudentsCreated,
		"only the non-graduated student is materialized; the alumnus enrollment is skipped")

	instanceRows := listInstancesForDate(t, s.db, s.template.ID, materializeDate)
	require.Len(t, instanceRows, 1)
	s.registerCleanup("schedule.activity_instances", instanceRows[0].ID)

	// The graduated student must not appear in instance_students.
	count, err := s.db.NewSelect().
		TableExpr(`schedule.instance_students`).
		Where("instance_id = ?", instanceRows[0].ID).
		Where("student_id = ?", s.students[0]).
		Count(s.ctx)
	require.NoError(t, err)
	assert.Zero(t, count, "graduated student must not be copied into instance_students")
}

func TestMaterializeForTenant_MultipleDynamicTargetsFollowClassChanges(t *testing.T) {
	firstMonday := timezone.NewDate(2026, time.April, 20)
	s := makeScenario(t, activitiesModels.WeekdayMonday, firstMonday)
	defer s.runCleanup(t)

	laterStudent := testpkg.CreateTestStudentForTenant(
		t, s.db, s.tenantID, "Dora", fmt.Sprintf("Later-%d", time.Now().UnixNano()), "5a",
	)
	s.extraCleanups = append(s.extraCleanups, func() {
		testpkg.CleanupTableRecords(t, s.db, "users.students", laterStudent.ID)
	})

	class3a := "3a"
	class4a := "4a"
	targetRepo, ok := activitiesRepo.NewGroupRepository(s.db).(activitiesModels.GroupTargetRepository)
	require.True(t, ok)
	require.NoError(t, targetRepo.ReplaceTargets(s.ctx, s.template.ID, []*activitiesModels.GroupTarget{
		{TargetGroupType: activitiesModels.TargetGroupTypeKlasse, TargetSchoolClass: &class3a},
		{TargetGroupType: activitiesModels.TargetGroupTypeKlasse, TargetSchoolClass: &class4a},
	}))

	first, err := s.svc.MaterializeForTenant(s.ctx, firstMonday, firstMonday, scheduleSvc.MaterializationSourceManual)
	require.NoError(t, err)
	assert.Equal(t, 3, first.InstanceStudentsCreated, "explicit and dynamic memberships form a deduplicated union")
	firstInstances := listInstancesForDate(t, s.db, s.template.ID, firstMonday)
	require.Len(t, firstInstances, 1)
	s.registerCleanup("schedule.activity_instances", firstInstances[0].ID)

	_, err = s.db.NewUpdate().
		Table("users.students").
		Set("school_class = ?", class4a).
		Where("tenant_id = ?", s.tenantID).
		Where("id = ?", laterStudent.ID).
		Exec(s.ctx)
	require.NoError(t, err)

	secondMonday := firstMonday.AddDays(7)
	second, err := s.svc.MaterializeForTenant(s.ctx, secondMonday, secondMonday, scheduleSvc.MaterializationSourceManual)
	require.NoError(t, err)
	assert.Equal(t, 4, second.InstanceStudentsCreated, "future materialization uses the changed class membership")
	secondInstances := listInstancesForDate(t, s.db, s.template.ID, secondMonday)
	require.Len(t, secondInstances, 1)
	s.registerCleanup("schedule.activity_instances", secondInstances[0].ID)
}

func TestMaterializeForTenant_ExceptionCancelled_Skips(t *testing.T) {
	materializeDate := timezone.NewDate(2026, time.April, 20)
	s := makeScenario(t, activitiesModels.WeekdayMonday, materializeDate)
	defer s.runCleanup(t)

	// Insert a cancelled exception for the target date.
	exc := &scheduleModels.ActivityException{
		ActivityGroupID: s.template.ID,
		ExceptionDate:   materializeDate,
		ExceptionType:   scheduleModels.ActivityExceptionCancelled,
	}
	exc.SetTenantID(s.tenantID)
	_, err := s.db.NewInsert().Model(exc).ModelTableExpr(`schedule.activity_exceptions`).Exec(s.ctx)
	require.NoError(t, err)
	s.registerCleanup("schedule.activity_exceptions", exc.ID)

	r, err := s.svc.MaterializeForTenant(s.ctx, materializeDate, materializeDate.AddDays(6), scheduleSvc.MaterializationSourceManual)
	require.NoError(t, err)
	assert.Zero(t, r.InstancesCreated)
	assert.Equal(t, 1, r.CandidatesSkippedException)
}

func TestMaterializeForTenant_ExceptionModified_OverridesStartTime(t *testing.T) {
	materializeDate := timezone.NewDate(2026, time.April, 20)
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
	exc.SetTenantID(s.tenantID)
	_, err := s.db.NewInsert().Model(exc).ModelTableExpr(`schedule.activity_exceptions`).Exec(s.ctx)
	require.NoError(t, err)
	s.registerCleanup("schedule.activity_exceptions", exc.ID)

	r, err := s.svc.MaterializeForTenant(s.ctx, materializeDate, materializeDate.AddDays(6), scheduleSvc.MaterializationSourceManual)
	require.NoError(t, err)
	assert.Equal(t, 1, r.InstancesCreated)

	rows := listInstancesForDate(t, s.db, s.template.ID, materializeDate)
	require.Len(t, rows, 1)
	assert.Equal(t, 13, rows[0].StartTime.Hour(), "exception overrode start_time")
	s.registerCleanup("schedule.activity_instances", rows[0].ID)
}

func TestMaterializeForTenant_NoActivePeriod_ReturnsGracefully(t *testing.T) {
	db := testpkg.SetupTestDB(t)

	repoFactory := repositories.NewFactory(db)
	serviceFactory, err := services.NewFactory(repoFactory, db, slog.Default())
	require.NoError(t, err)
	svc := scheduleSvc.NewMaterializationService(
		repoFactory.ActivityGroup, repoFactory.ActivitySchedule, repoFactory.StudentEnrollment,
		repoFactory.ActivitySupervisor, repoFactory.CalendarPeriod, repoFactory.ActivityInstance,
		repoFactory.InstanceStaff, repoFactory.InstanceStudent, repoFactory.ActivityException,
		repoFactory.Timeframe, serviceFactory.CalendarPeriod, db, nil, slog.Default(),
	)

	// Use a fresh tenant id that we know has no active periods.
	//
	// tenant_id = some large unlikely value — the hermetic linter whitelists
	// the `tenant_id` key itself. We still must not use int64(1)-int64(9).
	const emptyTenantID = int64(990001)
	ctx := testpkg.TenantContext(emptyTenantID)
	from := timezone.NewDate(2026, 4, 20)
	to := from.AddDays(6)

	r, err := svc.MaterializeForTenant(ctx, from, to, scheduleSvc.MaterializationSourceManual)
	require.NoError(t, err)
	assert.Zero(t, r.InstancesCreated)
	assert.Zero(t, r.CandidatesSkippedExisting)
	assert.Zero(t, r.CandidatesSkippedNoPeriod)
	assert.Zero(t, r.CandidatesSkippedABWeek)

	// The early-return path must emit a typed warning so the UI can prompt
	// the admin instead of showing "0 angelegt" with no clue why.
	require.Len(t, r.Warnings, 1)
	assert.Equal(t, scheduleSvc.MaterializationWarningCodeNoActivePeriod, r.Warnings[0].Code)
	assert.NotEmpty(t, r.Warnings[0].Message)
}

func TestMaterializeForTenant_NoTemplates_ReturnsWarning(t *testing.T) {
	db := testpkg.SetupTestDB(t)

	repoFactory := repositories.NewFactory(db)
	serviceFactory, err := services.NewFactory(repoFactory, db, slog.Default())
	require.NoError(t, err)
	svc := scheduleSvc.NewMaterializationService(
		repoFactory.ActivityGroup, repoFactory.ActivitySchedule, repoFactory.StudentEnrollment,
		repoFactory.ActivitySupervisor, repoFactory.CalendarPeriod, repoFactory.ActivityInstance,
		repoFactory.InstanceStaff, repoFactory.InstanceStudent, repoFactory.ActivityException,
		repoFactory.Timeframe, serviceFactory.CalendarPeriod, db, nil, slog.Default(),
	)

	const emptyTemplateTenantID = int64(990002)
	ctx := testpkg.TenantContext(emptyTemplateTenantID)
	testpkg.EnsureTestTenant(t, db, emptyTemplateTenantID)
	_, err = db.ExecContext(context.Background(), `DELETE FROM schedule.calendar_periods WHERE tenant_id = ?`, emptyTemplateTenantID)
	require.NoError(t, err)
	defer func() {
		_, _ = db.ExecContext(context.Background(), `DELETE FROM schedule.calendar_periods WHERE tenant_id = ?`, emptyTemplateTenantID)
		_, _ = db.ExecContext(context.Background(), `DELETE FROM platform.schools WHERE id = ?`, emptyTemplateTenantID)
		_, _ = db.ExecContext(context.Background(), `DELETE FROM platform.organizations WHERE id = ?`, emptyTemplateTenantID)
	}()

	from := timezone.NewDate(2026, 4, 20)
	to := from.AddDays(6)
	period := &scheduleModels.CalendarPeriod{
		Name:            "No Templates Test 2025/2026",
		PeriodType:      scheduleModels.PeriodTypeSchoolYear,
		StartDate:       timezone.NewDate(2025, 8, 1),
		EndDate:         timezone.NewDate(2026, 7, 31),
		WeekCycleLength: 1,
		IsActive:        true,
	}
	require.NoError(t, serviceFactory.CalendarPeriod.CreatePeriod(ctx, period))

	r, err := svc.MaterializeForTenant(ctx, from, to, scheduleSvc.MaterializationSourceManual)
	require.NoError(t, err)
	assert.Zero(t, r.InstancesCreated)
	assert.Zero(t, r.CandidatesSkippedExisting)
	assert.Zero(t, r.CandidatesSkippedNoPeriod)
	assert.Zero(t, r.CandidatesSkippedABWeek)

	require.Len(t, r.Warnings, 1)
	assert.Equal(t, scheduleSvc.MaterializationWarningCodeNoTemplates, r.Warnings[0].Code)
	assert.NotEmpty(t, r.Warnings[0].Message)
}

func TestMaterializeForTenant_PreFetchObservesFirstRunThenSkips(t *testing.T) {
	// Verifies the "expected" half of the idempotency story the UNIQUE index
	// backstops: a first-run insert is observed by the second run's pre-fetch
	// and produces CandidatesSkippedExisting. The actual UNIQUE-violation
	// race branch (pre-fetch misses + concurrent insert wins) is exercised by
	// a unit test on isUniqueViolation — simulating it end-to-end would
	// require contrived concurrency that does not model real production.
	materializeDate := timezone.NewDate(2026, time.April, 20)
	s := makeScenario(t, activitiesModels.WeekdayMonday, materializeDate)
	defer s.runCleanup(t)

	r1, err := s.svc.MaterializeForTenant(s.ctx, materializeDate, materializeDate.AddDays(6), scheduleSvc.MaterializationSourceManual)
	require.NoError(t, err)
	require.Equal(t, 1, r1.InstancesCreated)

	rows := listInstancesForDate(t, s.db, s.template.ID, materializeDate)
	require.Len(t, rows, 1)
	s.registerCleanup("schedule.activity_instances", rows[0].ID)

	r2, err := s.svc.MaterializeForTenant(s.ctx, materializeDate, materializeDate.AddDays(6), scheduleSvc.MaterializationSourceManual)
	require.NoError(t, err)
	assert.Zero(t, r2.InstancesCreated)
	assert.Equal(t, 1, r2.CandidatesSkippedExisting)
	assert.Zero(t, r2.CandidatesRaced)
}

func TestMaterializeForTenant_TemplateScheduleBoundToPeriod_OutOfRange_Skips(t *testing.T) {
	materializeDate := timezone.NewDate(2026, time.April, 20)
	s := makeScenario(t, activitiesModels.WeekdayMonday, materializeDate)
	defer s.runCleanup(t)

	// Create a holiday period that is active but only covers Oct 2026.
	holiday := &scheduleModels.CalendarPeriod{
		Name:            fmt.Sprintf("Herbstferien-%d", time.Now().UnixNano()),
		PeriodType:      scheduleModels.PeriodTypeHoliday,
		StartDate:       timezone.NewDate(2026, 10, 14),
		EndDate:         timezone.NewDate(2026, 10, 25),
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
		Where("tenant_id = ?", s.tenantID).
		Exec(s.ctx)
	require.NoError(t, err)

	r, err := s.svc.MaterializeForTenant(s.ctx, materializeDate, materializeDate.AddDays(6), scheduleSvc.MaterializationSourceManual)
	require.NoError(t, err)
	assert.Zero(t, r.InstancesCreated)
	assert.Equal(t, 1, r.CandidatesSkippedNoPeriod,
		"schedule pinned to a period outside the window must be skipped")
}

func TestMaterializeForTenant_ABWeekSmoke(t *testing.T) {
	materializeDate := timezone.NewDate(2026, time.April, 20) // Mon, "Week A"
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
		Where("tenant_id = ?", s.tenantID).
		Exec(s.ctx)
	require.NoError(t, err)

	// Existing schedule has week_pattern=0 (every week) — set it to 2 (Week B).
	_, err = s.db.NewUpdate().
		Model((*activitiesModels.Schedule)(nil)).
		ModelTableExpr(`activities.schedules`).
		Set("week_pattern = ?", 2).
		Where("id = ?", s.schedule.ID).
		Where("tenant_id = ?", s.tenantID).
		Exec(s.ctx)
	require.NoError(t, err)

	// With schedule as Week B and anchor week as Week A, materializeDate must
	// be skipped by the A/B filter.
	r, err := s.svc.MaterializeForTenant(s.ctx, materializeDate, materializeDate.AddDays(6), scheduleSvc.MaterializationSourceManual)
	require.NoError(t, err)
	assert.Zero(t, r.InstancesCreated, "Week-B schedule should not materialize in anchor week (Week A)")
	assert.Equal(t, 1, r.CandidatesSkippedABWeek)

	// Next week is Week B → same schedule should now materialize.
	nextMon := materializeDate.AddDays(7)
	r2, err := s.svc.MaterializeForTenant(s.ctx, nextMon, nextMon.AddDays(6), scheduleSvc.MaterializationSourceManual)
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

func listInstancesForDate(tb testing.TB, db *bun.DB, templateID int64, date timezone.Date) []*scheduleModels.ActivityInstance {
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

// -----------------------------------------------------------------------------
// WP-B3: schedules capped via valid_until stop producing instances at the cap.
// -----------------------------------------------------------------------------

func TestMaterializeForTenant_ScheduleValidUntil_SkipsEndedDates(t *testing.T) {
	firstMonday := timezone.NewDate(2026, time.April, 20) // Mon
	secondMonday := firstMonday.AddDays(7)
	s := makeScenario(t, activitiesModels.WeekdayMonday, firstMonday)
	defer s.runCleanup(t)

	// Baseline: valid_until is NULL — the second Monday materializes normally
	// and nothing is counted as ended.
	r0, err := s.svc.MaterializeForTenant(s.ctx, secondMonday, secondMonday, scheduleSvc.MaterializationSourceManual)
	require.NoError(t, err)
	assert.Equal(t, 1, r0.InstancesCreated, "open-ended schedule materializes unchanged")
	assert.Zero(t, r0.CandidatesSkippedEnded)
	for _, row := range listInstancesForDate(t, s.db, s.template.ID, secondMonday) {
		s.registerCleanup("schedule.activity_instances", row.ID)
	}

	// Cap the schedule between the two Mondays (exclusive end): the first
	// Monday is before the cap and still materializes, the second is ended.
	capDate := firstMonday.AddDays(1) // Tue
	_, err = s.db.NewUpdate().
		Model((*activitiesModels.Schedule)(nil)).
		ModelTableExpr(`activities.schedules`).
		Set("valid_until = ?", capDate).
		Where("id = ?", s.schedule.ID).
		Where("tenant_id = ?", s.tenantID).
		Exec(s.ctx)
	require.NoError(t, err)

	r, err := s.svc.MaterializeForTenant(s.ctx, firstMonday, secondMonday.AddDays(6), scheduleSvc.MaterializationSourceManual)
	require.NoError(t, err)
	assert.Equal(t, 1, r.InstancesCreated, "only the Monday before valid_until materializes")
	assert.Equal(t, 1, r.CandidatesSkippedEnded, "the Monday on/after valid_until counts as ended")
	assert.Zero(t, r.CandidatesSkippedExisting,
		"the ended check fires before the existing-row check, so the pre-existing second-Monday row is not double-counted")

	created := listInstancesForDate(t, s.db, s.template.ID, firstMonday)
	require.Len(t, created, 1)
	s.registerCleanup("schedule.activity_instances", created[0].ID)
}

// -----------------------------------------------------------------------------
// WP-B3 follow-up: schedules with valid_from never materialize before that
// date — the successor of a template split must not produce phantom instances
// next to the old template's rows when a window starts before the split point.
// -----------------------------------------------------------------------------

func TestMaterializeForTenant_ScheduleValidFrom_SkipsNotStartedDates(t *testing.T) {
	firstMonday := timezone.NewDate(2026, time.April, 20) // Mon
	secondMonday := firstMonday.AddDays(7)
	s := makeScenario(t, activitiesModels.WeekdayMonday, firstMonday)
	defer s.runCleanup(t)

	// Start the schedule on the second Monday (inclusive): the first Monday
	// is before valid_from and must be skipped, the second materializes.
	_, err := s.db.NewUpdate().
		Model((*activitiesModels.Schedule)(nil)).
		ModelTableExpr(`activities.schedules`).
		Set("valid_from = ?", secondMonday).
		Where("id = ?", s.schedule.ID).
		Where("tenant_id = ?", s.tenantID).
		Exec(s.ctx)
	require.NoError(t, err)

	r, err := s.svc.MaterializeForTenant(s.ctx, firstMonday, secondMonday.AddDays(6), scheduleSvc.MaterializationSourceManual)
	require.NoError(t, err)
	assert.Equal(t, 1, r.InstancesCreated, "only the Monday on/after valid_from materializes")
	assert.Equal(t, 1, r.CandidatesSkippedNotStarted, "the Monday before valid_from counts as not started")
	assert.Zero(t, r.CandidatesSkippedEnded)

	assert.Empty(t, listInstancesForDate(t, s.db, s.template.ID, firstMonday),
		"no phantom instance before valid_from")
	created := listInstancesForDate(t, s.db, s.template.ID, secondMonday)
	require.Len(t, created, 1, "valid_from is inclusive")
	s.registerCleanup("schedule.activity_instances", created[0].ID)
}
