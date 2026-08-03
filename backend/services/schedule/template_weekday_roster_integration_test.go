package schedule_test

import (
	"context"
	"fmt"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activitiesModels "github.com/moto-nrw/project-phoenix/models/activities"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/services"
	scheduleSvc "github.com/moto-nrw/project-phoenix/services/schedule"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// Issue #2129: one Regeltermin that runs on several weekdays must be able to
// carry a different responsible person, different additional staff, and a
// different child list per weekday — without the school modelling five
// near-identical series.
//
// The tests below drive the real write path (TimetableData.CreateTemplate /
// UpdateTemplate) and then materialize, so what they assert is what a planner
// actually sees on the grid.

// weekdayRosterScenario holds the people and rooms a Mo+Tu template can be
// built from. Hermetic: unique tenant, real rows, full teardown.
type weekdayRosterScenario struct {
	db          *bun.DB
	ctx         context.Context
	tenantID    int64
	factory     *services.Factory
	materialize scheduleSvc.MaterializationService

	roomID     int64
	categoryID int64
	staffA     int64
	staffB     int64
	studentA   int64
	studentB   int64

	periodID    int64
	templateIDs []int64
	cleanup     []func()
}

func TestCreateTemplate_PerWeekdayRoster_MaterializesTheWeekdaysOwnPeople(t *testing.T) {
	monday := timezone.NewDate(2026, time.April, 20)
	tuesday := monday.AddDays(1)

	s := makeWeekdayRosterScenario(t, monday)
	defer s.teardown(t)

	// Monday: Anna with Alice. Tuesday: Bea with Bob. Nothing shared.
	result, err := s.factory.TimetableData.CreateTemplate(s.ctx, scheduleSvc.CreateTemplateInput{
		Name:            fmt.Sprintf("Lernzeit-%d", time.Now().UnixNano()),
		Type:            activitiesModels.GroupTypeCare,
		Weekdays:        []int{activitiesModels.WeekdayMonday, activitiesModels.WeekdayTuesday},
		StartTime:       clockTime(14, 0),
		EndTime:         clockTime(15, 0),
		RoomID:          s.roomID,
		CategoryID:      s.categoryID,
		MaxParticipants: 20,
		RosterValidFrom: monday.AddDays(-30),
		GradeLevelMax:   4,
		WeekdayAssignments: []scheduleSvc.WeekdayRosterAssignment{
			{
				Weekday:        activitiesModels.WeekdayMonday,
				StaffIDs:       []int64{s.staffA},
				PrimaryStaffID: &s.staffA,
				StudentIDs:     []int64{s.studentA},
			},
			{
				Weekday:        activitiesModels.WeekdayTuesday,
				StaffIDs:       []int64{s.staffB},
				PrimaryStaffID: &s.staffB,
				StudentIDs:     []int64{s.studentB},
			},
		},
	})
	require.NoError(t, err)
	s.registerTemplate(t, result.TemplateID, result.TimeframeID)

	materialized, err := s.materialize.MaterializeForTenant(s.ctx, monday, tuesday, scheduleSvc.MaterializationSourceManual)
	require.NoError(t, err)
	require.Equal(t, 2, materialized.InstancesCreated, "one occurrence per weekday")

	mondayInstance := s.singleInstance(t, result.TemplateID, monday)
	tuesdayInstance := s.singleInstance(t, result.TemplateID, tuesday)

	assert.Equal(t, []int64{s.staffA}, s.instanceStaffIDs(t, mondayInstance),
		"Monday must be staffed by Monday's person only")
	assert.Equal(t, []int64{s.staffB}, s.instanceStaffIDs(t, tuesdayInstance),
		"Tuesday must be staffed by Tuesday's person only")
	assert.Equal(t, []int64{s.studentA}, s.instanceStudentIDs(t, mondayInstance),
		"Monday's child list must not leak into Tuesday")
	assert.Equal(t, []int64{s.studentB}, s.instanceStudentIDs(t, tuesdayInstance))

	assert.True(t, s.instanceHasPrimary(t, mondayInstance, s.staffA),
		"each weekday keeps its own zuständige Person")
	assert.True(t, s.instanceHasPrimary(t, tuesdayInstance, s.staffB),
		"the Tuesday primary must not be cleared by the Monday primary")
}

// A weekday without its own entry falls back to the series' shared roster.
// That is the "gemeinsamer Standard + Abweichung" shape the planner UI edits.
func TestCreateTemplate_PerWeekdayRoster_UnlistedWeekdayKeepsSharedDefault(t *testing.T) {
	monday := timezone.NewDate(2026, time.April, 20)
	tuesday := monday.AddDays(1)

	s := makeWeekdayRosterScenario(t, monday)
	defer s.teardown(t)

	result, err := s.factory.TimetableData.CreateTemplate(s.ctx, scheduleSvc.CreateTemplateInput{
		Name:            fmt.Sprintf("Randstunde-%d", time.Now().UnixNano()),
		Type:            activitiesModels.GroupTypeCare,
		Weekdays:        []int{activitiesModels.WeekdayMonday, activitiesModels.WeekdayTuesday},
		StartTime:       clockTime(14, 0),
		EndTime:         clockTime(15, 0),
		RoomID:          s.roomID,
		CategoryID:      s.categoryID,
		MaxParticipants: 20,
		RosterValidFrom: monday.AddDays(-30),
		GradeLevelMax:   4,
		// Shared default: Anna with Alice …
		StaffIDs:       []int64{s.staffA},
		PrimaryStaffID: &s.staffA,
		StudentIDs:     []int64{s.studentA},
		// … deviating only on Tuesday.
		WeekdayAssignments: []scheduleSvc.WeekdayRosterAssignment{{
			Weekday:        activitiesModels.WeekdayTuesday,
			StaffIDs:       []int64{s.staffB},
			PrimaryStaffID: &s.staffB,
			StudentIDs:     []int64{s.studentA, s.studentB},
		}},
	})
	require.NoError(t, err)
	s.registerTemplate(t, result.TemplateID, result.TimeframeID)

	_, err = s.materialize.MaterializeForTenant(s.ctx, monday, tuesday, scheduleSvc.MaterializationSourceManual)
	require.NoError(t, err)

	mondayInstance := s.singleInstance(t, result.TemplateID, monday)
	tuesdayInstance := s.singleInstance(t, result.TemplateID, tuesday)

	assert.Equal(t, []int64{s.staffA}, s.instanceStaffIDs(t, mondayInstance),
		"Monday keeps the shared default")
	assert.Equal(t, []int64{s.studentA}, s.instanceStudentIDs(t, mondayInstance))
	assert.Equal(t, []int64{s.staffB}, s.instanceStaffIDs(t, tuesdayInstance),
		"Tuesday's deviation replaces the shared roster, it does not add to it")
	assert.ElementsMatch(t, []int64{s.studentA, s.studentB}, s.instanceStudentIDs(t, tuesdayInstance))
}

// A template written without any weekday assignment must behave exactly as it
// did before #2129: one roster, every weekday. This is the compatibility
// guarantee for every Regeltermin that already exists.
func TestCreateTemplate_WithoutWeekdayAssignments_KeepsOneSharedRoster(t *testing.T) {
	monday := timezone.NewDate(2026, time.April, 20)
	tuesday := monday.AddDays(1)

	s := makeWeekdayRosterScenario(t, monday)
	defer s.teardown(t)

	result, err := s.factory.TimetableData.CreateTemplate(s.ctx, scheduleSvc.CreateTemplateInput{
		Name:            fmt.Sprintf("Mensa-%d", time.Now().UnixNano()),
		Type:            activitiesModels.GroupTypeCare,
		Weekdays:        []int{activitiesModels.WeekdayMonday, activitiesModels.WeekdayTuesday},
		StartTime:       clockTime(14, 0),
		EndTime:         clockTime(15, 0),
		RoomID:          s.roomID,
		CategoryID:      s.categoryID,
		MaxParticipants: 20,
		RosterValidFrom: monday.AddDays(-30),
		GradeLevelMax:   4,
		StaffIDs:        []int64{s.staffA, s.staffB},
		PrimaryStaffID:  &s.staffA,
		StudentIDs:      []int64{s.studentA, s.studentB},
	})
	require.NoError(t, err)
	s.registerTemplate(t, result.TemplateID, result.TimeframeID)

	for _, row := range s.supervisorRows(t, result.TemplateID) {
		assert.Nil(t, row.Weekday, "a template without deviations stores series-wide roster rows")
	}
	for _, row := range s.enrollmentRows(t, result.TemplateID) {
		assert.Nil(t, row.Weekday)
	}

	_, err = s.materialize.MaterializeForTenant(s.ctx, monday, tuesday, scheduleSvc.MaterializationSourceManual)
	require.NoError(t, err)

	for _, date := range []timezone.Date{monday, tuesday} {
		instance := s.singleInstance(t, result.TemplateID, date)
		assert.ElementsMatch(t, []int64{s.staffA, s.staffB}, s.instanceStaffIDs(t, instance))
		assert.ElementsMatch(t, []int64{s.studentA, s.studentB}, s.instanceStudentIDs(t, instance))
	}
}

// Editing the series replaces the per-weekday roster wholesale, including
// dropping back to a single shared roster when the planner turns the
// per-weekday mode off again.
func TestUpdateTemplate_SwitchesBetweenSharedAndPerWeekdayRoster(t *testing.T) {
	monday := timezone.NewDate(2026, time.April, 20)
	tuesday := monday.AddDays(1)

	s := makeWeekdayRosterScenario(t, monday)
	defer s.teardown(t)

	name := fmt.Sprintf("Lernzeit-Update-%d", time.Now().UnixNano())
	result, err := s.factory.TimetableData.CreateTemplate(s.ctx, scheduleSvc.CreateTemplateInput{
		Name:            name,
		Type:            activitiesModels.GroupTypeCare,
		Weekdays:        []int{activitiesModels.WeekdayMonday, activitiesModels.WeekdayTuesday},
		StartTime:       clockTime(14, 0),
		EndTime:         clockTime(15, 0),
		RoomID:          s.roomID,
		CategoryID:      s.categoryID,
		MaxParticipants: 20,
		RosterValidFrom: monday.AddDays(-30),
		GradeLevelMax:   4,
		StaffIDs:        []int64{s.staffA},
		PrimaryStaffID:  &s.staffA,
		StudentIDs:      []int64{s.studentA},
	})
	require.NoError(t, err)
	s.registerTemplate(t, result.TemplateID, result.TimeframeID)

	updateInput := scheduleSvc.TemplateUpdateInput{
		TemplateID: result.TemplateID,
		Fields: activitiesModels.TemplateFieldsUpdate{
			Name:            name,
			Type:            activitiesModels.GroupTypeCare,
			CategoryID:      s.categoryID,
			RoomID:          s.roomID,
			MaxParticipants: 20,
			TargetGroupType: activitiesModels.TargetGroupTypeNone,
		},
		Weekdays:        []int{activitiesModels.WeekdayMonday, activitiesModels.WeekdayTuesday},
		TimeframeID:     result.TimeframeID,
		RosterValidFrom: monday.AddDays(-30),
		GradeLevelMax:   4,
		StaffIDs:        []int64{s.staffA},
		PrimaryStaffID:  &s.staffA,
		StudentIDs:      []int64{s.studentA},
		WeekdayAssignments: []scheduleSvc.WeekdayRosterAssignment{{
			Weekday:        activitiesModels.WeekdayTuesday,
			StaffIDs:       []int64{s.staffB},
			PrimaryStaffID: &s.staffB,
			StudentIDs:     []int64{s.studentB},
		}},
	}
	require.NoError(t, s.factory.TimetableData.UpdateTemplate(s.ctx, updateInput))

	_, err = s.materialize.MaterializeForTenant(s.ctx, monday, tuesday, scheduleSvc.MaterializationSourceManual)
	require.NoError(t, err)
	assert.Equal(t, []int64{s.staffA}, s.instanceStaffIDs(t, s.singleInstance(t, result.TemplateID, monday)))
	assert.Equal(t, []int64{s.staffB}, s.instanceStaffIDs(t, s.singleInstance(t, result.TemplateID, tuesday)))

	// Turn per-weekday staffing back off: every open roster row loses its scope.
	updateInput.WeekdayAssignments = nil
	updateInput.StaffIDs = []int64{s.staffA, s.staffB}
	require.NoError(t, s.factory.TimetableData.UpdateTemplate(s.ctx, updateInput))
	for _, row := range s.openSupervisorRows(t, result.TemplateID) {
		assert.Nil(t, row.Weekday, "resetting to a shared roster must clear the weekday scope")
	}
}

// A weekday the series does not run on is rejected rather than silently
// dropped or silently added to the recurrence.
func TestCreateTemplate_WeekdayAssignmentOutsideRecurrence_IsRejected(t *testing.T) {
	monday := timezone.NewDate(2026, time.April, 20)
	s := makeWeekdayRosterScenario(t, monday)
	defer s.teardown(t)

	_, err := s.factory.TimetableData.CreateTemplate(s.ctx, scheduleSvc.CreateTemplateInput{
		Name:            fmt.Sprintf("Invalid-%d", time.Now().UnixNano()),
		Type:            activitiesModels.GroupTypeCare,
		Weekdays:        []int{activitiesModels.WeekdayMonday},
		StartTime:       clockTime(14, 0),
		EndTime:         clockTime(15, 0),
		RoomID:          s.roomID,
		CategoryID:      s.categoryID,
		MaxParticipants: 20,
		RosterValidFrom: monday.AddDays(-30),
		GradeLevelMax:   4,
		WeekdayAssignments: []scheduleSvc.WeekdayRosterAssignment{{
			Weekday:  activitiesModels.WeekdayWednesday,
			StaffIDs: []int64{s.staffA},
		}},
	})
	require.ErrorIs(t, err, scheduleSvc.ErrWeekdayAssignmentUnscheduled)
}

func TestTemplateWeekdayRosterRead_PreservesEmptyDaysAndProtectedEnrollments(t *testing.T) {
	monday := timezone.NewDate(2026, time.April, 20)
	s := makeWeekdayRosterScenario(t, monday)
	defer s.teardown(t)

	result, err := s.factory.TimetableData.CreateTemplate(s.ctx, scheduleSvc.CreateTemplateInput{
		Name:            fmt.Sprintf("Read-Roster-%d", time.Now().UnixNano()),
		Type:            activitiesModels.GroupTypeCare,
		Weekdays:        []int{activitiesModels.WeekdayMonday, activitiesModels.WeekdayTuesday},
		StartTime:       clockTime(14, 0),
		EndTime:         clockTime(15, 0),
		RoomID:          s.roomID,
		CategoryID:      s.categoryID,
		MaxParticipants: 20,
		RosterValidFrom: monday.AddDays(-30),
		GradeLevelMax:   4,
		WeekdayAssignments: []scheduleSvc.WeekdayRosterAssignment{
			{Weekday: activitiesModels.WeekdayMonday},
			{
				Weekday:    activitiesModels.WeekdayTuesday,
				StaffIDs:   []int64{s.staffB},
				StudentIDs: []int64{s.studentB},
			},
		},
	})
	require.NoError(t, err)
	s.registerTemplate(t, result.TemplateID, result.TimeframeID)

	protected := &activitiesModels.StudentEnrollment{
		StudentID:        s.studentA,
		ActivityGroupID:  result.TemplateID,
		ValidFrom:        monday.AddDays(-30),
		SelectedWeekdays: []int{activitiesModels.WeekdayMonday},
	}
	require.NoError(t, repositories.NewFactory(s.db).StudentEnrollment.Create(s.ctx, protected))

	rows, err := repositories.NewFactory(s.db).ActivityGroup.ListTemplateWeekdayRoster(s.ctx, &result.TemplateID)
	require.NoError(t, err)

	type rosterKey struct {
		weekday int
		kind    string
		person  int64
	}
	found := make(map[rosterKey]bool, len(rows))
	for _, row := range rows {
		found[rosterKey{weekday: row.Weekday, kind: row.Kind, person: row.PersonID}] = true
	}
	assert.True(t, found[rosterKey{weekday: activitiesModels.WeekdayMonday, kind: activitiesModels.TemplateWeekdayRosterKindEmpty}],
		"an intentionally empty Monday needs an API marker")
	assert.True(t, found[rosterKey{weekday: activitiesModels.WeekdayTuesday, kind: activitiesModels.TemplateWeekdayRosterKindEmpty}])
	assert.True(t, found[rosterKey{weekday: activitiesModels.WeekdayMonday, kind: activitiesModels.TemplateWeekdayRosterKindStudent, person: s.studentA}],
		"the protected child must remain visible on the weekday where it materializes")
	assert.False(t, found[rosterKey{weekday: activitiesModels.WeekdayTuesday, kind: activitiesModels.TemplateWeekdayRosterKindStudent, person: s.studentA}],
		"selected_weekdays still limits the protected child")
	assert.True(t, found[rosterKey{weekday: activitiesModels.WeekdayTuesday, kind: activitiesModels.TemplateWeekdayRosterKindStudent, person: s.studentB}])
	assert.True(t, found[rosterKey{weekday: activitiesModels.WeekdayTuesday, kind: activitiesModels.TemplateWeekdayRosterKindStaff, person: s.staffB}])
}

// -----------------------------------------------------------------------------
// Scenario plumbing
// -----------------------------------------------------------------------------

func clockTime(hour, minute int) time.Time {
	return time.Date(2000, time.January, 1, hour, minute, 0, 0, time.UTC)
}

func makeWeekdayRosterScenario(t *testing.T, anchor timezone.Date) *weekdayRosterScenario {
	t.Helper()
	db := testpkg.SetupTestDB(t)

	repoFactory := repositories.NewFactory(db)
	serviceFactory, err := services.NewFactory(repoFactory, db, slog.Default())
	require.NoError(t, err)

	tenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, tenantID)
	ctx := testpkg.TenantContext(tenantID)
	suffix := time.Now().UnixNano()

	period := &scheduleModels.CalendarPeriod{
		Name:            fmt.Sprintf("Schuljahr-%d", suffix),
		PeriodType:      scheduleModels.PeriodTypeSchoolYear,
		StartDate:       timezone.NewDate(anchor.Year-1, 8, 1),
		EndDate:         timezone.NewDate(anchor.Year+1, 7, 31),
		WeekCycleLength: 1,
		IsActive:        true,
	}
	require.NoError(t, serviceFactory.CalendarPeriod.CreatePeriod(ctx, period))

	room := testpkg.CreateTestRoomForTenant(t, db, tenantID, fmt.Sprintf("Raum-%d", suffix))
	category := testpkg.CreateTestActivityCategoryForTenant(t, db, tenantID, fmt.Sprintf("Kat-%d", suffix))
	staffA := testpkg.CreateTestStaffForTenant(t, db, tenantID, "Anna", fmt.Sprintf("Montag-%d", suffix))
	staffB := testpkg.CreateTestStaffForTenant(t, db, tenantID, "Bea", fmt.Sprintf("Dienstag-%d", suffix))
	studentA := testpkg.CreateTestStudentForTenant(t, db, tenantID, "Alice", fmt.Sprintf("Eins-%d", suffix), "3a")
	studentB := testpkg.CreateTestStudentForTenant(t, db, tenantID, "Bob", fmt.Sprintf("Zwei-%d", suffix), "3a")

	s := &weekdayRosterScenario{
		db:          db,
		ctx:         ctx,
		tenantID:    tenantID,
		factory:     serviceFactory,
		materialize: serviceFactory.Materialization,
		roomID:      room.ID,
		categoryID:  category.ID,
		staffA:      staffA.ID,
		staffB:      staffB.ID,
		studentA:    studentA.ID,
		studentB:    studentB.ID,
		periodID:    period.ID,
	}
	s.cleanup = append(s.cleanup, func() {
		testpkg.CleanupActivityFixturesForTenant(t, db, tenantID, studentA.ID, staffA.ID, 0, 0, room.ID)
		testpkg.CleanupTableRecords(t, db, "users.students", studentB.ID)
		testpkg.CleanupTableRecords(t, db, "activities.categories", category.ID)
		testpkg.CleanupTableRecords(t, db, "schedule.calendar_periods", period.ID)
	})
	return s
}

