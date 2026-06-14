// Hermetic integration tests for the WP-B3 template split service
// ("Dieser und alle folgenden").
//
// Covers:
//   - Happy path: old schedules + rosters capped at the effective date, the
//     previously-active roster carries over, planned future instances of the
//     old template are deleted while started/cancelled/spontaneous/foreign
//     instances survive, the successor is materialized with its roster.
//   - Explicit new roster + week_pattern pass-through.
//   - Validation: past effective_date, non-template id, unknown id.
//
// Fixtures via makeScenario (materialization_integration_test.go) — unique
// tenant per test, cleanup via scenarioSetup.runCleanup. No hardcoded IDs.
package schedule_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activitiesModels "github.com/moto-nrw/project-phoenix/models/activities"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	scheduleSvc "github.com/moto-nrw/project-phoenix/services/schedule"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// futureMonday returns the first Monday strictly after today, shifted by
// offsetWeeks additional weeks. The split validates effective_date >= today,
// so all split tests anchor on future dates.
func futureMonday(offsetWeeks int) timezone.Date {
	d := timezone.TodayDate().AddDays(1)
	for d.Weekday() != time.Monday {
		d = d.AddDays(1)
	}
	return d.AddDays(7 * offsetWeeks)
}

// splitInsertInstance inserts a raw schedule.activity_instances row for the
// given (possibly nil) activity group at a distinct start hour and registers
// it for cleanup.
func splitInsertInstance(t *testing.T, s *scenarioSetup, groupID *int64, date timezone.Date, status string, spontaneous bool, startHour int) int64 {
	t.Helper()
	row := &scheduleModels.ActivityInstance{
		Date:            date,
		Title:           fmt.Sprintf("Split-%s-%d", status, time.Now().UnixNano()),
		StartTime:       time.Date(1, 1, 1, startHour, 0, 0, 0, time.UTC),
		EndTime:         time.Date(1, 1, 1, startHour+1, 0, 0, 0, time.UTC),
		RoomID:          s.roomID,
		Status:          status,
		IsSpontaneous:   spontaneous,
		ActivityGroupID: groupID,
	}
	row.SetTenantID(s.tenantID)
	_, err := s.db.NewInsert().Model(row).ModelTableExpr(`schedule.activity_instances`).Exec(s.ctx)
	require.NoError(t, err)
	s.registerCleanup("schedule.activity_instances", row.ID)
	return row.ID
}

func splitInstanceExists(t *testing.T, s *scenarioSetup, id int64) bool {
	t.Helper()
	var count int
	err := s.db.NewSelect().
		TableExpr(`schedule.activity_instances AS "ai"`).
		ColumnExpr("COUNT(*)").
		Where(`"ai".id = ?`, id).
		Where(`"ai".tenant_id = ?`, s.tenantID).
		Scan(s.ctx, &count)
	require.NoError(t, err)
	return count > 0
}

// reloadSplitSchedule fetches one activities.schedules row by id.
func reloadSplitSchedule(t *testing.T, s *scenarioSetup, id int64) *activitiesModels.Schedule {
	t.Helper()
	var row activitiesModels.Schedule
	err := s.db.NewSelect().Model(&row).
		ModelTableExpr(`activities.schedules AS "schedule"`).
		Where(`"schedule".id = ?`, id).
		Where(`"schedule".tenant_id = ?`, s.tenantID).
		Scan(s.ctx)
	require.NoError(t, err)
	return &row
}

func loadSplitSchedules(t *testing.T, s *scenarioSetup, groupID int64) []*activitiesModels.Schedule {
	t.Helper()
	var rows []*activitiesModels.Schedule
	err := s.db.NewSelect().Model(&rows).
		ModelTableExpr(`activities.schedules AS "schedule"`).
		Where(`"schedule".activity_group_id = ?`, groupID).
		Where(`"schedule".tenant_id = ?`, s.tenantID).
		Order("weekday ASC").
		Scan(s.ctx)
	require.NoError(t, err)
	return rows
}

