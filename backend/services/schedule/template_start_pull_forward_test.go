package schedule_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activitiesModels "github.com/moto-nrw/project-phoenix/models/activities"
	scheduleSvc "github.com/moto-nrw/project-phoenix/services/schedule"
)

// Issue #2226: a series that has not started yet can be pulled forward to an
// earlier start date within its calendar period. The schedule envelope and the
// series-managed roster move to the new valid_from together, and only the
// occurrences newly possible before the old start materialize — existing
// instances stay untouched.

// futureMondayForStartPull returns a Monday at least eight days in the future,
// so both the old and the pulled-forward start stay strictly after today on
// every weekday the suite runs.
func futureMondayForStartPull(t *testing.T) timezone.Date {
	t.Helper()
	today := timezone.TodayDate()
	daysAhead := (int(time.Monday) - int(today.Weekday()) + 7) % 7
	if daysAhead == 0 {
		daysAhead = 7
	}
	return today.AddDays(daysAhead + 7)
}

// startPullUpdateInput builds the minimal PUT-shaped update the pull-forward
// tests reuse: same recurrence and roster as the created template, plus the
// requested new series start.
func startPullUpdateInput(
	s *weekdayRosterScenario,
	templateID, timeframeID int64,
	name string,
	weekdays []int,
	startDate *timezone.Date,
) scheduleSvc.TemplateUpdateInput {
	return scheduleSvc.TemplateUpdateInput{
		TemplateID: templateID,
		Fields: activitiesModels.TemplateFieldsUpdate{
			Name:            name,
			Type:            activitiesModels.GroupTypeCare,
			CategoryID:      s.categoryID,
			RoomID:          s.roomID,
			MaxParticipants: 20,
			TargetGroupType: activitiesModels.TargetGroupTypeNone,
		},
		Weekdays:        weekdays,
		TimeframeID:     timeframeID,
		RosterValidFrom: timezone.TodayDate(),
		GradeLevelMax:   4,
		StaffIDs:        []int64{s.staffA},
		PrimaryStaffID:  &s.staffA,
		StudentIDs:      []int64{s.studentA},
		StartDate:       startDate,
	}
}

func TestUpdateTemplate_StartDatePullForward_MovesEnvelopeRosterAndMaterializesGapOnly(t *testing.T) {
	t.Parallel()

	newStart := futureMondayForStartPull(t)
	oldStart := newStart.AddDays(7)

	s := makeWeekdayRosterScenario(t, newStart)
	defer s.teardown(t)

	name := fmt.Sprintf("PullForward-%d", time.Now().UnixNano())
	weekdays := []int{activitiesModels.WeekdayMonday, activitiesModels.WeekdayTuesday}
	result, err := s.factory.TimetableData.CreateTemplate(s.ctx, scheduleSvc.CreateTemplateInput{
		Name:              name,
		Type:              activitiesModels.GroupTypeCare,
		Weekdays:          weekdays,
		StartTime:         clockTime(14, 0),
		EndTime:           clockTime(15, 0),
		RoomID:            s.roomID,
		CategoryID:        s.categoryID,
		MaxParticipants:   20,
		RosterValidFrom:   oldStart,
		GradeLevelMax:     4,
		StaffIDs:          []int64{s.staffA},
		PrimaryStaffID:    &s.staffA,
		StudentIDs:        []int64{s.studentA},
		ScheduleValidFrom: &oldStart,
	})
	require.NoError(t, err)
	s.registerTemplate(t, result.TemplateID, result.TimeframeID)

	// The old-start week is already materialized before the edit.
	materialized, err := s.materialize.MaterializeForTenant(s.ctx, oldStart, oldStart.AddDays(1), scheduleSvc.MaterializationSourceManual)
	require.NoError(t, err)
	require.Equal(t, 2, materialized.InstancesCreated)
	existingIDs := s.templateInstanceIDs(t, result.TemplateID)
	require.Len(t, existingIDs, 2)

	require.NoError(t, s.factory.TimetableData.UpdateTemplate(
		s.ctx,
		startPullUpdateInput(s, result.TemplateID, result.TimeframeID, name, weekdays, &newStart),
	))

	// Every weekday schedule row carries the new inclusive start.
	var scheduleFroms []string
	require.NoError(t, s.db.NewRaw(
		`SELECT valid_from::text FROM activities.schedules WHERE activity_group_id = ? AND tenant_id = ?`,
		result.TemplateID, s.tenantID,
	).Scan(s.ctx, &scheduleFroms))
	require.Len(t, scheduleFroms, 2)
	for _, from := range scheduleFroms {
		assert.Equal(t, newStart.String(), from, "schedule valid_from must move to the new series start")
	}

	// The series-managed roster follows the same boundary.
	for _, row := range s.openSupervisorRows(t, result.TemplateID) {
		assert.Equal(t, newStart, row.ValidFrom, "supervisor valid_from must follow the new start")
	}
	openEnrollments := 0
	for _, row := range s.enrollmentRows(t, result.TemplateID) {
		if row.ValidUntil != nil {
			continue
		}
		openEnrollments++
		assert.Equal(t, newStart, row.ValidFrom, "enrollment valid_from must follow the new start")
	}
	require.Positive(t, openEnrollments)

	// Re-planning the widened window creates exactly the two new occurrences
	// in front of the old start and rewrites nothing that already exists.
	materialized, err = s.materialize.MaterializeForTenant(s.ctx, newStart, oldStart.AddDays(1), scheduleSvc.MaterializationSourceManual)
	require.NoError(t, err)
	assert.Equal(t, 2, materialized.InstancesCreated, "only the gap occurrences may be created")

	afterIDs := s.templateInstanceIDs(t, result.TemplateID)
	assert.Len(t, afterIDs, 4)
	for _, id := range existingIDs {
		assert.Contains(t, afterIDs, id, "existing instances must survive unchanged")
	}
}