// registerTemplate schedules teardown for everything one CreateTemplate call
// produced, children first so the FKs hold.
func (s *weekdayRosterScenario) registerTemplate(t *testing.T, templateID, timeframeID int64) {
	t.Helper()
	s.templateIDs = append(s.templateIDs, templateID)
	s.cleanup = append([]func(){func() {
		instanceIDs := s.templateInstanceIDs(t, templateID)
		for _, table := range []string{"schedule.instance_students", "schedule.instance_staff"} {
			s.deleteByInstanceIDs(t, table, instanceIDs)
		}
		testpkg.CleanupTableRecords(t, s.db, "schedule.activity_instances", instanceIDs...)
		s.deleteByTemplate(t, "activities.student_enrollments", "activity_group_id", templateID)
		s.deleteByTemplate(t, "activities.supervisors", "group_id", templateID)
		s.deleteByTemplate(t, "activities.schedules", "activity_group_id", templateID)
		testpkg.CleanupTableRecords(t, s.db, "activities.groups", templateID)
		testpkg.CleanupScheduleFixtures(t, s.db, timeframeID)
	}}, s.cleanup...)
}

func (s *weekdayRosterScenario) teardown(t *testing.T) {
	t.Helper()
	for _, fn := range s.cleanup {
		fn()
	}
	_ = s.db.Close()
}