func loadSplitEnrollments(t *testing.T, s *scenarioSetup, groupID int64) []*activitiesModels.StudentEnrollment {
	t.Helper()
	var rows []*activitiesModels.StudentEnrollment
	err := s.db.NewSelect().Model(&rows).
		ModelTableExpr(`activities.student_enrollments AS "student_enrollment"`).
		Where(`"student_enrollment".activity_group_id = ?`, groupID).
		Where(`"student_enrollment".tenant_id = ?`, s.tenantID).
		Order("student_id ASC").
		Scan(s.ctx)
	require.NoError(t, err)
	return rows
}

func loadSplitSupervisors(t *testing.T, s *scenarioSetup, groupID int64) []*activitiesModels.SupervisorPlanned {
	t.Helper()
	var rows []*activitiesModels.SupervisorPlanned
	err := s.db.NewSelect().Model(&rows).
		ModelTableExpr(`activities.supervisors AS "supervisor_planned"`).
		Where(`"supervisor_planned".group_id = ?`, groupID).
		Where(`"supervisor_planned".tenant_id = ?`, s.tenantID).
		Order("staff_id ASC").
		Scan(s.ctx)
	require.NoError(t, err)
	return rows
}

// registerSuccessorCleanup registers everything the split created for the
// successor template: schedules, roster rows, instances (with cascading
// children), and — via a PREPENDED extra cleanup so it runs before the
// scenario's category/room teardown — the group row and its timeframe.
func registerSuccessorCleanup(t *testing.T, s *scenarioSetup, newGroupID int64) {
	t.Helper()
	schedules := loadSplitSchedules(t, s, newGroupID)
	var timeframeIDs []int64
	for _, sch := range schedules {
		s.registerCleanup("activities.schedules", sch.ID)
		if sch.TimeframeID != nil {
			timeframeIDs = append(timeframeIDs, *sch.TimeframeID)
		}
	}
	for _, e := range loadSplitEnrollments(t, s, newGroupID) {
		s.registerCleanup("activities.student_enrollments", e.ID)
	}
	for _, sp := range loadSplitSupervisors(t, s, newGroupID) {
		s.registerCleanup("activities.supervisors", sp.ID)
	}
	s.extraCleanups = append([]func(){func() {
		testpkg.CleanupTableRecords(t, s.db, "activities.groups", newGroupID)
		if len(timeframeIDs) > 0 {
			testpkg.CleanupTableRecords(t, s.db, "schedule.timeframes", timeframeIDs...)
		}
	}}, s.extraCleanups...)
}

func baseSplitInput(s *scenarioSetup, effective timezone.Date, name string) scheduleSvc.TemplateSplitInput {
	return scheduleSvc.TemplateSplitInput{
		TemplateID:    s.template.ID,
		EffectiveDate: effective,
		Name:          name,
		Type:          activitiesModels.GroupTypeActivity,
		Weekdays:      []int{activitiesModels.WeekdayMonday},
		StartTime:     time.Date(2000, 1, 1, 15, 0, 0, 0, time.UTC),
		EndTime:       time.Date(2000, 1, 1, 16, 0, 0, 0, time.UTC),
		RoomID:        s.roomID,
		CategoryID:    s.categoryID,
	}
}

