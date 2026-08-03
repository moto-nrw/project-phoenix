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
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/schoolclass"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activitiesModels "github.com/moto-nrw/project-phoenix/models/activities"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	scheduleSvc "github.com/moto-nrw/project-phoenix/services/schedule"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
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

func createSplitRequestChild(t *testing.T, s *scenarioSetup, studentID int64) int64 {
	t.Helper()
	suffix := time.Now().UnixNano()
	var phaseID int64
	require.NoError(t, s.db.NewRaw(`
		INSERT INTO enrollment.phases
			(tenant_id, name, kind, service_start_date, service_end_date)
		VALUES (?, ?, 'custom', '2026-01-01', '2028-12-31')
		RETURNING id
	`, s.tenantID, fmt.Sprintf("Split provenance %d", suffix)).Scan(s.ctx, &phaseID))

	var requestID int64
	require.NoError(t, s.db.NewRaw(`
		INSERT INTO enrollment.requests
			(tenant_id, phase_id, guardian_first_name, guardian_last_name,
			 guardian_email, consent_flags, custom_data, status_token, submitted_at)
		VALUES (?, ?, 'Anna', 'Beispiel', ?, '{}'::jsonb, '{}'::jsonb, ?, NOW())
		RETURNING id
	`, s.tenantID, phaseID, fmt.Sprintf("split-%d@example.test", suffix), fmt.Sprintf("split-source-%d", suffix)).Scan(s.ctx, &requestID))

	var childID int64
	require.NoError(t, s.db.NewRaw(`
		INSERT INTO enrollment.request_children
			(tenant_id, request_id, first_name, last_name, date_of_birth,
			 status, activation_mode, created_student_id, sort_order, custom_data)
		VALUES (?, ?, 'Lina', 'Quelle', '2018-04-15',
			'approved', 'scheduled', ?, 0, '{}'::jsonb)
		RETURNING id
	`, s.tenantID, requestID, studentID).Scan(s.ctx, &childID))

	s.extraCleanups = append([]func(){func() {
		testpkg.CleanupTableRecords(t, s.db, "enrollment.request_children", childID)
		testpkg.CleanupTableRecords(t, s.db, "enrollment.requests", requestID)
		testpkg.CleanupTableRecords(t, s.db, "enrollment.phases", phaseID)
	}}, s.extraCleanups...)
	return childID
}

func createSplitRequestChildInPhase(t *testing.T, s *scenarioSetup, phaseID, studentID int64) int64 {
	t.Helper()
	suffix := time.Now().UnixNano()
	var requestID int64
	require.NoError(t, s.db.NewRaw(`
		INSERT INTO enrollment.requests
			(tenant_id, phase_id, guardian_first_name, guardian_last_name,
			 guardian_email, consent_flags, custom_data, status_token, submitted_at)
		VALUES (?, ?, 'Anna', 'Periode', ?, '{}'::jsonb, '{}'::jsonb, ?, NOW())
		RETURNING id
	`, s.tenantID, phaseID, fmt.Sprintf("period-rebase-%d@example.test", suffix), fmt.Sprintf("period-rebase-%d", suffix)).Scan(s.ctx, &requestID))
	var childID int64
	require.NoError(t, s.db.NewRaw(`
		INSERT INTO enrollment.request_children
			(tenant_id, request_id, first_name, last_name, date_of_birth,
			 status, activation_mode, created_student_id, sort_order, custom_data)
		VALUES (?, ?, 'Lina', 'Periode', '2018-04-15',
			'approved', 'scheduled', ?, 0, '{}'::jsonb)
		RETURNING id
	`, s.tenantID, requestID, studentID).Scan(s.ctx, &childID))
	s.extraCleanups = append([]func(){func() {
		testpkg.CleanupTableRecords(t, s.db, "enrollment.request_children", childID)
		testpkg.CleanupTableRecords(t, s.db, "enrollment.requests", requestID)
	}}, s.extraCleanups...)
	return childID
}

func createProtectedSplitEnrollment(
	t *testing.T,
	s *scenarioSetup,
	requestChildID, studentID int64,
	validFrom, validUntil timezone.Date,
	periodID int64,
) *activitiesModels.StudentEnrollment {
	t.Helper()
	row := &activitiesModels.StudentEnrollment{
		StudentID: studentID, ActivityGroupID: s.template.ID,
		ValidFrom: validFrom, ValidUntil: &validUntil,
		CalendarPeriodID: &periodID, EnrollmentRequestChildID: &requestChildID,
		SelectedWeekdays: []int{activitiesModels.WeekdayMonday},
	}
	row.SetTenantID(s.tenantID)
	require.NoError(t, repositories.NewFactory(s.db).StudentEnrollment.Create(s.ctx, row))
	s.registerCleanup("activities.student_enrollments", row.ID)
	return row
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
		GradeLevelMax: schoolclass.MaxGradeLevel,
	}
}

func createLinkedCareOffering(
	t *testing.T,
	s *scenarioSetup,
	serviceStart, serviceEnd timezone.Date,
) *enrollmentModels.CareOffering {
	t.Helper()

	_, err := s.db.NewUpdate().
		Model((*activitiesModels.Group)(nil)).
		ModelTableExpr(`activities.groups AS "group"`).
		Set("calendar_period_id = ?", s.period.ID).
		Where(`"group".id = ?`, s.template.ID).
		Where(`"group".tenant_id = ?`, s.tenantID).
		Exec(s.ctx)
	require.NoError(t, err)
	s.template.CalendarPeriodID = &s.period.ID

	phase := &enrollmentModels.Phase{
		Name:                      fmt.Sprintf("Linked care phase %d", time.Now().UnixNano()),
		Kind:                      enrollmentModels.PhaseKindCustom,
		ServiceStartDate:          serviceStart,
		ServiceEndDate:            serviceEnd,
		CareOverflowMode:          enrollmentModels.PhaseCareOverflowWaitlist,
		CareOfferingSelectionMode: enrollmentModels.PhaseCareOfferingSelectionOptional,
		AvailableSchoolClasses:    []string{},
		IsActive:                  true,
	}
	phase.SetTenantID(s.tenantID)
	repos := repositories.NewFactory(s.db)
	require.NoError(t, repos.Phase.Create(s.ctx, phase))

	offering := &enrollmentModels.CareOffering{
		PhaseID:            phase.ID,
		ActivityGroupID:    &s.template.ID,
		Name:               fmt.Sprintf("Linked care offering %d", time.Now().UnixNano()),
		DaysOfWeekMode:     enrollmentModels.DaysOfWeekModeFixed,
		AvailableDays:      []string{"mon"},
		IsActive:           true,
		CountsAsCare:       true,
		CountsAsCareSet:    true,
		SelectionRule:      enrollmentModels.SelectionRuleOptional,
		AutoAddGradeLevels: []int{},
	}
	var created *enrollmentModels.CareOffering
	require.NoError(t, tenant.WithTenantTx(s.ctx, s.db, s.tenantID, func(txCtx context.Context, _ bun.Tx) error {
		var createErr error
		created, createErr = s.factory.EnrollmentCareOffering.Create(txCtx, offering)
		return createErr
	}))

	s.extraCleanups = append([]func(){func() {
		testpkg.CleanupTableRecords(t, s.db, "enrollment.care_offerings", created.ID)
		testpkg.CleanupTableRecords(t, s.db, "enrollment.phases", phase.ID)
	}}, s.extraCleanups...)
	return created
}

func createShortCalendarPeriod(t *testing.T, s *scenarioSetup, start, end timezone.Date) *scheduleModels.CalendarPeriod {
	t.Helper()
	period := &scheduleModels.CalendarPeriod{
		Name:            fmt.Sprintf("Short linked period %d", time.Now().UnixNano()),
		PeriodType:      scheduleModels.PeriodTypeCustom,
		StartDate:       start,
		EndDate:         end,
		WeekCycleLength: 1,
		IsActive:        true,
	}
	require.NoError(t, s.factory.CalendarPeriod.CreatePeriod(s.ctx, period))
	s.extraCleanups = append([]func(){func() {
		testpkg.CleanupTableRecords(t, s.db, "schedule.calendar_periods", period.ID)
	}}, s.extraCleanups...)
	return period
}

func createCompatibleCalendarPeriod(t *testing.T, s *scenarioSetup) *scheduleModels.CalendarPeriod {
	t.Helper()
	period := createShortCalendarPeriod(t, s, s.period.StartDate, s.period.EndDate)
	period.Name = fmt.Sprintf("Compatible linked period %d", time.Now().UnixNano())
	return period
}

func splitInstanceContainsStudent(t *testing.T, s *scenarioSetup, instanceID, studentID int64) bool {
	t.Helper()
	count, err := s.db.NewSelect().
		TableExpr(`schedule.instance_students AS "instance_student"`).
		Where(`"instance_student".tenant_id = ?`, s.tenantID).
		Where(`"instance_student".instance_id = ?`, instanceID).
		Where(`"instance_student".student_id = ?`, studentID).
		Count(s.ctx)
	require.NoError(t, err)
	return count > 0
}