func (s *weekdayRosterScenario) deleteByTemplate(t *testing.T, table, column string, templateID int64) {
	t.Helper()
	_, err := s.db.NewRaw(
		fmt.Sprintf(`DELETE FROM %s WHERE %s = ? AND tenant_id = ?`, table, column),
		templateID, s.tenantID,
	).Exec(s.ctx)
	require.NoError(t, err)
}

func (s *weekdayRosterScenario) deleteByInstanceIDs(t *testing.T, table string, instanceIDs []int64) {
	t.Helper()
	if len(instanceIDs) == 0 {
		return
	}
	_, err := s.db.NewRaw(
		fmt.Sprintf(`DELETE FROM %s WHERE instance_id IN (?) AND tenant_id = ?`, table),
		bun.List(instanceIDs), s.tenantID,
	).Exec(s.ctx)
	require.NoError(t, err)
}

func (s *weekdayRosterScenario) templateInstanceIDs(t *testing.T, templateID int64) []int64 {
	t.Helper()
	ids := make([]int64, 0)
	err := s.db.NewRaw(
		`SELECT id FROM schedule.activity_instances WHERE activity_group_id = ? AND tenant_id = ?`,
		templateID, s.tenantID,
	).Scan(s.ctx, &ids)
	require.NoError(t, err)
	return ids
}