func TestTemplateSplit_HappyPath_CarriesRosterAndProtectsHistory(t *testing.T) {
	effective := futureMonday(1)
	secondMonday := effective.AddDays(7)
	s := makeScenario(t, activitiesModels.WeekdayMonday, effective)
	defer s.runCleanup(t)
	suffix := time.Now().UnixNano()

	// --- Arrange: materialize the old template for both Mondays -----------
	r0, err := s.svc.MaterializeForTenant(s.ctx, effective, secondMonday, scheduleSvc.MaterializationSourceManual)
	require.NoError(t, err)
	require.Equal(t, 2, r0.InstancesCreated)

	first := listInstancesForDate(t, s.db, s.template.ID, effective)
	require.Len(t, first, 1)
	second := listInstancesForDate(t, s.db, s.template.ID, secondMonday)
	require.Len(t, second, 1)
	s.registerCleanup("schedule.activity_instances", first[0].ID, second[0].ID)

	// First Monday's instance has started — history that must survive.
	_, err = s.db.NewUpdate().
		Model((*scheduleModels.ActivityInstance)(nil)).
		ModelTableExpr(`schedule.activity_instances AS "activity_instance"`).
		Set("status = ?", scheduleModels.InstanceStatusActive).
		Where(`"activity_instance".id = ?`, first[0].ID).
		Where(`"activity_instance".tenant_id = ?`, s.tenantID).
		Exec(s.ctx)
	require.NoError(t, err)

	// Protected kinds inside the horizon (mirrors flow_d): completed,
	// cancelled, spontaneous planned — all on the OLD template — plus a
	// planned instance of a DIFFERENT group.
	completedID := splitInsertInstance(t, s, &s.template.ID, effective, scheduleModels.InstanceStatusCompleted, false, 9)
	cancelledID := splitInsertInstance(t, s, &s.template.ID, effective, scheduleModels.InstanceStatusCancelled, false, 10)
	spontaneousID := splitInsertInstance(t, s, &s.template.ID, effective, scheduleModels.InstanceStatusPlanned, true, 11)

	roomID := s.roomID
	otherGroup := &activitiesModels.Group{
		Name:            fmt.Sprintf("Split-Fremd-%d", suffix),
		MaxParticipants: 10,
		CategoryID:      s.categoryID,
		PlannedRoomID:   &roomID,
		IsTemplate:      true,
	}
	otherGroup.SetTenantID(s.tenantID)
	_, err = s.db.NewInsert().Model(otherGroup).ModelTableExpr(`activities.groups AS "group"`).Exec(s.ctx)
	require.NoError(t, err)
	s.extraCleanups = append([]func(){func() {
		testpkg.CleanupTableRecords(t, s.db, "activities.groups", otherGroup.ID)
	}}, s.extraCleanups...)
	otherPlannedID := splitInsertInstance(t, s, &otherGroup.ID, effective, scheduleModels.InstanceStatusPlanned, false, 12)

	// --- Act ----------------------------------------------------------------
	in := baseSplitInput(s, effective, fmt.Sprintf("Split-Nachfolger-%d", suffix))
	in.MaterializeFrom = &effective
	in.MaterializeTo = &secondMonday
	// StudentIDs/StaffIDs nil → carry over the previously-active roster.

	res, err := s.factory.TemplateSplit.Split(s.ctx, in)
	require.NoError(t, err)
	registerSuccessorCleanup(t, s, res.NewTemplateID)
	for _, d := range []timezone.Date{effective, secondMonday} {
		for _, inst := range listInstancesForDate(t, s.db, res.NewTemplateID, d) {
			s.registerCleanup("schedule.activity_instances", inst.ID)
		}
	}

	// --- Assert: result shape ------------------------------------------------
	assert.Equal(t, s.template.ID, res.OldTemplateID)
	require.NotZero(t, res.NewTemplateID)
	assert.NotEqual(t, res.OldTemplateID, res.NewTemplateID)
	require.Len(t, res.NewScheduleIDs, 1)

	// Old schedule capped (exclusive end = effective date).
	oldSchedule := reloadSplitSchedule(t, s, s.schedule.ID)
	require.NotNil(t, oldSchedule.ValidUntil, "old schedule must be capped")
	assert.Equal(t, effective, *oldSchedule.ValidUntil)

	// Successor schedule open-ended, same weekday, week_pattern preserved
	// (input nil → 0, matching the old schedule's pattern).
	newSchedules := loadSplitSchedules(t, s, res.NewTemplateID)
	require.Len(t, newSchedules, 1)
	assert.Equal(t, activitiesModels.WeekdayMonday, newSchedules[0].Weekday)
	assert.Equal(t, s.schedule.WeekPattern, newSchedules[0].WeekPattern, "week_pattern preserved")
	assert.Nil(t, newSchedules[0].ValidUntil, "successor starts open-ended")

	// Old roster capped: the two active enrollments end at the effective
	// date; the already-expired one keeps its original end.
	oldEnrollments := loadSplitEnrollments(t, s, s.template.ID)
	require.Len(t, oldEnrollments, 3)
	expiredUntil := effective.AddDays(-1)
	for _, e := range oldEnrollments {
		require.NotNil(t, e.ValidUntil, "every old enrollment must be ended")
		if e.StudentID == s.students[2] {
			assert.Equal(t, expiredUntil, *e.ValidUntil, "already-expired row untouched")
		} else {
			assert.Equal(t, effective, *e.ValidUntil, "active row capped at effective date")
		}
	}
	oldSupervisors := loadSplitSupervisors(t, s, s.template.ID)
	require.Len(t, oldSupervisors, 1)
	require.NotNil(t, oldSupervisors[0].ValidUntil)
	assert.Equal(t, effective, *oldSupervisors[0].ValidUntil)

	// Carried-over roster: only the previously-active students, starting at
	// the effective date, open-ended; primary flag preserved.
	newEnrollments := loadSplitEnrollments(t, s, res.NewTemplateID)
	require.Len(t, newEnrollments, 2, "expired student must not carry over")
	assert.Equal(t, []int64{s.students[0], s.students[1]},
		[]int64{newEnrollments[0].StudentID, newEnrollments[1].StudentID})
	for _, e := range newEnrollments {
		assert.Equal(t, effective, e.ValidFrom)
		assert.Nil(t, e.ValidUntil)
	}
	newSupervisors := loadSplitSupervisors(t, s, res.NewTemplateID)
	require.Len(t, newSupervisors, 1)
	assert.Equal(t, s.staffID, newSupervisors[0].StaffID)
	assert.True(t, newSupervisors[0].IsPrimary, "primary flag preserved on carry-over")
	assert.Equal(t, effective, newSupervisors[0].ValidFrom)

	// Planned future instance of the old template deleted; everything else
	// survives.
	assert.Equal(t, 1, res.DeletedInstances, "only the planned non-spontaneous old-template instance is deleted")
	assert.False(t, splitInstanceExists(t, s, second[0].ID), "planned old-template instance deleted")
	assert.True(t, splitInstanceExists(t, s, first[0].ID), "active instance survives")
	assert.True(t, splitInstanceExists(t, s, completedID), "completed instance survives")
	assert.True(t, splitInstanceExists(t, s, cancelledID), "cancelled instance survives")
	assert.True(t, splitInstanceExists(t, s, spontaneousID), "spontaneous instance survives")
	assert.True(t, splitInstanceExists(t, s, otherPlannedID), "other group's planned instance survives")

	// Successor materialized with roster on both Mondays; the capped old
	// schedule is counted as ended for both.
	require.NotNil(t, res.Materialization)
	assert.Equal(t, 2, res.Materialization.InstancesCreated)
	assert.Equal(t, 2, res.Materialization.CandidatesSkippedEnded)
	assert.Equal(t, 4, res.Materialization.InstanceStudentsCreated, "2 carried students x 2 instances")
	assert.Equal(t, 2, res.Materialization.InstanceStaffCreated)
	assert.Len(t, listInstancesForDate(t, s.db, res.NewTemplateID, effective), 1)
	assert.Len(t, listInstancesForDate(t, s.db, res.NewTemplateID, secondMonday), 1)
}