// AC 4 (#2226): weekday-scoped roster rows (per-weekday staffing, #2129)
// follow the pulled-forward start exactly like the shared roster.
func TestUpdateTemplate_StartDatePullForward_MovesWeekdayScopedRoster(t *testing.T) {
	t.Parallel()

	newStart := futureMondayForStartPull(t)
	oldStart := newStart.AddDays(7)

	s := makeWeekdayRosterScenario(t, newStart)
	defer s.teardown(t)

	name := fmt.Sprintf("PullForwardWeekday-%d", time.Now().UnixNano())
	weekdays := []int{activitiesModels.WeekdayMonday, activitiesModels.WeekdayTuesday}
	assignments := []scheduleSvc.WeekdayRosterAssignment{
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
	}
	result, err := s.factory.TimetableData.CreateTemplate(s.ctx, scheduleSvc.CreateTemplateInput{
		Name:               name,
		Type:               activitiesModels.GroupTypeCare,
		Weekdays:           weekdays,
		StartTime:          clockTime(14, 0),
		EndTime:            clockTime(15, 0),
		RoomID:             s.roomID,
		CategoryID:         s.categoryID,
		MaxParticipants:    20,
		RosterValidFrom:    oldStart,
		GradeLevelMax:      4,
		WeekdayAssignments: assignments,
		ScheduleValidFrom:  &oldStart,
	})
	require.NoError(t, err)
	s.registerTemplate(t, result.TemplateID, result.TimeframeID)

	in := startPullUpdateInput(s, result.TemplateID, result.TimeframeID, name, weekdays, &newStart)
	in.StaffIDs = nil
	in.PrimaryStaffID = nil
	in.StudentIDs = nil
	in.WeekdayAssignments = assignments
	require.NoError(t, s.factory.TimetableData.UpdateTemplate(s.ctx, in))

	for _, row := range s.openSupervisorRows(t, result.TemplateID) {
		require.NotNil(t, row.Weekday, "per-weekday mode keeps weekday-scoped staff rows")
		assert.Equal(t, newStart, row.ValidFrom,
			"weekday-scoped supervisor rows must follow the new start")
	}
	openEnrollments := 0
	for _, row := range s.enrollmentRows(t, result.TemplateID) {
		if row.ValidUntil != nil {
			continue
		}
		openEnrollments++
		require.NotNil(t, row.Weekday)
		assert.Equal(t, newStart, row.ValidFrom,
			"weekday-scoped enrollment rows must follow the new start")
	}
	require.Equal(t, 2, openEnrollments)

	// The pulled-forward week materializes each weekday with its own people.
	_, err = s.materialize.MaterializeForTenant(s.ctx, newStart, newStart.AddDays(1), scheduleSvc.MaterializationSourceManual)
	require.NoError(t, err)
	assert.Equal(t, []int64{s.staffA}, s.instanceStaffIDs(t, s.singleInstance(t, result.TemplateID, newStart)))
	assert.Equal(t, []int64{s.staffB}, s.instanceStaffIDs(t, s.singleInstance(t, result.TemplateID, newStart.AddDays(1))))
}