func (s *weekdayRosterScenario) singleInstance(t *testing.T, templateID int64, date timezone.Date) int64 {
	t.Helper()
	instances := listInstancesForDate(t, s.db, templateID, date)
	require.Len(t, instances, 1, "expected exactly one occurrence on %s", date.String())
	return instances[0].ID
}

func (s *weekdayRosterScenario) instanceStaffIDs(t *testing.T, instanceID int64) []int64 {
	t.Helper()
	ids := make([]int64, 0)
	err := s.db.NewRaw(
		`SELECT staff_id FROM schedule.instance_staff WHERE instance_id = ? AND tenant_id = ? ORDER BY staff_id`,
		instanceID, s.tenantID,
	).Scan(s.ctx, &ids)
	require.NoError(t, err)
	return ids
}

func (s *weekdayRosterScenario) instanceHasPrimary(t *testing.T, instanceID, staffID int64) bool {
	t.Helper()
	count, err := s.db.NewSelect().
		TableExpr(`schedule.instance_staff AS "instance_staff"`).
		Where(`"instance_staff".instance_id = ?`, instanceID).
		Where(`"instance_staff".staff_id = ?`, staffID).
		Where(`"instance_staff".tenant_id = ?`, s.tenantID).
		Where(`"instance_staff".is_primary`).
		Count(s.ctx)
	require.NoError(t, err)
	return count == 1
}