func TestTemplateSplit_ExplicitRosterAndWeekPattern(t *testing.T) {
	effective := futureMonday(1)
	s := makeScenario(t, activitiesModels.WeekdayMonday, effective)
	defer s.runCleanup(t)
	suffix := time.Now().UnixNano()

	weekPattern := 1
	in := baseSplitInput(s, effective, fmt.Sprintf("Split-Explizit-%d", suffix))
	in.Type = activitiesModels.GroupTypeCare
	in.Weekdays = []int{activitiesModels.WeekdayTuesday, activitiesModels.WeekdayThursday}
	in.WeekPattern = &weekPattern
	// Explicit roster wins over carry-over: only the (previously expired)
	// third student, duplicates and non-positive ids dropped.
	in.StudentIDs = []int64{s.students[2], s.students[2], 0}
	in.StaffIDs = []int64{s.staffID}
	in.PrimaryStaffID = &s.staffID
	// No MaterializeFrom/To → no materialization run.

	res, err := s.factory.TemplateSplit.Split(s.ctx, in)
	require.NoError(t, err)
	registerSuccessorCleanup(t, s, res.NewTemplateID)

	assert.Nil(t, res.Materialization, "no window requested, no materialization")
	assert.Zero(t, res.DeletedInstances, "no planned instances existed in the horizon")
	require.Len(t, res.NewScheduleIDs, 2)

	newSchedules := loadSplitSchedules(t, s, res.NewTemplateID)
	require.Len(t, newSchedules, 2)
	assert.Equal(t, activitiesModels.WeekdayTuesday, newSchedules[0].Weekday)
	assert.Equal(t, activitiesModels.WeekdayThursday, newSchedules[1].Weekday)
	for _, sch := range newSchedules {
		assert.Equal(t, weekPattern, sch.WeekPattern, "explicit week_pattern passes through")
		assert.Nil(t, sch.ValidUntil)
	}

	newEnrollments := loadSplitEnrollments(t, s, res.NewTemplateID)
	require.Len(t, newEnrollments, 1, "explicit roster wins; duplicates and zero ids dropped")
	assert.Equal(t, s.students[2], newEnrollments[0].StudentID)
	assert.Equal(t, effective, newEnrollments[0].ValidFrom)
	assert.Nil(t, newEnrollments[0].ValidUntil)

	newSupervisors := loadSplitSupervisors(t, s, res.NewTemplateID)
	require.Len(t, newSupervisors, 1)
	assert.True(t, newSupervisors[0].IsPrimary, "PrimaryStaffID applies to explicit staff roster")

	// Old roster is capped even when an explicit roster is supplied.
	for _, e := range loadSplitEnrollments(t, s, s.template.ID) {
		require.NotNil(t, e.ValidUntil)
	}
	oldSchedule := reloadSplitSchedule(t, s, s.schedule.ID)
	require.NotNil(t, oldSchedule.ValidUntil)
	assert.Equal(t, effective, *oldSchedule.ValidUntil)
}