func TestUpdateTemplate_StartDatePullForward_MidweekDateStartsAtNextMatchingWeekday(t *testing.T) {
	t.Parallel()

	newStart := futureMondayForStartPull(t) // a Monday
	oldStart := newStart.AddDays(8)         // Tuesday one week later

	s := makeWeekdayRosterScenario(t, newStart)
	defer s.teardown(t)

	name := fmt.Sprintf("PullForwardMidweek-%d", time.Now().UnixNano())
	weekdays := []int{activitiesModels.WeekdayTuesday}
	result, err := s.factory.TimetableData.CreateTemplate(s.ctx, scheduleSvc.CreateTemplateInput{
		Name:              name,
		Type:              activitiesModels.GroupTypeCare,
		Weekdays:          weekdays,
		StartTime:         clockTime(14, 0),
		EndTime:           clockTime(15, 0),
		RoomID:            s.roomID,
		CategoryID:        s.categoryID,
		MaxParticipants:   20,
		RosterValidFrom:   oldStart,
		GradeLevelMax:     4,
		StaffIDs:          []int64{s.staffA},
		PrimaryStaffID:    &s.staffA,
		StudentIDs:        []int64{s.studentA},
		ScheduleValidFrom: &oldStart,
	})
	require.NoError(t, err)
	s.registerTemplate(t, result.TemplateID, result.TimeframeID)

	// Pull the start to a Monday even though the series only runs on Tuesdays.
	require.NoError(t, s.factory.TimetableData.UpdateTemplate(
		s.ctx,
		startPullUpdateInput(s, result.TemplateID, result.TimeframeID, name, weekdays, &newStart),
	))

	_, err = s.materialize.MaterializeForTenant(s.ctx, newStart, oldStart, scheduleSvc.MaterializationSourceManual)
	require.NoError(t, err)

	assert.Empty(t, listInstancesForDate(t, s.db, result.TemplateID, newStart),
		"no occurrence may appear on the non-scheduled Monday")
	firstTuesday := newStart.AddDays(1)
	assert.Len(t, listInstancesForDate(t, s.db, result.TemplateID, firstTuesday), 1,
		"the first extra occurrence lands on the next matching weekday")
}

func TestUpdateTemplate_StartDatePullForward_Validation(t *testing.T) {
	t.Parallel()

	newStart := futureMondayForStartPull(t)
	oldStart := newStart.AddDays(7)

	s := makeWeekdayRosterScenario(t, newStart)
	defer s.teardown(t)

	weekdays := []int{activitiesModels.WeekdayMonday}
	name := fmt.Sprintf("PullForwardValidation-%d", time.Now().UnixNano())
	result, err := s.factory.TimetableData.CreateTemplate(s.ctx, scheduleSvc.CreateTemplateInput{
		Name:              name,
		Type:              activitiesModels.GroupTypeCare,
		Weekdays:          weekdays,
		StartTime:         clockTime(14, 0),
		EndTime:           clockTime(15, 0),
		RoomID:            s.roomID,
		CategoryID:        s.categoryID,
		MaxParticipants:   20,
		RosterValidFrom:   oldStart,
		GradeLevelMax:     4,
		StaffIDs:          []int64{s.staffA},
		PrimaryStaffID:    &s.staffA,
		StudentIDs:        []int64{s.studentA},
		ScheduleValidFrom: &oldStart,
	})
	require.NoError(t, err)
	s.registerTemplate(t, result.TemplateID, result.TimeframeID)

	later := oldStart.AddDays(7)
	err = s.factory.TimetableData.UpdateTemplate(
		s.ctx,
		startPullUpdateInput(s, result.TemplateID, result.TimeframeID, name, weekdays, &later),
	)
	require.ErrorIs(t, err, scheduleSvc.ErrTemplateStartNotEarlier,
		"moving the start later is out of scope and must be rejected")

	past := timezone.TodayDate().AddDays(-1)
	err = s.factory.TimetableData.UpdateTemplate(
		s.ctx,
		startPullUpdateInput(s, result.TemplateID, result.TimeframeID, name, weekdays, &past),
	)
	require.ErrorIs(t, err, scheduleSvc.ErrTemplateStartInPast,
		"a start in the past must be rejected")

	// Equal to the stored start: a no-op, not an error — idempotent retries of
	// the same PUT must keep succeeding.
	require.NoError(t, s.factory.TimetableData.UpdateTemplate(
		s.ctx,
		startPullUpdateInput(s, result.TemplateID, result.TimeframeID, name, weekdays, &oldStart),
	))

	// A series without a stored start begins with its period — there is
	// nothing in front of that to pull forward to.
	openName := fmt.Sprintf("PullForwardOpenStart-%d", time.Now().UnixNano())
	openResult, err := s.factory.TimetableData.CreateTemplate(s.ctx, scheduleSvc.CreateTemplateInput{
		Name:            openName,
		Type:            activitiesModels.GroupTypeCare,
		Weekdays:        weekdays,
		StartTime:       clockTime(15, 0),
		EndTime:         clockTime(16, 0),
		RoomID:          s.roomID,
		CategoryID:      s.categoryID,
		MaxParticipants: 20,
		RosterValidFrom: timezone.TodayDate(),
		GradeLevelMax:   4,
		StaffIDs:        []int64{s.staffA},
		PrimaryStaffID:  &s.staffA,
		StudentIDs:      []int64{s.studentA},
	})
	require.NoError(t, err)
	s.registerTemplate(t, openResult.TemplateID, openResult.TimeframeID)

	err = s.factory.TimetableData.UpdateTemplate(
		s.ctx,
		startPullUpdateInput(s, openResult.TemplateID, openResult.TimeframeID, openName, weekdays, &newStart),
	)
	require.ErrorIs(t, err, scheduleSvc.ErrTemplateStartNotEarlier)
}