func linkedTemplateUpdateInput(t *testing.T, s *scenarioSetup, periodID *int64) scheduleSvc.TemplateUpdateInput {
	t.Helper()
	group := reloadSplitGroup(t, s, s.template.ID)
	return scheduleSvc.TemplateUpdateInput{
		TemplateID: s.template.ID,
		Fields: activitiesModels.TemplateFieldsUpdate{
			Name:              group.Name + "-edited",
			Type:              group.Type,
			CategoryID:        group.CategoryID,
			RoomID:            *group.PlannedRoomID,
			EducationGroupID:  group.EducationGroupID,
			MaxParticipants:   group.MaxParticipants,
			CalendarPeriodID:  periodID,
			TargetGroupType:   group.TargetGroupType,
			TargetGradeLevel:  group.TargetGradeLevel,
			TargetSchoolClass: group.TargetSchoolClass,
		},
		Weekdays:         []int{activitiesModels.WeekdayMonday},
		TimeframeID:      s.timeframe.ID,
		WeekPattern:      s.schedule.WeekPattern,
		CalendarPeriodID: periodID,
		RosterValidFrom:  s.period.StartDate,
		StudentIDs:       []int64{s.students[0], s.students[1]},
		StaffIDs:         []int64{s.staffID},
		PrimaryStaffID:   &s.staffID,
		GradeLevelMax:    schoolclass.MaxGradeLevel,
	}
}

func TestTemplateMutations_RejectCareOfferingSeriesConflictsWithoutPersisting(t *testing.T) {
	effective := futureMonday(1)

	t.Run("split", func(t *testing.T) {
		s := makeScenario(t, activitiesModels.WeekdayMonday, effective)
		defer s.runCleanup(t)
		createLinkedCareOffering(t, s, effective.AddDays(-7), effective.AddDays(21))
		shortPeriod := createShortCalendarPeriod(t, s, effective, effective.AddDays(7))

		in := baseSplitInput(s, effective, fmt.Sprintf("Split-Care-Conflict-%d", time.Now().UnixNano()))
		in.CalendarPeriodID = &shortPeriod.ID
		_, err := s.factory.TemplateSplit.Split(s.ctx, in)
		require.ErrorIs(t, err, scheduleSvc.ErrTemplateCareOfferingConflict)
		require.ErrorIs(t, err, scheduleSvc.ErrSplitInvalidInput)

		assert.Nil(t, reloadSplitSchedule(t, s, s.schedule.ID).ValidUntil)
		series, seriesErr := repositories.NewFactory(s.db).ActivityGroup.FindTemplateSeries(s.ctx, s.template.ID)
		require.NoError(t, seriesErr)
		require.Len(t, series, 1)
		assert.Equal(t, s.template.ID, series[0].ID)
	})

	t.Run("update", func(t *testing.T) {
		s := makeScenario(t, activitiesModels.WeekdayMonday, effective)
		defer s.runCleanup(t)
		createLinkedCareOffering(t, s, effective, effective.AddDays(21))
		shortPeriod := createShortCalendarPeriod(t, s, effective, effective.AddDays(7))
		original := reloadSplitGroup(t, s, s.template.ID)

		err := s.factory.TimetableData.UpdateTemplate(s.ctx, linkedTemplateUpdateInput(t, s, &shortPeriod.ID))
		require.ErrorIs(t, err, scheduleSvc.ErrTemplateCareOfferingConflict)

		reloaded := reloadSplitGroup(t, s, s.template.ID)
		assert.Equal(t, original.Name, reloaded.Name)
		require.NotNil(t, reloaded.CalendarPeriodID)
		assert.Equal(t, s.period.ID, *reloaded.CalendarPeriodID)
		schedules := loadSplitSchedules(t, s, s.template.ID)
		require.Len(t, schedules, 1)
		assert.Nil(t, schedules[0].CalendarPeriodID)
	})

	t.Run("end", func(t *testing.T) {
		s := makeScenario(t, activitiesModels.WeekdayMonday, effective)
		defer s.runCleanup(t)
		createLinkedCareOffering(t, s, effective, effective.AddDays(21))

		_, err := s.factory.TemplateSplit.EndFromDate(s.ctx, scheduleSvc.TemplateEndInput{
			TemplateID:    s.template.ID,
			EffectiveDate: effective,
		})
		require.ErrorIs(t, err, scheduleSvc.ErrTemplateCareOfferingConflict)
		assert.Nil(t, reloadSplitSchedule(t, s, s.schedule.ID).ValidUntil)
	})

	t.Run("end mid-phase tail", func(t *testing.T) {
		s := makeScenario(t, activitiesModels.WeekdayMonday, effective)
		defer s.runCleanup(t)
		createLinkedCareOffering(t, s, effective.AddDays(-7), effective.AddDays(21))

		_, err := s.factory.TemplateSplit.EndFromDate(s.ctx, scheduleSvc.TemplateEndInput{
			TemplateID:    s.template.ID,
			EffectiveDate: effective,
		})
		require.ErrorIs(t, err, scheduleSvc.ErrTemplateCareOfferingConflict)
		assert.Nil(t, reloadSplitSchedule(t, s, s.schedule.ID).ValidUntil)
	})

	t.Run("end inactive offering still selected by approved child", func(t *testing.T) {
		s := makeScenario(t, activitiesModels.WeekdayMonday, effective)
		defer s.runCleanup(t)
		offering := createLinkedCareOffering(t, s, effective.AddDays(-7), effective.AddDays(21))
		childID := createSplitRequestChildInPhase(t, s, offering.PhaseID, s.students[2])
		repos := repositories.NewFactory(s.db)
		selection := &enrollmentModels.RequestChildOffering{
			RequestChildID: childID,
			CareOfferingID: offering.ID,
			SelectedDays:   []string{"mon"},
		}
		require.NoError(t, repos.RequestChildOffering.Create(s.ctx, selection))
		s.extraCleanups = append([]func(){func() {
			testpkg.CleanupTableRecords(t, s.db, "enrollment.request_child_offerings", selection.ID)
		}}, s.extraCleanups...)
		offering.IsActive = false
		require.NoError(t, repos.CareOffering.Update(s.ctx, offering))

		_, err := s.factory.TemplateSplit.EndFromDate(s.ctx, scheduleSvc.TemplateEndInput{
			TemplateID:    s.template.ID,
			EffectiveDate: effective,
		})
		require.ErrorIs(t, err, scheduleSvc.ErrTemplateCareOfferingConflict)
		assert.Nil(t, reloadSplitSchedule(t, s, s.schedule.ID).ValidUntil)
	})

	t.Run("split removes advertised weekday", func(t *testing.T) {
		s := makeScenario(t, activitiesModels.WeekdayMonday, effective)
		defer s.runCleanup(t)
		createLinkedCareOffering(t, s, effective.AddDays(-7), effective.AddDays(21))
		in := baseSplitInput(s, effective, fmt.Sprintf("Split-Weekday-Conflict-%d", time.Now().UnixNano()))
		in.Weekdays = []int{activitiesModels.WeekdayTuesday}
		in.CalendarPeriodID = &s.period.ID

		_, err := s.factory.TemplateSplit.Split(s.ctx, in)
		require.ErrorIs(t, err, scheduleSvc.ErrTemplateCareOfferingConflict)
		assert.Nil(t, reloadSplitSchedule(t, s, s.schedule.ID).ValidUntil)
	})

	t.Run("update removes advertised weekday", func(t *testing.T) {
		s := makeScenario(t, activitiesModels.WeekdayMonday, effective)
		defer s.runCleanup(t)
		createLinkedCareOffering(t, s, effective, effective.AddDays(21))
		in := linkedTemplateUpdateInput(t, s, &s.period.ID)
		in.Weekdays = []int{activitiesModels.WeekdayTuesday}

		err := s.factory.TimetableData.UpdateTemplate(s.ctx, in)
		require.ErrorIs(t, err, scheduleSvc.ErrTemplateCareOfferingConflict)
		schedules := loadSplitSchedules(t, s, s.template.ID)
		require.Len(t, schedules, 1)
		assert.Equal(t, activitiesModels.WeekdayMonday, schedules[0].Weekday)
	})

	t.Run("update removes B-week occurrences", func(t *testing.T) {
		s := makeScenario(t, activitiesModels.WeekdayMonday, effective)
		defer s.runCleanup(t)
		anchor := effective
		s.period.WeekCycleLength = 2
		s.period.WeekCycleAnchor = &anchor
		require.NoError(t, s.factory.CalendarPeriod.UpdatePeriod(s.ctx, s.period))
		createLinkedCareOffering(t, s, effective, effective.AddDays(21))
		in := linkedTemplateUpdateInput(t, s, &s.period.ID)
		in.WeekPattern = 1

		err := s.factory.TimetableData.UpdateTemplate(s.ctx, in)
		require.ErrorIs(t, err, scheduleSvc.ErrTemplateCareOfferingConflict)
		assert.Equal(t, 0, reloadSplitSchedule(t, s, s.schedule.ID).WeekPattern)
	})

	t.Run("archive", func(t *testing.T) {
		s := makeScenario(t, activitiesModels.WeekdayMonday, effective)
		defer s.runCleanup(t)
		createLinkedCareOffering(t, s, effective, effective.AddDays(21))

		_, err := s.factory.TimetableData.ArchiveTemplate(s.ctx, s.template.ID)
		require.ErrorIs(t, err, scheduleSvc.ErrTemplateCareOfferingConflict)
		assert.Nil(t, reloadSplitGroup(t, s, s.template.ID).ArchivedAt)
	})
}