func TestTemplateSplit_ValidationErrors(t *testing.T) {
	effective := futureMonday(1)
	s := makeScenario(t, activitiesModels.WeekdayMonday, effective)
	defer s.runCleanup(t)
	suffix := time.Now().UnixNano()

	t.Run("past effective_date", func(t *testing.T) {
		in := baseSplitInput(s, timezone.TodayDate().AddDays(-1), fmt.Sprintf("Split-Vergangen-%d", suffix))
		_, err := s.factory.TemplateSplit.Split(s.ctx, in)
		require.ErrorIs(t, err, scheduleSvc.ErrSplitInvalidInput)
	})

	t.Run("non-template id", func(t *testing.T) {
		roomID := s.roomID
		plain := &activitiesModels.Group{
			Name:            fmt.Sprintf("Split-KeinTemplate-%d", suffix),
			MaxParticipants: 10,
			CategoryID:      s.categoryID,
			PlannedRoomID:   &roomID,
			IsTemplate:      false,
		}
		plain.SetTenantID(s.tenantID)
		_, err := s.db.NewInsert().Model(plain).ModelTableExpr(`activities.groups AS "group"`).Exec(s.ctx)
		require.NoError(t, err)
		s.extraCleanups = append([]func(){func() {
			testpkg.CleanupTableRecords(t, s.db, "activities.groups", plain.ID)
		}}, s.extraCleanups...)

		in := baseSplitInput(s, effective, fmt.Sprintf("Split-KeinTemplateNeu-%d", suffix))
		in.TemplateID = plain.ID
		_, err = s.factory.TemplateSplit.Split(s.ctx, in)
		require.ErrorIs(t, err, scheduleSvc.ErrSplitTemplateNotFound)
	})

	t.Run("unknown id", func(t *testing.T) {
		in := baseSplitInput(s, effective, fmt.Sprintf("Split-Unbekannt-%d", suffix))
		in.TemplateID = 999999999
		_, err := s.factory.TemplateSplit.Split(s.ctx, in)
		require.ErrorIs(t, err, scheduleSvc.ErrSplitTemplateNotFound)
	})

	// None of the failed attempts may have mutated the old template.
	oldSchedule := reloadSplitSchedule(t, s, s.schedule.ID)
	assert.Nil(t, oldSchedule.ValidUntil, "failed splits must not cap the old schedule")
}