func (s *weekdayRosterScenario) instanceStudentIDs(t *testing.T, instanceID int64) []int64 {
	t.Helper()
	ids := make([]int64, 0)
	err := s.db.NewRaw(
		`SELECT student_id FROM schedule.instance_students WHERE instance_id = ? AND tenant_id = ? ORDER BY student_id`,
		instanceID, s.tenantID,
	).Scan(s.ctx, &ids)
	require.NoError(t, err)
	return ids
}

func (s *weekdayRosterScenario) supervisorRows(t *testing.T, templateID int64) []*activitiesModels.SupervisorPlanned {
	t.Helper()
	var rows []*activitiesModels.SupervisorPlanned
	require.NoError(t, s.db.NewSelect().Model(&rows).
		ModelTableExpr(`activities.supervisors AS "supervisor_planned"`).
		Where(`"supervisor_planned".group_id = ?`, templateID).
		Where(`"supervisor_planned".tenant_id = ?`, s.tenantID).
		Scan(s.ctx))
	return rows
}

func (s *weekdayRosterScenario) openSupervisorRows(t *testing.T, templateID int64) []*activitiesModels.SupervisorPlanned {
	t.Helper()
	open := make([]*activitiesModels.SupervisorPlanned, 0)
	for _, row := range s.supervisorRows(t, templateID) {
		if row.ValidUntil == nil {
			open = append(open, row)
		}
	}
	require.NotEmpty(t, open, "expected at least one open supervision row")
	return open
}

func (s *weekdayRosterScenario) enrollmentRows(t *testing.T, templateID int64) []*activitiesModels.StudentEnrollment {
	t.Helper()
	var rows []*activitiesModels.StudentEnrollment
	require.NoError(t, s.db.NewSelect().Model(&rows).
		ModelTableExpr(`activities.student_enrollments AS "student_enrollment"`).
		Where(`"student_enrollment".activity_group_id = ?`, templateID).
		Where(`"student_enrollment".tenant_id = ?`, s.tenantID).
		Scan(s.ctx))
	return rows
}