func TestTemplatePeriodChange_RebasesProtectedEnrollmentForMaterialization(t *testing.T) {
	effective := futureMonday(1)

	t.Run("split A to B", func(t *testing.T) {
		s := makeScenario(t, activitiesModels.WeekdayMonday, effective)
		defer s.runCleanup(t)
		phaseStart, phaseEnd := effective.AddDays(-7), effective.AddDays(21)
		offering := createLinkedCareOffering(t, s, phaseStart, phaseEnd)
		childID := createSplitRequestChildInPhase(t, s, offering.PhaseID, s.students[2])
		protectedUntil := phaseEnd.AddDays(1)
		createProtectedSplitEnrollment(t, s, childID, s.students[2], phaseStart, protectedUntil, s.period.ID)
		periodB := createCompatibleCalendarPeriod(t, s)

		in := baseSplitInput(s, effective, fmt.Sprintf("Split-Rebase-%d", time.Now().UnixNano()))
		in.CalendarPeriodID = &periodB.ID
		result, err := s.factory.TemplateSplit.Split(s.ctx, in)
		require.NoError(t, err)
		registerSuccessorCleanup(t, s, result.NewTemplateID)

		rows := loadSplitEnrollments(t, s, result.NewTemplateID)
		var protected *activitiesModels.StudentEnrollment
		for _, row := range rows {
			if row.EnrollmentRequestChildID != nil && *row.EnrollmentRequestChildID == childID {
				protected = row
				break
			}
		}
		require.NotNil(t, protected)
		require.NotNil(t, protected.CalendarPeriodID)
		assert.Equal(t, periodB.ID, *protected.CalendarPeriodID)
		assert.Equal(t, []int{activitiesModels.WeekdayMonday}, protected.SelectedWeekdays)

		materialized, err := s.svc.MaterializeForTenant(s.ctx, effective, effective, scheduleSvc.MaterializationSourceManual)
		require.NoError(t, err)
		require.Equal(t, 1, materialized.InstancesCreated)
		instances := listInstancesForDate(t, s.db, result.NewTemplateID, effective)
		require.Len(t, instances, 1)
		s.registerCleanup("schedule.activity_instances", instances[0].ID)
		assert.True(t, splitInstanceContainsStudent(t, s, instances[0].ID, s.students[2]))
	})

	t.Run("update A to B", func(t *testing.T) {
		s := makeScenario(t, activitiesModels.WeekdayMonday, effective)
		defer s.runCleanup(t)
		phaseStart, phaseEnd := effective, effective.AddDays(21)
		offering := createLinkedCareOffering(t, s, phaseStart, phaseEnd)
		childID := createSplitRequestChildInPhase(t, s, offering.PhaseID, s.students[2])
		protectedUntil := phaseEnd.AddDays(1)
		protected := createProtectedSplitEnrollment(t, s, childID, s.students[2], phaseStart, protectedUntil, s.period.ID)
		periodB := createCompatibleCalendarPeriod(t, s)

		err := s.factory.TimetableData.UpdateTemplate(s.ctx, linkedTemplateUpdateInput(t, s, &periodB.ID))
		require.NoError(t, err)
		reloaded := loadSplitEnrollments(t, s, s.template.ID)
		var protectedAfter *activitiesModels.StudentEnrollment
		for _, row := range reloaded {
			if row.ID == protected.ID {
				protectedAfter = row
				break
			}
		}
		require.NotNil(t, protectedAfter)
		require.NotNil(t, protectedAfter.CalendarPeriodID)
		assert.Equal(t, periodB.ID, *protectedAfter.CalendarPeriodID)
		assert.Equal(t, []int{activitiesModels.WeekdayMonday}, protectedAfter.SelectedWeekdays)

		materialized, err := s.svc.MaterializeForTenant(s.ctx, effective, effective, scheduleSvc.MaterializationSourceManual)
		require.NoError(t, err)
		require.Equal(t, 1, materialized.InstancesCreated)
		instances := listInstancesForDate(t, s.db, s.template.ID, effective)
		require.Len(t, instances, 1)
		s.registerCleanup("schedule.activity_instances", instances[0].ID)
		assert.True(t, splitInstanceContainsStudent(t, s, instances[0].ID, s.students[2]))
	})
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
	successor := reloadSplitGroup(t, s, res.NewTemplateID)
	require.NotNil(t, successor.SeriesRootID)
	assert.Nil(t, successor.PlanningTrackID)
	assert.Equal(t, s.template.ID, *successor.SeriesRootID,
		"successor must retain the stable source-series identity")

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

func TestTemplateSplitKeepsArchivedPlanningTrackAssignment(t *testing.T) {
	effective := futureMonday(1)
	s := makeScenario(t, activitiesModels.WeekdayMonday, effective)
	defer s.runCleanup(t)

	track, err := s.factory.PlanningTracks.CreatePlanningTrack(s.ctx, scheduleSvc.PlanningTrackInput{
		Name: "Bestehende Spur", Color: "#5080D8", SortOrder: 0,
	})
	require.NoError(t, err)
	_, err = s.db.NewUpdate().Table("activities.groups").
		Set("planning_track_id = ?", track.ID).
		Where("tenant_id = ?", s.tenantID).
		Where("id = ?", s.template.ID).
		Exec(s.ctx)
	require.NoError(t, err)
	_, err = s.factory.PlanningTracks.ArchivePlanningTrack(s.ctx, track.ID)
	require.NoError(t, err)

	in := baseSplitInput(s, effective, fmt.Sprintf("Split-Track-%d", time.Now().UnixNano()))
	in.PlanningTrackID = &track.ID
	result, err := s.factory.TemplateSplit.Split(s.ctx, in)
	require.NoError(t, err)
	registerSuccessorCleanup(t, s, result.NewTemplateID)

	predecessor := reloadSplitGroup(t, s, result.OldTemplateID)
	successor := reloadSplitGroup(t, s, result.NewTemplateID)
	require.NotNil(t, predecessor.PlanningTrackID)
	require.NotNil(t, successor.PlanningTrackID)
	assert.Equal(t, track.ID, *predecessor.PlanningTrackID)
	assert.Equal(t, track.ID, *successor.PlanningTrackID)
}

func TestTemplateEndFromDate_CapsTemplateAndProtectsHistory(t *testing.T) {
	effective := futureMonday(1)
	secondMonday := effective.AddDays(7)
	s := makeScenario(t, activitiesModels.WeekdayMonday, effective)
	defer s.runCleanup(t)
	suffix := time.Now().UnixNano()

	r0, err := s.svc.MaterializeForTenant(s.ctx, effective, secondMonday, scheduleSvc.MaterializationSourceManual)
	require.NoError(t, err)
	require.Equal(t, 2, r0.InstancesCreated)

	first := listInstancesForDate(t, s.db, s.template.ID, effective)
	require.Len(t, first, 1)
	second := listInstancesForDate(t, s.db, s.template.ID, secondMonday)
	require.Len(t, second, 1)
	s.registerCleanup("schedule.activity_instances", first[0].ID, second[0].ID)

	_, err = s.db.NewUpdate().
		Model((*scheduleModels.ActivityInstance)(nil)).
		ModelTableExpr(`schedule.activity_instances AS "activity_instance"`).
		Set("status = ?", scheduleModels.InstanceStatusActive).
		Where(`"activity_instance".id = ?`, first[0].ID).
		Where(`"activity_instance".tenant_id = ?`, s.tenantID).
		Exec(s.ctx)
	require.NoError(t, err)

	completedID := splitInsertInstance(t, s, &s.template.ID, effective, scheduleModels.InstanceStatusCompleted, false, 9)
	cancelledID := splitInsertInstance(t, s, &s.template.ID, effective, scheduleModels.InstanceStatusCancelled, false, 10)
	spontaneousID := splitInsertInstance(t, s, &s.template.ID, effective, scheduleModels.InstanceStatusPlanned, true, 11)

	roomID := s.roomID
	otherGroup := &activitiesModels.Group{
		Name:            fmt.Sprintf("End-Fremd-%d", suffix),
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

	res, err := s.factory.TemplateSplit.EndFromDate(s.ctx, scheduleSvc.TemplateEndInput{
		TemplateID:    s.template.ID,
		EffectiveDate: effective,
	})
	require.NoError(t, err)

	assert.Equal(t, s.template.ID, res.TemplateID)
	assert.Equal(t, effective, res.EffectiveDate)
	assert.Equal(t, 1, res.DeletedInstances, "only still-planned non-spontaneous old-template rows are deleted")
	assert.EqualValues(t, 1, res.CappedSchedules)
	assert.EqualValues(t, 2, res.CappedEnrollments)
	assert.EqualValues(t, 1, res.CappedSupervisors)

	oldSchedule := reloadSplitSchedule(t, s, s.schedule.ID)
	require.NotNil(t, oldSchedule.ValidUntil)
	assert.Equal(t, effective, *oldSchedule.ValidUntil)

	oldEnrollments := loadSplitEnrollments(t, s, s.template.ID)
	expiredUntil := effective.AddDays(-1)
	for _, e := range oldEnrollments {
		require.NotNil(t, e.ValidUntil)
		if e.StudentID == s.students[2] {
			assert.Equal(t, expiredUntil, *e.ValidUntil)
		} else {
			assert.Equal(t, effective, *e.ValidUntil)
		}
	}
	oldSupervisors := loadSplitSupervisors(t, s, s.template.ID)
	require.Len(t, oldSupervisors, 1)
	require.NotNil(t, oldSupervisors[0].ValidUntil)
	assert.Equal(t, effective, *oldSupervisors[0].ValidUntil)

	assert.True(t, splitInstanceExists(t, s, first[0].ID), "active instance survives")
	assert.False(t, splitInstanceExists(t, s, second[0].ID), "planned future old-template instance deleted")
	assert.True(t, splitInstanceExists(t, s, completedID), "completed instance survives")
	assert.True(t, splitInstanceExists(t, s, cancelledID), "cancelled instance survives")
	assert.True(t, splitInstanceExists(t, s, spontaneousID), "spontaneous instance survives")
	assert.True(t, splitInstanceExists(t, s, otherPlannedID), "other group's planned instance survives")

	r1, err := s.svc.MaterializeForTenant(s.ctx, effective, secondMonday, scheduleSvc.MaterializationSourceManual)
	require.NoError(t, err)
	assert.Zero(t, r1.InstancesCreated, "ended template must not re-create future appointments")
	assert.Equal(t, 2, r1.CandidatesSkippedEnded)
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

// Editing either side of a split series must preserve its segment boundary.
// This is the end-to-end regression for the duplicate reported after editing
// the complete successor series: the predecessor still owns the prior Monday,
// while the successor starts exactly at the split Monday and never backfills.
func TestTemplateSplit_UpdateSegmentsPreservesBoundsDuringMaterialization(t *testing.T) {
	effective := futureMonday(1)
	previousMonday := effective.AddDays(-7)
	s := makeScenario(t, activitiesModels.WeekdayMonday, effective)
	defer s.runCleanup(t)

	requestChildID := createSplitRequestChild(t, s, s.students[2])
	sourcedFrom := effective.AddDays(-10)
	sourcedUntil := effective.AddDays(20)
	sourced := &activitiesModels.StudentEnrollment{
		StudentID:                s.students[2],
		ActivityGroupID:          s.template.ID,
		ValidFrom:                sourcedFrom,
		ValidUntil:               &sourcedUntil,
		EnrollmentRequestChildID: &requestChildID,
		SelectedWeekdays:         []int{activitiesModels.WeekdayMonday, activitiesModels.WeekdayWednesday},
	}
	sourced.SetTenantID(s.tenantID)
	_, err := s.db.NewInsert().Model(sourced).ModelTableExpr(`activities.student_enrollments`).Exec(s.ctx)
	require.NoError(t, err)
	s.registerCleanup("activities.student_enrollments", sourced.ID)

	futureRosterStart := effective.AddDays(5)
	weekdaySpecific := &activitiesModels.StudentEnrollment{
		StudentID:        s.students[2],
		ActivityGroupID:  s.template.ID,
		ValidFrom:        futureRosterStart,
		SelectedWeekdays: []int{activitiesModels.WeekdayMonday, activitiesModels.WeekdayWednesday},
	}
	weekdaySpecific.SetTenantID(s.tenantID)
	_, err = s.db.NewInsert().Model(weekdaySpecific).ModelTableExpr(`activities.student_enrollments`).Exec(s.ctx)
	require.NoError(t, err)
	s.registerCleanup("activities.student_enrollments", weekdaySpecific.ID)

	in := baseSplitInput(s, effective, fmt.Sprintf("Split-Update-Bounds-%d", time.Now().UnixNano()))
	in.CalendarPeriodID = &s.period.ID
	res, err := s.factory.TemplateSplit.Split(s.ctx, in)
	require.NoError(t, err)
	registerSuccessorCleanup(t, s, res.NewTemplateID)

	oldGroup := reloadSplitGroup(t, s, s.template.ID)
	require.NotNil(t, oldGroup.PlannedRoomID)
	err = s.factory.TimetableData.UpdateTemplate(s.ctx, scheduleSvc.TemplateUpdateInput{
		TemplateID: s.template.ID,
		Fields: activitiesModels.TemplateFieldsUpdate{
			Name:              oldGroup.Name + "-edited",
			Type:              oldGroup.Type,
			CategoryID:        oldGroup.CategoryID,
			RoomID:            *oldGroup.PlannedRoomID,
			EducationGroupID:  oldGroup.EducationGroupID,
			MaxParticipants:   oldGroup.MaxParticipants,
			CalendarPeriodID:  oldGroup.CalendarPeriodID,
			TargetGroupType:   oldGroup.TargetGroupType,
			TargetGradeLevel:  oldGroup.TargetGradeLevel,
			TargetSchoolClass: oldGroup.TargetSchoolClass,
		},
		Weekdays:        []int{activitiesModels.WeekdayMonday},
		TimeframeID:     s.timeframe.ID,
		WeekPattern:     s.schedule.WeekPattern,
		RosterValidFrom: effective.AddDays(-30),
		StudentIDs:      []int64{s.students[0], s.students[1]},
		StaffIDs:        []int64{s.staffID},
		PrimaryStaffID:  &s.staffID,
		GradeLevelMax:   schoolclass.MaxGradeLevel,
	})
	require.ErrorIs(t, err, scheduleSvc.ErrTemplateSegmentNotEditable)
	assert.Equal(t, oldGroup.Name, reloadSplitGroup(t, s, s.template.ID).Name,
		"bounded predecessor fields must remain unchanged")

	successorGroup := reloadSplitGroup(t, s, res.NewTemplateID)
	require.NotNil(t, successorGroup.PlannedRoomID)
	err = s.factory.TimetableData.UpdateTemplate(s.ctx, scheduleSvc.TemplateUpdateInput{
		TemplateID: res.NewTemplateID,
		Fields: activitiesModels.TemplateFieldsUpdate{
			Name:              successorGroup.Name + "-edited",
			Type:              successorGroup.Type,
			CategoryID:        successorGroup.CategoryID,
			RoomID:            *successorGroup.PlannedRoomID,
			EducationGroupID:  successorGroup.EducationGroupID,
			MaxParticipants:   successorGroup.MaxParticipants,
			CalendarPeriodID:  successorGroup.CalendarPeriodID,
			TargetGroupType:   successorGroup.TargetGroupType,
			TargetGradeLevel:  successorGroup.TargetGradeLevel,
			TargetSchoolClass: successorGroup.TargetSchoolClass,
		},
		Weekdays:         []int{activitiesModels.WeekdayMonday},
		TimeframeID:      s.timeframe.ID,
		WeekPattern:      s.schedule.WeekPattern,
		CalendarPeriodID: successorGroup.CalendarPeriodID,
		RosterValidFrom:  effective.AddDays(-30),
		StudentIDs:       []int64{s.students[0], s.students[1], s.students[2]},
		StaffIDs:         []int64{s.staffID},
		PrimaryStaffID:   &s.staffID,
		GradeLevelMax:    schoolclass.MaxGradeLevel,
	})
	require.NoError(t, err)

	oldSchedules := loadSplitSchedules(t, s, s.template.ID)
	require.Len(t, oldSchedules, 1)
	assert.Nil(t, oldSchedules[0].ValidFrom)
	require.NotNil(t, oldSchedules[0].ValidUntil)
	assert.Equal(t, effective, *oldSchedules[0].ValidUntil,
		"editing the predecessor must not reopen it past the split")

	successorSchedules := loadSplitSchedules(t, s, res.NewTemplateID)
	require.Len(t, successorSchedules, 1)
	require.NotNil(t, successorSchedules[0].ValidFrom)
	assert.Equal(t, effective, *successorSchedules[0].ValidFrom,
		"editing the successor must not backfill before the split")
	assert.Nil(t, successorSchedules[0].ValidUntil)

	oldReplacementFrom := effective.AddDays(-30)
	assertPredecessorRosterBounds(t, s, oldReplacementFrom, effective)
	assertPreservedSourcedEnrollment(t, s, sourced, requestChildID, sourcedFrom, sourcedUntil)
	assertSuccessorStudentRoster(t, s, res.NewTemplateID, effective, futureRosterStart, weekdaySpecific, sourced, requestChildID, sourcedUntil)
	assertSuccessorStaffRoster(t, s, res.NewTemplateID, effective)

	_, err = s.svc.MaterializeForTenant(s.ctx, previousMonday, effective, scheduleSvc.MaterializationSourceManual)
	require.NoError(t, err)
	registerSplitInstancesForCleanup(t, s, []int64{s.template.ID, res.NewTemplateID}, []timezone.Date{previousMonday, effective})

	assert.Len(t, listInstancesForDate(t, s.db, s.template.ID, previousMonday), 1)
	assert.Empty(t, listInstancesForDate(t, s.db, res.NewTemplateID, previousMonday),
		"successor must not duplicate the predecessor before valid_from")
	assert.Empty(t, listInstancesForDate(t, s.db, s.template.ID, effective),
		"predecessor must stop at its exclusive valid_until")
	assert.Len(t, listInstancesForDate(t, s.db, res.NewTemplateID, effective), 1)
}

func assertPredecessorRosterBounds(
	t *testing.T,
	s *scenarioSetup,
	replacementFrom, effective timezone.Date,
) {
	t.Helper()
	replacementStudents := 0
	for _, enrollment := range loadSplitEnrollments(t, s, s.template.ID) {
		if enrollment.EnrollmentRequestChildID != nil || len(enrollment.SelectedWeekdays) > 0 {
			if enrollment.ValidUntil != nil {
				assert.False(t, enrollment.ValidUntil.Before(enrollment.ValidFrom),
					"protected predecessor enrollment bounds must never invert")
			}
			continue
		}
		require.NotNil(t, enrollment.ValidUntil, "editing the predecessor must not create an open enrollment")
		assert.False(t, enrollment.ValidUntil.Before(enrollment.ValidFrom), "predecessor enrollment bounds must never invert")
		if enrollment.ValidFrom == replacementFrom && *enrollment.ValidUntil == effective {
			replacementStudents++
		}
	}
	assert.Equal(t, 2, replacementStudents, "the selected predecessor roster must inherit the schedule end")

	replacementStaff := 0
	for _, supervisor := range loadSplitSupervisors(t, s, s.template.ID) {
		require.NotNil(t, supervisor.ValidUntil, "editing the predecessor must not create open supervision")
		assert.False(t, supervisor.ValidUntil.Before(supervisor.ValidFrom), "predecessor supervision bounds must never invert")
		if supervisor.ValidFrom == replacementFrom && *supervisor.ValidUntil == effective {
			replacementStaff++
		}
	}
	assert.Equal(t, 1, replacementStaff)
}

func assertPreservedSourcedEnrollment(
	t *testing.T,
	s *scenarioSetup,
	source *activitiesModels.StudentEnrollment,
	requestChildID int64,
	wantFrom, wantUntil timezone.Date,
) {
	t.Helper()
	var preserved activitiesModels.StudentEnrollment
	require.NoError(t, s.db.NewSelect().Model(&preserved).
		ModelTableExpr(`activities.student_enrollments AS "student_enrollment"`).
		Where(`"student_enrollment".id = ?`, source.ID).
		Where(`"student_enrollment".tenant_id = ?`, s.tenantID).
		Scan(s.ctx))
	require.NotNil(t, preserved.EnrollmentRequestChildID)
	assert.Equal(t, requestChildID, *preserved.EnrollmentRequestChildID,
		"template PUT must preserve enrollment-decision provenance")
	assert.Equal(t, wantFrom, preserved.ValidFrom)
	require.NotNil(t, preserved.ValidUntil)
	assert.Equal(t, wantUntil, *preserved.ValidUntil,
		"template PUT must not truncate a hidden bounded care-offer window")
	assert.Equal(t, source.SelectedWeekdays, preserved.SelectedWeekdays)
}

func assertSuccessorStudentRoster(
	t *testing.T,
	s *scenarioSetup,
	groupID int64,
	effective, futureStart timezone.Date,
	weekdaySpecific, sourced *activitiesModels.StudentEnrollment,
	requestChildID int64,
	sourcedUntil timezone.Date,
) {
	t.Helper()
	rows := loadSplitEnrollments(t, s, groupID)
	assertActiveSuccessorStudents(t, s, rows, effective, futureStart)
	assertProtectedSuccessorRows(t, s, rows, futureStart, weekdaySpecific)
	assertCarriedSourcedRows(t, rows, effective, sourced, requestChildID, sourcedUntil)
}

func assertActiveSuccessorStudents(
	t *testing.T,
	s *scenarioSetup,
	rows []*activitiesModels.StudentEnrollment,
	effective, futureStart timezone.Date,
) {
	t.Helper()
	active := 0
	for _, enrollment := range rows {
		if enrollment.ValidUntil != nil {
			assert.False(t, enrollment.ValidUntil.Before(enrollment.ValidFrom), "retired successor enrollment bounds must never invert")
			continue
		}
		active++
		if enrollment.StudentID == s.students[2] && len(enrollment.SelectedWeekdays) > 0 {
			assert.Equal(t, futureStart, enrollment.ValidFrom, "future protected roster must retain its later start")
			continue
		}
		assert.Equal(t, effective, enrollment.ValidFrom, "successor manual enrollment must start at schedule valid_from")
	}
	assert.Equal(t, 3, active)
}

func assertProtectedSuccessorRows(
	t *testing.T,
	s *scenarioSetup,
	rows []*activitiesModels.StudentEnrollment,
	futureStart timezone.Date,
	source *activitiesModels.StudentEnrollment,
) {
	t.Helper()
	protected := 0
	for _, enrollment := range rows {
		if enrollment.StudentID != s.students[2] || len(enrollment.SelectedWeekdays) == 0 || enrollment.EnrollmentRequestChildID != nil {
			continue
		}
		protected++
		assert.Equal(t, futureStart, enrollment.ValidFrom, "future weekday-specific roster must not be pulled forward")
		assert.Equal(t, source.SelectedWeekdays, enrollment.SelectedWeekdays)
	}
	assert.Equal(t, 1, protected, "explicit StudentIDs must not duplicate or widen the protected carried row")
}

func assertCarriedSourcedRows(
	t *testing.T,
	rows []*activitiesModels.StudentEnrollment,
	effective timezone.Date,
	source *activitiesModels.StudentEnrollment,
	requestChildID int64,
	sourcedUntil timezone.Date,
) {
	t.Helper()
	carried := 0
	for _, enrollment := range rows {
		if enrollment.EnrollmentRequestChildID == nil || *enrollment.EnrollmentRequestChildID != requestChildID {
			continue
		}
		carried++
		assert.Equal(t, effective, enrollment.ValidFrom, "sourced row must be clipped to the successor start")
		require.NotNil(t, enrollment.ValidUntil)
		assert.Equal(t, sourcedUntil, *enrollment.ValidUntil)
		assert.Equal(t, source.SelectedWeekdays, enrollment.SelectedWeekdays)
	}
	assert.Equal(t, 1, carried, "split must carry a hidden decision-owned enrollment independently of explicit StudentIDs")
}

func assertSuccessorStaffRoster(t *testing.T, s *scenarioSetup, groupID int64, effective timezone.Date) {
	t.Helper()
	active := 0
	for _, supervisor := range loadSplitSupervisors(t, s, groupID) {
		if supervisor.ValidUntil != nil {
			assert.False(t, supervisor.ValidUntil.Before(supervisor.ValidFrom), "retired successor supervision bounds must never invert")
			continue
		}
		active++
		assert.Equal(t, effective, supervisor.ValidFrom, "successor supervision must start at schedule valid_from")
	}
	assert.Equal(t, 1, active)
}

func registerSplitInstancesForCleanup(t *testing.T, s *scenarioSetup, groupIDs []int64, dates []timezone.Date) {
	t.Helper()
	for _, groupID := range groupIDs {
		for _, date := range dates {
			for _, instance := range listInstancesForDate(t, s.db, groupID, date) {
				s.registerCleanup("schedule.activity_instances", instance.ID)
			}
		}
	}
}

// Re-splitting the bounded predecessor [a,b) at e must produce [a,e) and
// [e,b), not an open middle segment that overlaps the already-existing
// successor [b,∞). Materializing beyond b must therefore yield exactly one
// occurrence from the original later successor.
func TestTemplateSplit_RejectsResplittingBoundedPredecessor(t *testing.T) {
	outerBoundary := futureMonday(3)
	innerBoundary := outerBoundary.AddDays(-7)
	afterOuter := outerBoundary.AddDays(7)
	s := makeScenario(t, activitiesModels.WeekdayMonday, innerBoundary)
	defer s.runCleanup(t)

	first, err := s.factory.TemplateSplit.Split(s.ctx,
		baseSplitInput(s, outerBoundary, fmt.Sprintf("Split-Later-%d", time.Now().UnixNano())))
	require.NoError(t, err)
	registerSuccessorCleanup(t, s, first.NewTemplateID)

	_, err = s.factory.TemplateSplit.Split(s.ctx,
		baseSplitInput(s, innerBoundary, fmt.Sprintf("Split-Middle-%d", time.Now().UnixNano())))
	require.ErrorIs(t, err, scheduleSvc.ErrSplitInvalidInput)

	oldSchedules := loadSplitSchedules(t, s, s.template.ID)
	require.Len(t, oldSchedules, 1)
	require.NotNil(t, oldSchedules[0].ValidUntil)
	assert.Equal(t, outerBoundary, *oldSchedules[0].ValidUntil,
		"rejected re-split must not shorten the predecessor")

	materialized, err := s.svc.MaterializeForTenant(
		s.ctx, outerBoundary, afterOuter, scheduleSvc.MaterializationSourceManual)
	require.NoError(t, err)
	assert.Equal(t, 2, materialized.InstancesCreated)
	registerSplitInstancesForCleanup(t, s,
		[]int64{s.template.ID, first.NewTemplateID},
		[]timezone.Date{outerBoundary, afterOuter})

	for _, date := range []timezone.Date{outerBoundary, afterOuter} {
		assert.Empty(t, listInstancesForDate(t, s.db, s.template.ID, date))
		assert.Len(t, listInstancesForDate(t, s.db, first.NewTemplateID, date), 1,
			"the original open successor remains the only owner after a rejected re-split")
	}
}

func TestTemplateSplitAndEnd_RespectCurrentSegmentEnvelope(t *testing.T) {
	boundary := futureMonday(2)
	beforeBoundary := boundary.AddDays(-1)
	futureRosterStart := boundary.AddDays(5)
	s := makeScenario(t, activitiesModels.WeekdayMonday, boundary)
	defer s.runCleanup(t)

	futureEnrollment := &activitiesModels.StudentEnrollment{
		StudentID:        s.students[0],
		ActivityGroupID:  s.template.ID,
		ValidFrom:        futureRosterStart,
		CalendarPeriodID: &s.period.ID,
	}
	futureEnrollment.SetTenantID(s.tenantID)
	_, err := s.db.NewInsert().Model(futureEnrollment).
		ModelTableExpr(`activities.student_enrollments`).ExcludeColumn("selected_weekdays").Exec(s.ctx)
	require.NoError(t, err)
	s.registerCleanup("activities.student_enrollments", futureEnrollment.ID)

	futureSupervisor := &activitiesModels.SupervisorPlanned{
		StaffID:          s.staffID,
		GroupID:          s.template.ID,
		ValidFrom:        futureRosterStart,
		CalendarPeriodID: &s.period.ID,
	}
	futureSupervisor.SetTenantID(s.tenantID)
	_, err = s.db.NewInsert().Model(futureSupervisor).ModelTableExpr(`activities.supervisors`).Exec(s.ctx)
	require.NoError(t, err)
	s.registerCleanup("activities.supervisors", futureSupervisor.ID)

	firstInput := baseSplitInput(s, boundary, fmt.Sprintf("Split-Envelope-%d", time.Now().UnixNano()))
	firstInput.CalendarPeriodID = &s.period.ID
	first, err := s.factory.TemplateSplit.Split(s.ctx, firstInput)
	require.NoError(t, err)
	registerSuccessorCleanup(t, s, first.NewTemplateID)

	assertFutureRosterCarried(t, s, first.NewTemplateID, futureRosterStart)
	assertTemplateRosterWindowsNotInverted(t, s, s.template.ID)

	t.Run("split rejects before successor start", func(t *testing.T) {
		in := baseSplitInput(s, beforeBoundary, fmt.Sprintf("Split-Too-Early-%d", time.Now().UnixNano()))
		in.TemplateID = first.NewTemplateID
		_, err := s.factory.TemplateSplit.Split(s.ctx, in)
		require.ErrorIs(t, err, scheduleSvc.ErrSplitInvalidInput)
	})

	t.Run("split and end reject predecessor exclusive end", func(t *testing.T) {
		in := baseSplitInput(s, boundary, fmt.Sprintf("Split-At-End-%d", time.Now().UnixNano()))
		_, splitErr := s.factory.TemplateSplit.Split(s.ctx, in)
		require.ErrorIs(t, splitErr, scheduleSvc.ErrSplitInvalidInput)
		_, endErr := s.factory.TemplateSplit.EndFromDate(s.ctx, scheduleSvc.TemplateEndInput{
			TemplateID:    s.template.ID,
			EffectiveDate: boundary,
		})
		require.ErrorIs(t, endErr, scheduleSvc.ErrSplitInvalidInput)
	})

	t.Run("end before future successor clamps to its start", func(t *testing.T) {
		ended, err := s.factory.TemplateSplit.EndFromDate(s.ctx, scheduleSvc.TemplateEndInput{
			TemplateID:    first.NewTemplateID,
			EffectiveDate: beforeBoundary,
		})
		require.NoError(t, err)
		assert.Equal(t, boundary, ended.EffectiveDate)

		schedules := loadSplitSchedules(t, s, first.NewTemplateID)
		require.Len(t, schedules, 1)
		require.NotNil(t, schedules[0].ValidFrom)
		require.NotNil(t, schedules[0].ValidUntil)
		assert.Equal(t, boundary, *schedules[0].ValidFrom)
		assert.Equal(t, boundary, *schedules[0].ValidUntil)
		assertTemplateRosterWindowsNotInverted(t, s, first.NewTemplateID)
	})
}

func assertFutureRosterCarried(t *testing.T, s *scenarioSetup, groupID int64, wantStart timezone.Date) {
	t.Helper()
	carriedStudent := false
	for _, enrollment := range loadSplitEnrollments(t, s, groupID) {
		if enrollment.StudentID == s.students[0] {
			carriedStudent = true
			assert.Equal(t, wantStart, enrollment.ValidFrom)
		}
	}
	assert.True(t, carriedStudent)
	staff := loadSplitSupervisors(t, s, groupID)
	require.Len(t, staff, 1)
	assert.Equal(t, wantStart, staff[0].ValidFrom,
		"future-starting open supervision must retain its start on the successor")
}

func assertTemplateRosterWindowsNotInverted(t *testing.T, s *scenarioSetup, groupID int64) {
	t.Helper()
	for _, enrollment := range loadSplitEnrollments(t, s, groupID) {
		if enrollment.ValidUntil != nil {
			assert.False(t, enrollment.ValidUntil.Before(enrollment.ValidFrom))
		}
	}
	for _, supervisor := range loadSplitSupervisors(t, s, groupID) {
		if supervisor.ValidUntil != nil {
			assert.False(t, supervisor.ValidUntil.Before(supervisor.ValidFrom))
		}
	}
}

// Mirrors the reported UI sequence end to end: edit one materialized week,
// split from the following occurrence, edit the complete successor segment,
// then re-plan across both dates. The moved single occurrence and its
// exception must not be joined by a backfilled successor occurrence.
func TestTemplateSplit_SingleEditThenSuccessorUpdateDoesNotDuplicate(t *testing.T) {
	effective := futureMonday(2)
	singleDate := effective.AddDays(-7)
	s := makeScenario(t, activitiesModels.WeekdayMonday, singleDate)
	defer s.runCleanup(t)

	materialized, err := s.svc.MaterializeForTenant(s.ctx, singleDate, singleDate, scheduleSvc.MaterializationSourceManual)
	require.NoError(t, err)
	require.Equal(t, 1, materialized.InstancesCreated)
	instances := listInstancesForDate(t, s.db, s.template.ID, singleDate)
	require.Len(t, instances, 1)
	single := instances[0]
	s.registerCleanup("schedule.activity_instances", single.ID)

	// "Nur diese Woche": move the start by one hour. UpdatePlanned keeps the
	// instance on this date and writes a cancellation exception for the old
	// template slot so a re-plan cannot resurrect the original time.
	updatedSingle, err := s.factory.Instance.UpdatePlanned(
		s.ctx,
		single.ID,
		moveInput(s, single, singleDate, 1),
		nil,
	)
	require.NoError(t, err)
	assert.Equal(t, single.ID, updatedSingle.ID)
	exceptions := loadExceptions(t, s, s.template.ID)
	require.Len(t, exceptions, 1)
	assert.Equal(t, singleDate, exceptions[0].ExceptionDate)
	assert.Equal(t, scheduleModels.ActivityExceptionCancelled, exceptions[0].ExceptionType)

	// "Ab jetzt dauerhaft": split at the next Monday.
	in := baseSplitInput(s, effective, fmt.Sprintf("Split-Single-Following-%d", time.Now().UnixNano()))
	res, err := s.factory.TemplateSplit.Split(s.ctx, in)
	require.NoError(t, err)
	registerSuccessorCleanup(t, s, res.NewTemplateID)

	// "Alle Termine der Serie" on the successor: a full schedule replacement
	// must retain valid_from=effective.
	successor := reloadSplitGroup(t, s, res.NewTemplateID)
	require.NotNil(t, successor.PlannedRoomID)
	err = s.factory.TimetableData.UpdateTemplate(s.ctx, scheduleSvc.TemplateUpdateInput{
		TemplateID: res.NewTemplateID,
		Fields: activitiesModels.TemplateFieldsUpdate{
			Name:              successor.Name + "-all-edited",
			Type:              successor.Type,
			CategoryID:        successor.CategoryID,
			RoomID:            *successor.PlannedRoomID,
			EducationGroupID:  successor.EducationGroupID,
			MaxParticipants:   successor.MaxParticipants,
			CalendarPeriodID:  successor.CalendarPeriodID,
			TargetGroupType:   successor.TargetGroupType,
			TargetGradeLevel:  successor.TargetGradeLevel,
			TargetSchoolClass: successor.TargetSchoolClass,
		},
		Weekdays:        []int{activitiesModels.WeekdayMonday},
		TimeframeID:     s.timeframe.ID,
		WeekPattern:     s.schedule.WeekPattern,
		RosterValidFrom: singleDate.AddDays(-30),
		StudentIDs:      []int64{s.students[0], s.students[1]},
		StaffIDs:        []int64{s.staffID},
		PrimaryStaffID:  &s.staffID,
		GradeLevelMax:   schoolclass.MaxGradeLevel,
	})
	require.NoError(t, err)
	successorSchedules := loadSplitSchedules(t, s, res.NewTemplateID)
	require.Len(t, successorSchedules, 1)
	require.NotNil(t, successorSchedules[0].ValidFrom)
	assert.Equal(t, effective, *successorSchedules[0].ValidFrom)

	// The frontend follows a full-series PUT with a successor-scoped re-plan.
	// Materialization still examines every template, making this the exact
	// point where an erased valid_from used to create the duplicate.
	result, err := s.factory.Instance.ReplanWeek(s.ctx, singleDate, effective, &res.NewTemplateID, nil)
	require.NoError(t, err)
	require.NotNil(t, result.Materialization)
	registerSplitInstancesForCleanup(t, s,
		[]int64{s.template.ID, res.NewTemplateID},
		[]timezone.Date{singleDate, effective})

	oldSingle := listInstancesForDate(t, s.db, s.template.ID, singleDate)
	require.Len(t, oldSingle, 1, "the explicitly edited week remains")
	assert.Equal(t, single.ID, oldSingle[0].ID)
	assert.Empty(t, listInstancesForDate(t, s.db, res.NewTemplateID, singleDate),
		"the successor must not backfill next to the single-week edit")
	assert.Empty(t, listInstancesForDate(t, s.db, s.template.ID, effective))
	assert.Len(t, listInstancesForDate(t, s.db, res.NewTemplateID, effective), 1)
}

// Forces the stale-envelope race: EndFromDate caps the schedules but holds
// its transaction open while PUT starts concurrently. PUT must wait on the
// shared tenant recurrence gate, then reject the now-bounded segment instead
// of mutating a template the active CRUD contract no longer exposes.
func TestTemplateEnd_ConcurrentTemplateUpdatePreservesCommittedCap(t *testing.T) {
	effective := futureMonday(1)
	s := makeScenario(t, activitiesModels.WeekdayMonday, effective)
	defer s.runCleanup(t)

	group := reloadSplitGroup(t, s, s.template.ID)
	require.NotNil(t, group.PlannedRoomID)
	update := scheduleSvc.TemplateUpdateInput{
		TemplateID: s.template.ID,
		Fields: activitiesModels.TemplateFieldsUpdate{
			Name:              group.Name + "-concurrent-edit",
			Type:              group.Type,
			CategoryID:        group.CategoryID,
			RoomID:            *group.PlannedRoomID,
			EducationGroupID:  group.EducationGroupID,
			MaxParticipants:   group.MaxParticipants,
			CalendarPeriodID:  group.CalendarPeriodID,
			TargetGroupType:   group.TargetGroupType,
			TargetGradeLevel:  group.TargetGradeLevel,
			TargetSchoolClass: group.TargetSchoolClass,
		},
		Weekdays:        []int{activitiesModels.WeekdayMonday},
		TimeframeID:     s.timeframe.ID,
		WeekPattern:     s.schedule.WeekPattern,
		RosterValidFrom: effective.AddDays(-30),
		StudentIDs:      []int64{s.students[0], s.students[1]},
		StaffIDs:        []int64{s.staffID},
		PrimaryStaffID:  &s.staffID,
		GradeLevelMax:   schoolclass.MaxGradeLevel,
	}

	endMutationComplete := make(chan struct{})
	releaseEnd := make(chan struct{})
	endDone := make(chan error, 1)
	var releaseOnce sync.Once
	release := func() { releaseOnce.Do(func() { close(releaseEnd) }) }
	defer release()

	go func() {
		endDone <- tenant.WithTenantTx(s.ctx, s.db, s.tenantID, func(txCtx context.Context, _ bun.Tx) error {
			_, err := s.factory.TemplateSplit.EndFromDate(txCtx, scheduleSvc.TemplateEndInput{
				TemplateID:    s.template.ID,
				EffectiveDate: effective,
			})
			if err != nil {
				return err
			}
			close(endMutationComplete)
			<-releaseEnd // keep the cap uncommitted and the advisory lock held
			return nil
		})
	}()

	select {
	case <-endMutationComplete:
	case err := <-endDone:
		require.NoError(t, err)
		t.Fatal("end transaction completed before the concurrency barrier")
	case <-time.After(5 * time.Second):
		release()
		t.Fatal("timed out waiting for end mutation")
	}

	updateStarted := make(chan struct{})
	updateDone := make(chan error, 1)
	go func() {
		close(updateStarted)
		updateDone <- s.factory.TimetableData.UpdateTemplate(s.ctx, update)
	}()
	<-updateStarted

	// Give the concurrent PUT enough time to reach the contested template.
	// Without the shared lock it reads NULL here and later recreates stale open
	// rows after blocking on DELETE; with the lock it cannot read yet.
	updateFinishedEarly := false
	select {
	case <-updateDone:
		updateFinishedEarly = true
	case <-time.After(250 * time.Millisecond):
	}

	release()
	select {
	case err := <-endDone:
		require.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for end transaction to commit")
	}
	if updateFinishedEarly {
		t.Fatal("template update completed while the end transaction held the recurrence lock")
	}
	select {
	case err := <-updateDone:
		require.ErrorIs(t, err, scheduleSvc.ErrTemplateSegmentNotEditable)
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for serialized template update")
	}

	schedules := loadSplitSchedules(t, s, s.template.ID)
	require.Len(t, schedules, 1)
	require.NotNil(t, schedules[0].ValidUntil)
	assert.Equal(t, effective, *schedules[0].ValidUntil,
		"rejected PUT must preserve the cap committed by the preceding end transaction")
	assert.Equal(t, group.Name, reloadSplitGroup(t, s, s.template.ID).Name,
		"rejected PUT must not mutate bounded template fields")
}

// The inverse side of the same gate: materialization must not read an old
// open schedule while EndFromDate's cap is uncommitted, then insert a stale
// predecessor occurrence after the end operation has already performed its
// future-instance delete.
func TestTemplateEnd_ConcurrentMaterializationCannotInsertPastCommittedCap(t *testing.T) {
	effective := futureMonday(1)
	s := makeScenario(t, activitiesModels.WeekdayMonday, effective)
	defer s.runCleanup(t)

	endMutationComplete := make(chan struct{})
	releaseEnd := make(chan struct{})
	endDone := make(chan error, 1)
	var releaseOnce sync.Once
	release := func() { releaseOnce.Do(func() { close(releaseEnd) }) }
	defer release()

	go func() {
		endDone <- tenant.WithTenantTx(s.ctx, s.db, s.tenantID, func(txCtx context.Context, _ bun.Tx) error {
			_, err := s.factory.TemplateSplit.EndFromDate(txCtx, scheduleSvc.TemplateEndInput{
				TemplateID:    s.template.ID,
				EffectiveDate: effective,
			})
			if err != nil {
				return err
			}
			close(endMutationComplete)
			<-releaseEnd
			return nil
		})
	}()

	select {
	case <-endMutationComplete:
	case err := <-endDone:
		require.NoError(t, err)
		t.Fatal("end transaction completed before the concurrency barrier")
	case <-time.After(5 * time.Second):
		release()
		t.Fatal("timed out waiting for end mutation")
	}

	type materializeOutcome struct {
		result *scheduleSvc.MaterializationResult
		err    error
	}
	materializeStarted := make(chan struct{})
	materializeDone := make(chan materializeOutcome, 1)
	go func() {
		close(materializeStarted)
		result, err := s.svc.MaterializeForTenant(s.ctx, effective, effective, scheduleSvc.MaterializationSourceManual)
		materializeDone <- materializeOutcome{result: result, err: err}
	}()
	<-materializeStarted

	var early materializeOutcome
	materializedEarly := false
	select {
	case early = <-materializeDone:
		materializedEarly = true
	case <-time.After(250 * time.Millisecond):
	}

	release()
	select {
	case err := <-endDone:
		require.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for end transaction to commit")
	}
	if materializedEarly {
		require.NoError(t, early.err)
		t.Fatal("materialization completed while the end transaction held the recurrence gate")
	}

	var outcome materializeOutcome
	select {
	case outcome = <-materializeDone:
		require.NoError(t, outcome.err)
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for serialized materialization")
	}
	require.NotNil(t, outcome.result)
	assert.Zero(t, outcome.result.InstancesCreated)
	assert.Equal(t, 1, outcome.result.CandidatesSkippedEnded)
	instances := listInstancesForDate(t, s.db, s.template.ID, effective)
	for _, instance := range instances {
		s.registerCleanup("schedule.activity_instances", instance.ID)
	}
	assert.Empty(t, instances, "no predecessor occurrence may survive on/after the committed cap")
}

// Archival is also a recurrence writer: once archived_at commits, a
// materializer that started concurrently must not insert an occurrence from a
// template snapshot taken before the archive.
func TestTemplateArchive_ConcurrentMaterializationCannotInsertStaleOccurrence(t *testing.T) {
	date := futureMonday(1)
	s := makeScenario(t, activitiesModels.WeekdayMonday, date)
	defer s.runCleanup(t)

	archiveMutationComplete := make(chan struct{})
	releaseArchive := make(chan struct{})
	archiveDone := make(chan error, 1)
	var releaseOnce sync.Once
	release := func() { releaseOnce.Do(func() { close(releaseArchive) }) }
	defer release()

	go func() {
		archiveDone <- tenant.WithTenantTx(s.ctx, s.db, s.tenantID, func(txCtx context.Context, _ bun.Tx) error {
			archived, err := s.factory.TimetableData.ArchiveTemplate(txCtx, s.template.ID)
			if err != nil {
				return err
			}
			if archived != 1 {
				return fmt.Errorf("expected one archived template, got %d", archived)
			}
			close(archiveMutationComplete)
			<-releaseArchive
			return nil
		})
	}()

	select {
	case <-archiveMutationComplete:
	case err := <-archiveDone:
		require.NoError(t, err)
		t.Fatal("archive transaction completed before the concurrency barrier")
	case <-time.After(5 * time.Second):
		release()
		t.Fatal("timed out waiting for archive mutation")
	}

	type materializeOutcome struct {
		result *scheduleSvc.MaterializationResult
		err    error
	}
	materializeStarted := make(chan struct{})
	materializeDone := make(chan materializeOutcome, 1)
	go func() {
		close(materializeStarted)
		result, err := s.svc.MaterializeForTenant(s.ctx, date, date, scheduleSvc.MaterializationSourceManual)
		materializeDone <- materializeOutcome{result: result, err: err}
	}()
	<-materializeStarted

	var early materializeOutcome
	materializedEarly := false
	select {
	case early = <-materializeDone:
		materializedEarly = true
	case <-time.After(250 * time.Millisecond):
	}

	release()
	select {
	case err := <-archiveDone:
		require.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for archive transaction to commit")
	}
	if materializedEarly {
		require.NoError(t, early.err)
		t.Fatal("materialization completed while archive held the recurrence gate")
	}

	var outcome materializeOutcome
	select {
	case outcome = <-materializeDone:
		require.NoError(t, outcome.err)
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for serialized materialization")
	}
	require.NotNil(t, outcome.result)
	assert.Zero(t, outcome.result.InstancesCreated)
	require.Len(t, outcome.result.Warnings, 1)
	assert.Equal(t, scheduleSvc.MaterializationWarningCodeNoTemplates, outcome.result.Warnings[0].Code)
	instances := listInstancesForDate(t, s.db, s.template.ID, date)
	for _, instance := range instances {
		s.registerCleanup("schedule.activity_instances", instance.ID)
	}
	assert.Empty(t, instances, "archived template must not produce a stale occurrence")
}

// Multi-period protected rosters retain each period/weekday assignment. Plain
// supervisor rows still collapse to one successor row per person — previously
// the carry path stamped every active row with the successor's period id and
// violated the active-row uniqueness indexes (500 on split).
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
	require.Len(t, newEnrollments, 3, "both protected period/weekday rows plus the other manual student survive")
	byStudent := map[int64][]*activitiesModels.StudentEnrollment{}
	for _, e := range newEnrollments {
		byStudent[e.StudentID] = append(byStudent[e.StudentID], e)
	}
	require.Contains(t, byStudent, s.students[0])
	require.Contains(t, byStudent, s.students[1])
	require.Len(t, byStudent[s.students[0]], 2)
	weekdaysByPeriod := make(map[int64][]int)
	for _, enrollment := range byStudent[s.students[0]] {
		periodID := int64(0)
		if enrollment.CalendarPeriodID != nil {
			periodID = *enrollment.CalendarPeriodID
		}
		weekdaysByPeriod[periodID] = enrollment.SelectedWeekdays
	}
	assert.Equal(t, []int{activitiesModels.WeekdayMonday}, weekdaysByPeriod[0])
	assert.Equal(t, []int{activitiesModels.WeekdayWednesday}, weekdaysByPeriod[s.period.ID])
	require.Len(t, byStudent[s.students[1]], 1)

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

// reloadSplitGroup fetches one activities.groups row by id.
func reloadSplitGroup(t *testing.T, s *scenarioSetup, id int64) *activitiesModels.Group {
	t.Helper()
	var row activitiesModels.Group
	err := s.db.NewSelect().Model(&row).
		ModelTableExpr(`activities.groups AS "group"`).
		Where(`"group".id = ?`, id).
		Where(`"group".tenant_id = ?`, s.tenantID).
		Scan(s.ctx)
	require.NoError(t, err)
	return &row
}

func TestTemplateSplit_CarriesTargetGroupAndCalendarPeriodToSuccessor(t *testing.T) {
	effective := futureMonday(1)
	s := makeScenario(t, activitiesModels.WeekdayMonday, effective)
	defer s.runCleanup(t)
	suffix := time.Now().UnixNano()

	gradeLevel := int16(3)
	in := baseSplitInput(s, effective, fmt.Sprintf("Split-Zielgruppe-%d", suffix))
	in.CalendarPeriodID = &s.period.ID
	in.TargetGroupType = activitiesModels.TargetGroupTypeJahrgang
	in.TargetGradeLevel = &gradeLevel

	res, err := s.factory.TemplateSplit.Split(s.ctx, in)
	require.NoError(t, err)
	registerSuccessorCleanup(t, s, res.NewTemplateID)

	successor := reloadSplitGroup(t, s, res.NewTemplateID)
	require.NotNil(t, successor.CalendarPeriodID)
	assert.Equal(t, s.period.ID, *successor.CalendarPeriodID, "successor Group carries the period pin, not just its schedule rows")
	assert.Equal(t, activitiesModels.TargetGroupTypeJahrgang, successor.TargetGroupType)
	require.NotNil(t, successor.TargetGradeLevel)
	assert.Equal(t, gradeLevel, *successor.TargetGradeLevel)
	assert.Nil(t, successor.TargetSchoolClass)

	// The successor's own schedule rows are ALSO stamped with the period —
	// both the template-level and schedule-level pins are set, per the
	// two-tier fallback in materialization_service.go's schedulePinnedPeriodID.
	newSchedules := loadSplitSchedules(t, s, res.NewTemplateID)
	require.Len(t, newSchedules, 1)
	require.NotNil(t, newSchedules[0].CalendarPeriodID)
	assert.Equal(t, s.period.ID, *newSchedules[0].CalendarPeriodID)
}

func TestTemplateSplit_SecondSuccessorKeepsOriginalSeriesRoot(t *testing.T) {
	firstBoundary := futureMonday(1)
	secondBoundary := futureMonday(2)
	s := makeScenario(t, activitiesModels.WeekdayMonday, firstBoundary)
	defer s.runCleanup(t)

	firstInput := baseSplitInput(s, firstBoundary, fmt.Sprintf("Split-Series-First-%d", time.Now().UnixNano()))
	first, err := s.factory.TemplateSplit.Split(s.ctx, firstInput)
	require.NoError(t, err)
	registerSuccessorCleanup(t, s, first.NewTemplateID)

	secondInput := baseSplitInput(s, secondBoundary, fmt.Sprintf("Split-Series-Second-%d", time.Now().UnixNano()))
	secondInput.TemplateID = first.NewTemplateID
	second, err := s.factory.TemplateSplit.Split(s.ctx, secondInput)
	require.NoError(t, err)
	registerSuccessorCleanup(t, s, second.NewTemplateID)

	secondSuccessor := reloadSplitGroup(t, s, second.NewTemplateID)
	require.NotNil(t, secondSuccessor.SeriesRootID)
	assert.Equal(t, s.template.ID, *secondSuccessor.SeriesRootID)
	series, err := repositories.NewFactory(s.db).ActivityGroup.FindTemplateSeries(s.ctx, second.NewTemplateID)
	require.NoError(t, err)
	assert.Len(t, series, 3)
}

func TestTemplateSplit_ValidationErrors_RejectsInvalidTargetGroup(t *testing.T) {
	effective := futureMonday(1)
	s := makeScenario(t, activitiesModels.WeekdayMonday, effective)
	defer s.runCleanup(t)

	in := baseSplitInput(s, effective, fmt.Sprintf("Split-BadZielgruppe-%d", time.Now().UnixNano()))
	in.TargetGroupType = activitiesModels.TargetGroupTypeJahrgang
	// TargetGradeLevel intentionally left nil — jahrgang requires it.

	_, err := s.factory.TemplateSplit.Split(s.ctx, in)
	require.Error(t, err)
	assert.ErrorIs(t, err, scheduleSvc.ErrSplitInvalidInput)
}

// setSourceTemplateNotes sets the Wochennotiz on the scenario's source template
// so split carry-over behavior can be asserted (#1837 follow-up).
func setSourceTemplateNotes(t *testing.T, s *scenarioSetup, notes string) {
	t.Helper()
	_, err := s.db.NewUpdate().
		Model((*activitiesModels.Group)(nil)).
		ModelTableExpr(`activities.groups AS "group"`).
		Set("notes = ?", notes).
		Where(`"group".id = ?`, s.template.ID).
		Where(`"group".tenant_id = ?`, s.tenantID).
		Exec(s.ctx)
	require.NoError(t, err)
}

// TestTemplateSplit_CarriesWochennotiz verifies the durable series note is
// inherited by the successor when omitted, overwritten when provided, and
// cleared when explicitly nulled (#1837 follow-up).
func TestTemplateSplit_CarriesWochennotiz(t *testing.T) {
	t.Run("omitted note inherits the source template's Wochennotiz", func(t *testing.T) {
		effective := futureMonday(1)
		s := makeScenario(t, activitiesModels.WeekdayMonday, effective)
		defer s.runCleanup(t)
		setSourceTemplateNotes(t, s, "Raum erst ab 14 Uhr offen")

		in := baseSplitInput(s, effective, fmt.Sprintf("Split-InheritNote-%d", time.Now().UnixNano()))
		// NotesProvided stays false → inherit.
		res, err := s.factory.TemplateSplit.Split(s.ctx, in)
		require.NoError(t, err)
		registerSuccessorCleanup(t, s, res.NewTemplateID)

		successor := reloadSplitGroup(t, s, res.NewTemplateID)
		require.NotNil(t, successor.Notes, "successor must inherit the source note")
		assert.Equal(t, "Raum erst ab 14 Uhr offen", *successor.Notes)
	})

	t.Run("provided note overwrites on the successor", func(t *testing.T) {
		effective := futureMonday(1)
		s := makeScenario(t, activitiesModels.WeekdayMonday, effective)
		defer s.runCleanup(t)
		setSourceTemplateNotes(t, s, "Alt")

		newNote := "Ab jetzt: Turnhalle"
		in := baseSplitInput(s, effective, fmt.Sprintf("Split-SetNote-%d", time.Now().UnixNano()))
		in.Notes = &newNote
		in.NotesProvided = true
		res, err := s.factory.TemplateSplit.Split(s.ctx, in)
		require.NoError(t, err)
		registerSuccessorCleanup(t, s, res.NewTemplateID)

		successor := reloadSplitGroup(t, s, res.NewTemplateID)
		require.NotNil(t, successor.Notes)
		assert.Equal(t, newNote, *successor.Notes)
	})

	t.Run("explicit null clears the note on the successor", func(t *testing.T) {
		effective := futureMonday(1)
		s := makeScenario(t, activitiesModels.WeekdayMonday, effective)
		defer s.runCleanup(t)
		setSourceTemplateNotes(t, s, "Alt")

		in := baseSplitInput(s, effective, fmt.Sprintf("Split-ClearNote-%d", time.Now().UnixNano()))
		in.Notes = nil
		in.NotesProvided = true
		res, err := s.factory.TemplateSplit.Split(s.ctx, in)
		require.NoError(t, err)
		registerSuccessorCleanup(t, s, res.NewTemplateID)

		successor := reloadSplitGroup(t, s, res.NewTemplateID)
		assert.Nil(t, successor.Notes, "explicit null must clear the successor note")
	})
}