// Successor schedules carry valid_from = effective date, so a later
// materialization run over a window BEFORE the split point never emits
// phantom successor instances next to the old template's rows.
func TestTemplateSplit_SuccessorValidFrom_NoPhantomBeforeEffective(t *testing.T) {
	effective := futureMonday(1)
	prevMonday := effective.AddDays(-7) // still in the future (futureMonday(0))
	s := makeScenario(t, activitiesModels.WeekdayMonday, effective)
	defer s.runCleanup(t)
	suffix := time.Now().UnixNano()

	in := baseSplitInput(s, effective, fmt.Sprintf("Split-ValidFrom-%d", suffix))
	res, err := s.factory.TemplateSplit.Split(s.ctx, in)
	require.NoError(t, err)
	registerSuccessorCleanup(t, s, res.NewTemplateID)

	newSchedules := loadSplitSchedules(t, s, res.NewTemplateID)
	require.Len(t, newSchedules, 1)
	require.NotNil(t, newSchedules[0].ValidFrom, "successor schedule must carry valid_from")
	assert.Equal(t, effective, *newSchedules[0].ValidFrom)

	// Materialize the week BEFORE the effective date: the old (capped)
	// template still owns it; the successor must be skipped as not started.
	r, err := s.svc.MaterializeForTenant(s.ctx, prevMonday, prevMonday, scheduleSvc.MaterializationSourceManual)
	require.NoError(t, err)
	for _, inst := range listInstancesForDate(t, s.db, s.template.ID, prevMonday) {
		s.registerCleanup("schedule.activity_instances", inst.ID)
	}
	assert.Equal(t, 1, r.InstancesCreated, "old template still materializes before the split point")
	assert.Equal(t, 1, r.CandidatesSkippedNotStarted, "successor skipped before its valid_from")
	assert.Empty(t, listInstancesForDate(t, s.db, res.NewTemplateID, prevMonday),
		"no phantom successor instance before the effective date")
}