func TestUpdateTemplate_StartDatePullForward_RejectsPredecessorOverlap(t *testing.T) {
	t.Parallel()

	newStart := futureMondayForStartPull(t)
	splitDate := newStart.AddDays(14)

	s := makeWeekdayRosterScenario(t, newStart)
	defer s.teardown(t)

	weekdays := []int{activitiesModels.WeekdayMonday}
	predecessorName := fmt.Sprintf("PullForwardPredecessor-%d", time.Now().UnixNano())
	predecessor, err := s.factory.TimetableData.CreateTemplate(s.ctx, scheduleSvc.CreateTemplateInput{
		Name:            predecessorName,
		Type:            activitiesModels.GroupTypeCare,
		Weekdays:        weekdays,
		StartTime:       clockTime(14, 0),
		EndTime:         clockTime(15, 0),
		RoomID:          s.roomID,
		CategoryID:      s.categoryID,
		MaxParticipants: 20,
		RosterValidFrom: timezone.TodayDate(),
		GradeLevelMax:   4,
		StaffIDs:        []int64{s.staffA},
		PrimaryStaffID:  &s.staffA,
		StudentIDs:      []int64{s.studentA},
	})
	require.NoError(t, err)
	s.registerTemplate(t, predecessor.TemplateID, predecessor.TimeframeID)

	successorName := fmt.Sprintf("PullForwardSuccessor-%d", time.Now().UnixNano())
	successor, err := s.factory.TimetableData.CreateTemplate(s.ctx, scheduleSvc.CreateTemplateInput{
		Name:              successorName,
		Type:              activitiesModels.GroupTypeCare,
		Weekdays:          weekdays,
		StartTime:         clockTime(16, 0),
		EndTime:           clockTime(17, 0),
		RoomID:            s.roomID,
		CategoryID:        s.categoryID,
		MaxParticipants:   20,
		RosterValidFrom:   splitDate,
		GradeLevelMax:     4,
		StaffIDs:          []int64{s.staffA},
		PrimaryStaffID:    &s.staffA,
		StudentIDs:        []int64{s.studentA},
		ScheduleValidFrom: &splitDate,
	})
	require.NoError(t, err)
	s.registerTemplate(t, successor.TemplateID, successor.TimeframeID)

	// Shape the two templates into a split chain: the predecessor is capped at
	// the split date, the successor points at it as its series root.
	_, err = s.db.NewRaw(
		`UPDATE activities.groups SET series_root_id = ? WHERE id = ? AND tenant_id = ?`,
		predecessor.TemplateID, successor.TemplateID, s.tenantID,
	).Exec(s.ctx)
	require.NoError(t, err)
	_, err = s.db.NewRaw(
		`UPDATE activities.schedules SET valid_until = ? WHERE activity_group_id = ? AND tenant_id = ?`,
		splitDate, predecessor.TemplateID, s.tenantID,
	).Exec(s.ctx)
	require.NoError(t, err)

	overlapping := splitDate.AddDays(-7)
	err = s.factory.TimetableData.UpdateTemplate(
		s.ctx,
		startPullUpdateInput(s, successor.TemplateID, successor.TimeframeID, successorName, weekdays, &overlapping),
	)
	require.ErrorIs(t, err, scheduleSvc.ErrTemplateStartPredecessorOverlap,
		"the new start must not reach into the predecessor's window")

	// Once the predecessor ends earlier, the freed week becomes claimable.
	earlierCap := splitDate.AddDays(-7)
	_, err = s.db.NewRaw(
		`UPDATE activities.schedules SET valid_until = ? WHERE activity_group_id = ? AND tenant_id = ?`,
		earlierCap, predecessor.TemplateID, s.tenantID,
	).Exec(s.ctx)
	require.NoError(t, err)

	require.NoError(t, s.factory.TimetableData.UpdateTemplate(
		s.ctx,
		startPullUpdateInput(s, successor.TemplateID, successor.TimeframeID, successorName, weekdays, &overlapping),
	))

	var successorFroms []string
	require.NoError(t, s.db.NewRaw(
		`SELECT valid_from::text FROM activities.schedules WHERE activity_group_id = ? AND tenant_id = ?`,
		successor.TemplateID, s.tenantID,
	).Scan(s.ctx, &successorFroms))
	require.NotEmpty(t, successorFroms)
	for _, from := range successorFroms {
		assert.Equal(t, overlapping.String(), from)
	}
}