// Multi-period rosters (one active row per calendar period since migration
// 1.15.52) must collapse to ONE successor row per person — previously the
// carry path stamped every active row with the successor's period id and
// violated idx_student_enrollments_active (500 on split).
func TestTemplateSplit_MultiPeriodRosterCarriesOnce(t *testing.T) {
	effective := futureMonday(1)
	s := makeScenario(t, activitiesModels.WeekdayMonday, effective)
	defer s.runCleanup(t)
	suffix := time.Now().UnixNano()

	// Scope the first student's existing active row to weekdays [Mon].
	// jsonb column — bind the JSON literal, not a Go slice (bun would render
	// a slice as an SQL list, not jsonb).
	_, err := s.db.NewUpdate().
		Model((*activitiesModels.StudentEnrollment)(nil)).
		ModelTableExpr(`activities.student_enrollments`).
		Set("selected_weekdays = ?::jsonb", `[1]`).
		Where("id = ?", s.enrollmentIDs[0]).
		Where("tenant_id = ?", s.tenantID).
		Exec(s.ctx)
	require.NoError(t, err)

	// … and add a SECOND active row for the same student, scoped to the
	// scenario period and weekdays [Wed]. Both rows are active (valid_until
	// IS NULL) — legal under the period-scoped unique index.
	secondEnroll := &activitiesModels.StudentEnrollment{
		StudentID:        s.students[0],
		ActivityGroupID:  s.template.ID,
		ValidFrom:        effective.AddDays(-14),
		CalendarPeriodID: &s.period.ID,
		SelectedWeekdays: []int{activitiesModels.WeekdayWednesday},
	}
	secondEnroll.SetTenantID(s.tenantID)
	_, err = s.db.NewInsert().Model(secondEnroll).ModelTableExpr(`activities.student_enrollments`).Exec(s.ctx)
	require.NoError(t, err)
	s.registerCleanup("activities.student_enrollments", secondEnroll.ID)

	// Same situation for the supervisor: a second active period-scoped row.
	secondSup := &activitiesModels.SupervisorPlanned{
		StaffID:          s.staffID,
		GroupID:          s.template.ID,
		IsPrimary:        false,
		ValidFrom:        effective.AddDays(-14),
		CalendarPeriodID: &s.period.ID,
	}
	secondSup.SetTenantID(s.tenantID)
	_, err = s.db.NewInsert().Model(secondSup).ModelTableExpr(`activities.supervisors`).Exec(s.ctx)
	require.NoError(t, err)
	s.registerCleanup("activities.supervisors", secondSup.ID)

	in := baseSplitInput(s, effective, fmt.Sprintf("Split-MultiPeriode-%d", suffix))
	// nil StudentIDs/StaffIDs → carry; nil CalendarPeriodID → successor rows
	// land on COALESCE(calendar_period_id, 0) = 0 and would collide without
	// the per-person dedupe.
	res, err := s.factory.TemplateSplit.Split(s.ctx, in)
	require.NoError(t, err, "multi-period roster must not violate idx_student_enrollments_active")
	registerSuccessorCleanup(t, s, res.NewTemplateID)

	newEnrollments := loadSplitEnrollments(t, s, res.NewTemplateID)
	require.Len(t, newEnrollments, 2, "exactly one successor row per student")
	byStudent := map[int64]*activitiesModels.StudentEnrollment{}
	for _, e := range newEnrollments {
		byStudent[e.StudentID] = e
	}
	require.Contains(t, byStudent, s.students[0])
	require.Contains(t, byStudent, s.students[1])
	assert.Equal(t, []int{activitiesModels.WeekdayMonday, activitiesModels.WeekdayWednesday},
		byStudent[s.students[0]].SelectedWeekdays, "selected_weekdays unioned across the student's rows")

	newSupervisors := loadSplitSupervisors(t, s, res.NewTemplateID)
	require.Len(t, newSupervisors, 1, "exactly one successor row per staff member")
	assert.Equal(t, s.staffID, newSupervisors[0].StaffID)
	assert.True(t, newSupervisors[0].IsPrimary,
		"preferred (period-matching, here nil-period) row's is_primary flag wins")
}

// Self-inconsistent materialization windows are rejected up front as
// ErrSplitInvalidInput (→ 400) instead of bubbling out of the materializer
// as 500s.
func TestTemplateSplit_WindowValidation(t *testing.T) {
	effective := futureMonday(1)
	s := makeScenario(t, activitiesModels.WeekdayMonday, effective)
	defer s.runCleanup(t)
	suffix := time.Now().UnixNano()

	t.Run("materialize_to before effective_date", func(t *testing.T) {
		in := baseSplitInput(s, effective, fmt.Sprintf("Split-Fenster-A-%d", suffix))
		before := effective.AddDays(-1)
		in.MaterializeTo = &before
		_, err := s.factory.TemplateSplit.Split(s.ctx, in)
		require.ErrorIs(t, err, scheduleSvc.ErrSplitInvalidInput)
	})

	t.Run("materialize_from after materialize_to", func(t *testing.T) {
		in := baseSplitInput(s, effective, fmt.Sprintf("Split-Fenster-B-%d", suffix))
		from := effective.AddDays(14)
		to := effective.AddDays(7)
		in.MaterializeFrom = &from
		in.MaterializeTo = &to
		_, err := s.factory.TemplateSplit.Split(s.ctx, in)
		require.ErrorIs(t, err, scheduleSvc.ErrSplitInvalidInput)
	})

	t.Run("window exceeds max days after clamping", func(t *testing.T) {
		in := baseSplitInput(s, effective, fmt.Sprintf("Split-Fenster-C-%d", suffix))
		from := effective
		to := effective.AddDays(scheduleSvc.MaxMaterializationWindowDays) // span = max+1
		in.MaterializeFrom = &from
		in.MaterializeTo = &to
		_, err := s.factory.TemplateSplit.Split(s.ctx, in)
		require.ErrorIs(t, err, scheduleSvc.ErrSplitInvalidInput)
	})

	// None of the failed attempts may have mutated the old template.
	oldSchedule := reloadSplitSchedule(t, s, s.schedule.ID)
	assert.Nil(t, oldSchedule.ValidUntil, "failed splits must not cap the old schedule")
}

// The old-template purge is decoupled from the materialization window: after
// a successful split NO planned non-spontaneous old-template instance exists
// on/after the effective date, even beyond materialize_to.
func TestTemplateSplit_DeletesPlannedBeyondMaterializeWindow(t *testing.T) {
	effective := futureMonday(1)
	secondMonday := effective.AddDays(7)
	s := makeScenario(t, activitiesModels.WeekdayMonday, effective)
	defer s.runCleanup(t)
	suffix := time.Now().UnixNano()

	// Materialize the old template for BOTH Mondays.
	r0, err := s.svc.MaterializeForTenant(s.ctx, effective, secondMonday, scheduleSvc.MaterializationSourceManual)
	require.NoError(t, err)
	require.Equal(t, 2, r0.InstancesCreated)
	first := listInstancesForDate(t, s.db, s.template.ID, effective)
	require.Len(t, first, 1)
	second := listInstancesForDate(t, s.db, s.template.ID, secondMonday)
	require.Len(t, second, 1)
	s.registerCleanup("schedule.activity_instances", first[0].ID, second[0].ID)

	// Split with a materialization window covering ONLY the first Monday.
	in := baseSplitInput(s, effective, fmt.Sprintf("Split-OffenesEnde-%d", suffix))
	in.MaterializeFrom = &effective
	in.MaterializeTo = &effective

	res, err := s.factory.TemplateSplit.Split(s.ctx, in)
	require.NoError(t, err)
	registerSuccessorCleanup(t, s, res.NewTemplateID)
	for _, inst := range listInstancesForDate(t, s.db, res.NewTemplateID, effective) {
		s.registerCleanup("schedule.activity_instances", inst.ID)
	}

	assert.Equal(t, 2, res.DeletedInstances,
		"both planned old-template instances deleted, including the one beyond materialize_to")
	assert.False(t, splitInstanceExists(t, s, first[0].ID))
	assert.False(t, splitInstanceExists(t, s, second[0].ID),
		"planned old-template instance beyond the materialization window must not survive")
}
