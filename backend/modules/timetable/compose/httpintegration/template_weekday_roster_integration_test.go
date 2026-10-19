package httpintegration_test

import (
	"context"
	"fmt"
	"log/slog"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/timetable/fixturetest"

	activityContract "github.com/moto-nrw/project-phoenix/modules/timetable/activitycontract"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/api"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/modules/timetable/compose"
	scheduleModels "github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain/schedulerecords"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
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
	db              *bun.DB
	ctx             context.Context
	tenantID        int64
	instance        timetable.InstanceLifecycleCapability
	timetableData   compose.Planning
	active          studentpresence.Presence
	materialization timetable.MaterializationCapability
	careDay         careplan.CareDayQuery
	autoStart       timetable.InstanceAutoStart
	careOffering    careplan.CareOfferingCapability
	statusDays      studentpresence.StatusDays
	materialize     timetable.MaterializationCapability

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

func ownedStudentEnrollmentRepository(t *testing.T, db *bun.DB) fixturetest.EnrollmentWriter {
	t.Helper()
	repos := api.NewUnobservedTimetableRepositories(testpkg.MustDatabase(db))
	return repos.StudentEnrollment
}

func TestCreateTemplate_PerWeekdayRoster_MaterializesTheWeekdaysOwnPeople(t *testing.T) {
	t.Parallel()

	monday := calendar.NewDate(2026, time.April, 20)
	tuesday := monday.AddDays(1)

	s := makeWeekdayRosterScenario(t, monday)
	defer s.teardown(t)

	// Monday: Anna with Alice. Tuesday: Bea with Bob. Nothing shared.
	result, err := s.timetableData.Templates.CreateTemplate(s.ctx, timetable.CreateTemplateCommand{
		Name:            fmt.Sprintf("Lernzeit-%d", time.Now().UnixNano()),
		Type:            activityContract.GroupTypeCare,
		Weekdays:        []int{activityContract.WeekdayMonday, activityContract.WeekdayTuesday},
		StartTime:       clockTime(14, 0),
		EndTime:         clockTime(15, 0),
		RoomID:          s.roomID,
		CategoryID:      s.categoryID,
		MaxParticipants: 20,
		RosterValidFrom: monday.AddDays(-30),
		GradeLevelMax:   4,
		WeekdayAssignments: []timetable.WeekdayRosterAssignment{
			{
				Weekday:        activityContract.WeekdayMonday,
				StaffIDs:       []int64{s.staffA},
				PrimaryStaffID: &s.staffA,
				StudentIDs:     []int64{s.studentA},
			},
			{
				Weekday:        activityContract.WeekdayTuesday,
				StaffIDs:       []int64{s.staffB},
				PrimaryStaffID: &s.staffB,
				StudentIDs:     []int64{s.studentB},
			},
		},
	})
	require.NoError(t, err)
	s.registerTemplate(t, result.TemplateID, result.TimeframeID)

	materialized, err := s.materialize.MaterializeForTenant(s.ctx, monday, tuesday, timetable.MaterializationSourceManual)
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
	t.Parallel()

	monday := calendar.NewDate(2026, time.April, 20)
	tuesday := monday.AddDays(1)

	s := makeWeekdayRosterScenario(t, monday)
	defer s.teardown(t)

	result, err := s.timetableData.Templates.CreateTemplate(s.ctx, timetable.CreateTemplateCommand{
		Name:            fmt.Sprintf("Randstunde-%d", time.Now().UnixNano()),
		Type:            activityContract.GroupTypeCare,
		Weekdays:        []int{activityContract.WeekdayMonday, activityContract.WeekdayTuesday},
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
		WeekdayAssignments: []timetable.WeekdayRosterAssignment{{
			Weekday:        activityContract.WeekdayTuesday,
			StaffIDs:       []int64{s.staffB},
			PrimaryStaffID: &s.staffB,
			StudentIDs:     []int64{s.studentA, s.studentB},
		}},
	})
	require.NoError(t, err)
	s.registerTemplate(t, result.TemplateID, result.TimeframeID)

	_, err = s.materialize.MaterializeForTenant(s.ctx, monday, tuesday, timetable.MaterializationSourceManual)
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
	t.Parallel()

	monday := calendar.NewDate(2026, time.April, 20)
	tuesday := monday.AddDays(1)

	s := makeWeekdayRosterScenario(t, monday)
	defer s.teardown(t)

	result, err := s.timetableData.Templates.CreateTemplate(s.ctx, timetable.CreateTemplateCommand{
		Name:            fmt.Sprintf("Mensa-%d", time.Now().UnixNano()),
		Type:            activityContract.GroupTypeCare,
		Weekdays:        []int{activityContract.WeekdayMonday, activityContract.WeekdayTuesday},
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

	_, err = s.materialize.MaterializeForTenant(s.ctx, monday, tuesday, timetable.MaterializationSourceManual)
	require.NoError(t, err)

	for _, date := range []calendar.Date{monday, tuesday} {
		instance := s.singleInstance(t, result.TemplateID, date)
		assert.ElementsMatch(t, []int64{s.staffA, s.staffB}, s.instanceStaffIDs(t, instance))
		assert.ElementsMatch(t, []int64{s.studentA, s.studentB}, s.instanceStudentIDs(t, instance))
	}
}

func TestMaterialization_PrefersScopedPrimaryWithoutClearingLegacyFallback(t *testing.T) {
	t.Parallel()

	monday := calendar.NewDate(2026, time.April, 20)
	tuesday := monday.AddDays(1)
	s := makeWeekdayRosterScenario(t, monday)
	defer s.teardown(t)

	result, err := s.timetableData.Templates.CreateTemplate(s.ctx, timetable.CreateTemplateCommand{
		Name:            fmt.Sprintf("Primary-Scope-%d", time.Now().UnixNano()),
		Type:            activityContract.GroupTypeCare,
		Weekdays:        []int{activityContract.WeekdayMonday, activityContract.WeekdayTuesday},
		StartTime:       clockTime(14, 0),
		EndTime:         clockTime(15, 0),
		RoomID:          s.roomID,
		CategoryID:      s.categoryID,
		MaxParticipants: 20,
		RosterValidFrom: monday.AddDays(-30),
		GradeLevelMax:   4,
		StaffIDs:        []int64{s.staffA},
		PrimaryStaffID:  &s.staffA,
	})
	require.NoError(t, err)
	s.registerTemplate(t, result.TemplateID, result.TimeframeID)

	weekday := activityContract.WeekdayMonday
	repos := api.NewUnobservedTimetableRepositories(testpkg.MustDatabase(s.db))
	require.NoError(t, repos.ActivitySupervisor.Create(s.ctx, &activityContract.SupervisorPlanned{
		StaffID:   s.staffA,
		GroupID:   result.TemplateID,
		IsPrimary: true,
		ValidFrom: activityContract.Date(monday.AddDays(-30)),
		Weekday:   &weekday,
	}))
	require.NoError(t, repos.ActivitySupervisor.Create(s.ctx, &activityContract.SupervisorPlanned{
		StaffID:          s.staffB,
		GroupID:          result.TemplateID,
		IsPrimary:        true,
		ValidFrom:        activityContract.Date(monday.AddDays(-30)),
		CalendarPeriodID: &s.periodID,
		Weekday:          &weekday,
	}))

	openRows := s.openSupervisorRows(t, result.TemplateID)
	require.Len(t, openRows, 3)
	var globalWeekdayPrimary, periodWeekdayPrimary bool
	for _, row := range openRows {
		if row.Weekday == nil {
			continue
		}
		if row.CalendarPeriodID == nil {
			globalWeekdayPrimary = row.IsPrimary
		} else if *row.CalendarPeriodID == s.periodID {
			periodWeekdayPrimary = row.IsPrimary
		}
	}
	assert.True(t, globalWeekdayPrimary, "the trigger must preserve the global weekday fallback")
	assert.True(t, periodWeekdayPrimary, "the trigger must preserve the period-specific primary")

	rosterRows, err := repos.ActivityGroup.ListTemplateWeekdayRoster(
		s.ctx,
		&result.TemplateID,
		&s.periodID,
	)
	require.NoError(t, err)
	primaryByWeekdayAndStaff := make(map[int]map[int64]bool)
	for _, row := range rosterRows {
		if row.Kind == timetable.TemplateWeekdayRosterKindStaff {
			if primaryByWeekdayAndStaff[row.Weekday] == nil {
				primaryByWeekdayAndStaff[row.Weekday] = make(map[int64]bool)
			}
			primaryByWeekdayAndStaff[row.Weekday][row.PersonID] = row.IsPrimary
		}
	}
	assert.False(t, primaryByWeekdayAndStaff[activityContract.WeekdayMonday][s.staffA],
		"the editor read must not expose the shared fallback as effective primary")
	assert.True(t, primaryByWeekdayAndStaff[activityContract.WeekdayMonday][s.staffB],
		"the editor read must preserve the period-specific effective primary")
	assert.True(t, primaryByWeekdayAndStaff[activityContract.WeekdayTuesday][s.staffA],
		"the shared primary must remain effective outside the scoped override")

	_, err = s.materialize.MaterializeForTenant(s.ctx, monday, tuesday, timetable.MaterializationSourceManual)
	require.NoError(t, err)

	mondayInstance := s.singleInstance(t, result.TemplateID, monday)
	tuesdayInstance := s.singleInstance(t, result.TemplateID, tuesday)
	assert.ElementsMatch(t, []int64{s.staffA, s.staffB}, s.instanceStaffIDs(t, mondayInstance))
	assert.False(t, s.instanceHasPrimary(t, mondayInstance, s.staffA))
	assert.True(t, s.instanceHasPrimary(t, mondayInstance, s.staffB),
		"the scoped Monday primary must win over the shared fallback")
	assert.Equal(t, []int64{s.staffA}, s.instanceStaffIDs(t, tuesdayInstance))
	assert.True(t, s.instanceHasPrimary(t, tuesdayInstance, s.staffA),
		"the shared primary must remain responsible outside the override")
}

// Editing the series replaces the per-weekday roster wholesale, including
// dropping back to a single shared roster when the planner turns the
// per-weekday mode off again.
func TestUpdateTemplate_SwitchesBetweenSharedAndPerWeekdayRoster(t *testing.T) {
	t.Parallel()

	monday := calendar.NewDate(2026, time.April, 20)
	tuesday := monday.AddDays(1)

	s := makeWeekdayRosterScenario(t, monday)
	defer s.teardown(t)

	name := fmt.Sprintf("Lernzeit-Update-%d", time.Now().UnixNano())
	result, err := s.timetableData.Templates.CreateTemplate(s.ctx, timetable.CreateTemplateCommand{
		Name:            name,
		Type:            activityContract.GroupTypeCare,
		Weekdays:        []int{activityContract.WeekdayMonday, activityContract.WeekdayTuesday},
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

	updateInput := timetable.UpdateTemplateCommand{
		TemplateID: result.TemplateID,
		Fields: timetable.TemplateFields{
			Name:            name,
			Type:            activityContract.GroupTypeCare,
			CategoryID:      s.categoryID,
			RoomID:          s.roomID,
			MaxParticipants: 20,
			TargetGroupType: activityContract.TargetGroupTypeNone,
		},
		Weekdays:        []int{activityContract.WeekdayMonday, activityContract.WeekdayTuesday},
		TimeframeID:     result.TimeframeID,
		RosterValidFrom: monday.AddDays(-30),
		GradeLevelMax:   4,
		StaffIDs:        []int64{s.staffA},
		PrimaryStaffID:  &s.staffA,
		StudentIDs:      []int64{s.studentA},
		WeekdayAssignments: []timetable.WeekdayRosterAssignment{{
			Weekday:        activityContract.WeekdayTuesday,
			StaffIDs:       []int64{s.staffB},
			PrimaryStaffID: &s.staffB,
			StudentIDs:     []int64{s.studentB},
		}},
	}
	require.NoError(t, s.timetableData.Templates.UpdateTemplate(s.ctx, updateInput))

	_, err = s.materialize.MaterializeForTenant(s.ctx, monday, tuesday, timetable.MaterializationSourceManual)
	require.NoError(t, err)
	assert.Equal(t, []int64{s.staffA}, s.instanceStaffIDs(t, s.singleInstance(t, result.TemplateID, monday)))
	assert.Equal(t, []int64{s.staffB}, s.instanceStaffIDs(t, s.singleInstance(t, result.TemplateID, tuesday)))

	// Turn per-weekday staffing back off: every open roster row loses its scope.
	updateInput.WeekdayAssignments = nil
	updateInput.StaffIDs = []int64{s.staffA, s.staffB}
	require.NoError(t, s.timetableData.Templates.UpdateTemplate(s.ctx, updateInput))
	for _, row := range s.openSupervisorRows(t, result.TemplateID) {
		assert.Nil(t, row.Weekday, "resetting to a shared roster must clear the weekday scope")
	}
}

func TestUpdateTemplate_PinningPeriodRetiresUnscopedEditorRoster(t *testing.T) {
	t.Parallel()

	monday := calendar.NewDate(2026, time.April, 20)
	tuesday := monday.AddDays(1)
	s := makeWeekdayRosterScenario(t, monday)
	defer s.teardown(t)

	name := fmt.Sprintf("Pinned-Roster-%d", time.Now().UnixNano())
	result, err := s.timetableData.Templates.CreateTemplate(s.ctx, timetable.CreateTemplateCommand{
		Name:            name,
		Type:            activityContract.GroupTypeCare,
		Weekdays:        []int{activityContract.WeekdayMonday, activityContract.WeekdayTuesday},
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

	require.NoError(t, s.timetableData.Templates.UpdateTemplate(s.ctx, timetable.UpdateTemplateCommand{
		TemplateID: result.TemplateID,
		Fields: timetable.TemplateFields{
			Name:             name,
			Type:             activityContract.GroupTypeCare,
			CategoryID:       s.categoryID,
			RoomID:           s.roomID,
			MaxParticipants:  20,
			CalendarPeriodID: &s.periodID,
			TargetGroupType:  activityContract.TargetGroupTypeNone,
		},
		Weekdays:         []int{activityContract.WeekdayMonday, activityContract.WeekdayTuesday},
		TimeframeID:      result.TimeframeID,
		CalendarPeriodID: &s.periodID,
		RosterValidFrom:  monday.AddDays(-30),
		GradeLevelMax:    4,
		WeekdayAssignments: []timetable.WeekdayRosterAssignment{
			{
				Weekday:        activityContract.WeekdayMonday,
				StaffIDs:       []int64{s.staffB},
				PrimaryStaffID: &s.staffB,
				StudentIDs:     []int64{s.studentB},
			},
			{
				Weekday:        activityContract.WeekdayTuesday,
				StaffIDs:       []int64{s.staffB},
				PrimaryStaffID: &s.staffB,
				StudentIDs:     []int64{s.studentB},
			},
		},
	}))

	for _, row := range s.openSupervisorRows(t, result.TemplateID) {
		require.NotNil(t, row.CalendarPeriodID)
		assert.Equal(t, s.periodID, *row.CalendarPeriodID)
		assert.Equal(t, s.staffB, row.StaffID)
	}
	openEnrollments := 0
	for _, row := range s.enrollmentRows(t, result.TemplateID) {
		if row.ValidUntil != nil {
			continue
		}
		openEnrollments++
		require.NotNil(t, row.CalendarPeriodID)
		assert.Equal(t, s.periodID, *row.CalendarPeriodID)
		assert.Equal(t, s.studentB, row.StudentID)
	}
	assert.Equal(t, 2, openEnrollments)

	_, err = s.materialize.MaterializeForTenant(s.ctx, monday, tuesday, timetable.MaterializationSourceManual)
	require.NoError(t, err)
	for _, date := range []calendar.Date{monday, tuesday} {
		instance := s.singleInstance(t, result.TemplateID, date)
		assert.Equal(t, []int64{s.staffB}, s.instanceStaffIDs(t, instance))
		assert.Equal(t, []int64{s.studentB}, s.instanceStudentIDs(t, instance))
	}
}

func TestUpdateTemplate_UnpinningPeriodRetiresScopedEditorRoster(t *testing.T) {
	t.Parallel()

	monday := calendar.NewDate(2026, time.April, 20)
	s := makeWeekdayRosterScenario(t, monday)
	defer s.teardown(t)

	name := fmt.Sprintf("Unpinned-Roster-%d", time.Now().UnixNano())
	result, err := s.timetableData.Templates.CreateTemplate(s.ctx, timetable.CreateTemplateCommand{
		Name:             name,
		Type:             activityContract.GroupTypeCare,
		Weekdays:         []int{activityContract.WeekdayMonday},
		StartTime:        clockTime(14, 0),
		EndTime:          clockTime(15, 0),
		RoomID:           s.roomID,
		CategoryID:       s.categoryID,
		MaxParticipants:  20,
		CalendarPeriodID: &s.periodID,
		RosterValidFrom:  monday.AddDays(-30),
		GradeLevelMax:    4,
		StaffIDs:         []int64{s.staffA},
		PrimaryStaffID:   &s.staffA,
		StudentIDs:       []int64{s.studentA},
	})
	require.NoError(t, err)
	s.registerTemplate(t, result.TemplateID, result.TimeframeID)

	require.NoError(t, s.timetableData.Templates.UpdateTemplate(s.ctx, timetable.UpdateTemplateCommand{
		TemplateID: result.TemplateID,
		Fields: timetable.TemplateFields{
			Name:            name,
			Type:            activityContract.GroupTypeCare,
			CategoryID:      s.categoryID,
			RoomID:          s.roomID,
			MaxParticipants: 20,
			TargetGroupType: activityContract.TargetGroupTypeNone,
		},
		Weekdays:        []int{activityContract.WeekdayMonday},
		TimeframeID:     result.TimeframeID,
		RosterValidFrom: monday.AddDays(-30),
		GradeLevelMax:   4,
		StaffIDs:        []int64{s.staffB},
		PrimaryStaffID:  &s.staffB,
		StudentIDs:      []int64{s.studentB},
	}))

	openSupervisors := s.openSupervisorRows(t, result.TemplateID)
	require.Len(t, openSupervisors, 1)
	assert.Nil(t, openSupervisors[0].CalendarPeriodID)
	assert.Equal(t, s.staffB, openSupervisors[0].StaffID)

	openEnrollments := make([]*activityContract.StudentEnrollment, 0)
	for _, row := range s.enrollmentRows(t, result.TemplateID) {
		if row.ValidUntil == nil {
			openEnrollments = append(openEnrollments, row)
		}
	}
	require.Len(t, openEnrollments, 1)
	assert.Nil(t, openEnrollments[0].CalendarPeriodID)
	assert.Equal(t, s.studentB, openEnrollments[0].StudentID)

	_, err = s.materialize.MaterializeForTenant(s.ctx, monday, monday, timetable.MaterializationSourceManual)
	require.NoError(t, err)
	instance := s.singleInstance(t, result.TemplateID, monday)
	assert.Equal(t, []int64{s.staffB}, s.instanceStaffIDs(t, instance))
	assert.Equal(t, []int64{s.studentB}, s.instanceStudentIDs(t, instance))
}

func TestUpdateTemplate_ProtectedWeekdayDoesNotSuppressManualOtherWeekday(t *testing.T) {
	t.Parallel()

	monday := calendar.NewDate(2026, time.April, 20)
	tuesday := monday.AddDays(1)
	s := makeWeekdayRosterScenario(t, monday)
	defer s.teardown(t)

	name := fmt.Sprintf("Protected-Weekday-Update-%d", time.Now().UnixNano())
	result, err := s.timetableData.Templates.CreateTemplate(s.ctx, timetable.CreateTemplateCommand{
		Name:            name,
		Type:            activityContract.GroupTypeCare,
		Weekdays:        []int{activityContract.WeekdayMonday, activityContract.WeekdayTuesday},
		StartTime:       clockTime(14, 0),
		EndTime:         clockTime(15, 0),
		RoomID:          s.roomID,
		CategoryID:      s.categoryID,
		MaxParticipants: 20,
		RosterValidFrom: monday.AddDays(-30),
		GradeLevelMax:   4,
	})
	require.NoError(t, err)
	s.registerTemplate(t, result.TemplateID, result.TimeframeID)

	protected := &activityContract.StudentEnrollment{
		StudentID:        s.studentA,
		ActivityGroupID:  result.TemplateID,
		ValidFrom:        activityContract.Date(monday.AddDays(-30)),
		SelectedWeekdays: []int{activityContract.WeekdayMonday},
	}
	require.NoError(t, ownedStudentEnrollmentRepository(t, s.db).Create(s.ctx, protected))

	update := timetable.UpdateTemplateCommand{
		TemplateID: result.TemplateID,
		Fields: timetable.TemplateFields{
			Name:            name,
			Type:            activityContract.GroupTypeCare,
			CategoryID:      s.categoryID,
			RoomID:          s.roomID,
			MaxParticipants: 20,
			TargetGroupType: activityContract.TargetGroupTypeNone,
		},
		Weekdays:        []int{activityContract.WeekdayMonday, activityContract.WeekdayTuesday},
		TimeframeID:     result.TimeframeID,
		RosterValidFrom: monday.AddDays(-30),
		GradeLevelMax:   4,
		StudentIDs:      []int64{s.studentA},
		WeekdayAssignments: []timetable.WeekdayRosterAssignment{
			{Weekday: activityContract.WeekdayMonday, StudentIDs: []int64{s.studentA}},
			{Weekday: activityContract.WeekdayTuesday, StudentIDs: []int64{s.studentA}},
		},
	}
	require.NoError(t, s.timetableData.Templates.UpdateTemplate(s.ctx, update))

	rows := s.enrollmentRows(t, result.TemplateID)
	manualWeekdays := make([]int, 0)
	for _, row := range rows {
		if row.StudentID != s.studentA || len(row.SelectedWeekdays) > 0 || row.EnrollmentRequestChildID != nil {
			continue
		}
		require.NotNil(t, row.Weekday)
		manualWeekdays = append(manualWeekdays, *row.Weekday)
	}
	assert.Equal(t, []int{activityContract.WeekdayTuesday}, manualWeekdays)

	_, err = s.materialize.MaterializeForTenant(s.ctx, monday, tuesday, timetable.MaterializationSourceManual)
	require.NoError(t, err)
	for _, date := range []calendar.Date{monday, tuesday} {
		instance := s.singleInstance(t, result.TemplateID, date)
		assert.Contains(t, s.instanceStudentIDs(t, instance), s.studentA)
	}
}

func TestUpdateTemplate_SharedRosterDoesNotBroadenProtectedWeekdays(t *testing.T) {
	t.Parallel()

	monday := calendar.NewDate(2026, time.April, 20)
	tuesday := monday.AddDays(1)
	s := makeWeekdayRosterScenario(t, monday)
	defer s.teardown(t)

	name := fmt.Sprintf("Protected-Shared-Update-%d", time.Now().UnixNano())
	result, err := s.timetableData.Templates.CreateTemplate(s.ctx, timetable.CreateTemplateCommand{
		Name:            name,
		Type:            activityContract.GroupTypeCare,
		Weekdays:        []int{activityContract.WeekdayMonday, activityContract.WeekdayTuesday},
		StartTime:       clockTime(14, 0),
		EndTime:         clockTime(15, 0),
		RoomID:          s.roomID,
		CategoryID:      s.categoryID,
		MaxParticipants: 20,
		RosterValidFrom: monday.AddDays(-30),
		GradeLevelMax:   4,
	})
	require.NoError(t, err)
	s.registerTemplate(t, result.TemplateID, result.TimeframeID)

	protected := &activityContract.StudentEnrollment{
		StudentID:        s.studentA,
		ActivityGroupID:  result.TemplateID,
		ValidFrom:        activityContract.Date(monday.AddDays(-30)),
		SelectedWeekdays: []int{activityContract.WeekdayMonday},
	}
	require.NoError(t, ownedStudentEnrollmentRepository(t, s.db).Create(s.ctx, protected))

	update := timetable.UpdateTemplateCommand{
		TemplateID: result.TemplateID,
		Fields: timetable.TemplateFields{
			Name:            name,
			Type:            activityContract.GroupTypeCare,
			CategoryID:      s.categoryID,
			RoomID:          s.roomID,
			MaxParticipants: 20,
			TargetGroupType: activityContract.TargetGroupTypeNone,
		},
		Weekdays:        []int{activityContract.WeekdayMonday, activityContract.WeekdayTuesday},
		TimeframeID:     result.TimeframeID,
		RosterValidFrom: monday.AddDays(-30),
		GradeLevelMax:   4,
		StudentIDs:      []int64{s.studentA},
	}
	require.NoError(t, s.timetableData.Templates.UpdateTemplate(s.ctx, update))

	rows := s.enrollmentRows(t, result.TemplateID)
	require.Len(t, rows, 1)
	assert.Equal(t, protected.ID, rows[0].ID)
	assert.Equal(t, []int{activityContract.WeekdayMonday}, rows[0].SelectedWeekdays)
	assert.Nil(t, rows[0].Weekday)

	_, err = s.materialize.MaterializeForTenant(s.ctx, monday, tuesday, timetable.MaterializationSourceManual)
	require.NoError(t, err)
	assert.Contains(t, s.instanceStudentIDs(t, s.singleInstance(t, result.TemplateID, monday)), s.studentA)
	assert.NotContains(t, s.instanceStudentIDs(t, s.singleInstance(t, result.TemplateID, tuesday)), s.studentA)
}

// A weekday the series does not run on is rejected rather than silently
// dropped or silently added to the recurrence.
func TestCreateTemplate_WeekdayAssignmentOutsideRecurrence_IsRejected(t *testing.T) {
	t.Parallel()

	monday := calendar.NewDate(2026, time.April, 20)
	s := makeWeekdayRosterScenario(t, monday)
	defer s.teardown(t)

	_, err := s.timetableData.Templates.CreateTemplate(s.ctx, timetable.CreateTemplateCommand{
		Name:            fmt.Sprintf("Invalid-%d", time.Now().UnixNano()),
		Type:            activityContract.GroupTypeCare,
		Weekdays:        []int{activityContract.WeekdayMonday},
		StartTime:       clockTime(14, 0),
		EndTime:         clockTime(15, 0),
		RoomID:          s.roomID,
		CategoryID:      s.categoryID,
		MaxParticipants: 20,
		RosterValidFrom: monday.AddDays(-30),
		GradeLevelMax:   4,
		WeekdayAssignments: []timetable.WeekdayRosterAssignment{{
			Weekday:  activityContract.WeekdayWednesday,
			StaffIDs: []int64{s.staffA},
		}},
	})
	require.ErrorIs(t, err, timetable.ErrWeekdayAssignmentUnscheduled)
}

func TestTemplateWeekdayRosterRead_PreservesEmptyDaysAndProtectedEnrollments(t *testing.T) {
	t.Parallel()

	monday := calendar.NewDate(2026, time.April, 20)
	s := makeWeekdayRosterScenario(t, monday)
	defer s.teardown(t)

	result, err := s.timetableData.Templates.CreateTemplate(s.ctx, timetable.CreateTemplateCommand{
		Name:            fmt.Sprintf("Read-Roster-%d", time.Now().UnixNano()),
		Type:            activityContract.GroupTypeCare,
		Weekdays:        []int{activityContract.WeekdayMonday, activityContract.WeekdayTuesday},
		StartTime:       clockTime(14, 0),
		EndTime:         clockTime(15, 0),
		RoomID:          s.roomID,
		CategoryID:      s.categoryID,
		MaxParticipants: 20,
		RosterValidFrom: monday.AddDays(-30),
		GradeLevelMax:   4,
		WeekdayAssignments: []timetable.WeekdayRosterAssignment{
			{Weekday: activityContract.WeekdayMonday},
			{
				Weekday:    activityContract.WeekdayTuesday,
				StaffIDs:   []int64{s.staffB},
				StudentIDs: []int64{s.studentB},
			},
		},
	})
	require.NoError(t, err)
	s.registerTemplate(t, result.TemplateID, result.TimeframeID)

	protected := &activityContract.StudentEnrollment{
		StudentID:        s.studentA,
		ActivityGroupID:  result.TemplateID,
		ValidFrom:        activityContract.Date(monday.AddDays(-30)),
		SelectedWeekdays: []int{activityContract.WeekdayMonday},
	}
	require.NoError(t, ownedStudentEnrollmentRepository(t, s.db).Create(s.ctx, protected))
	inconsistentWeekday := activityContract.WeekdayTuesday
	inconsistentProtected := &activityContract.StudentEnrollment{
		StudentID:        s.studentA,
		ActivityGroupID:  result.TemplateID,
		ValidFrom:        activityContract.Date(monday.AddDays(-30)),
		SelectedWeekdays: []int{activityContract.WeekdayMonday},
		Weekday:          &inconsistentWeekday,
	}
	require.NoError(t, ownedStudentEnrollmentRepository(t, s.db).Create(s.ctx, inconsistentProtected))

	rows, err := api.NewUnobservedTimetableRepositories(testpkg.MustDatabase(s.db)).ActivityGroup.ListTemplateWeekdayRoster(s.ctx, &result.TemplateID, nil)
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
	assert.True(t, found[rosterKey{weekday: activityContract.WeekdayMonday, kind: timetable.TemplateWeekdayRosterKindEmpty}],
		"an intentionally empty Monday needs an API marker")
	assert.True(t, found[rosterKey{weekday: activityContract.WeekdayTuesday, kind: timetable.TemplateWeekdayRosterKindEmpty}])
	assert.True(t, found[rosterKey{weekday: activityContract.WeekdayMonday, kind: timetable.TemplateWeekdayRosterKindStudent, person: s.studentA}],
		"the protected child must remain visible on the weekday where it materializes")
	assert.True(t, found[rosterKey{weekday: activityContract.WeekdayMonday, kind: timetable.TemplateWeekdayRosterKindProtectedStudent, person: s.studentA}],
		"the protected coverage must be exposed separately to the editor")
	assert.False(t, found[rosterKey{weekday: activityContract.WeekdayTuesday, kind: timetable.TemplateWeekdayRosterKindStudent, person: s.studentA}],
		"selected_weekdays must also limit a protected row carrying a weekday scope")
	assert.False(t, found[rosterKey{weekday: activityContract.WeekdayTuesday, kind: timetable.TemplateWeekdayRosterKindProtectedStudent, person: s.studentA}],
		"protected coverage must retain selected_weekdays")
	assert.True(t, found[rosterKey{weekday: activityContract.WeekdayTuesday, kind: timetable.TemplateWeekdayRosterKindStudent, person: s.studentB}])
	assert.True(t, found[rosterKey{weekday: activityContract.WeekdayTuesday, kind: timetable.TemplateWeekdayRosterKindStaff, person: s.staffB}])
}

func TestTemplateWeekdayRosterRead_ExpandsSharedRowsAcrossScopedDays(t *testing.T) {
	t.Parallel()

	monday := calendar.NewDate(2026, time.April, 20)
	tuesday := monday.AddDays(1)
	s := makeWeekdayRosterScenario(t, monday)
	defer s.teardown(t)

	result, err := s.timetableData.Templates.CreateTemplate(s.ctx, timetable.CreateTemplateCommand{
		Name:             fmt.Sprintf("Read-Mixed-Roster-%d", time.Now().UnixNano()),
		Type:             activityContract.GroupTypeCare,
		Weekdays:         []int{activityContract.WeekdayMonday, activityContract.WeekdayTuesday},
		StartTime:        clockTime(14, 0),
		EndTime:          clockTime(15, 0),
		RoomID:           s.roomID,
		CategoryID:       s.categoryID,
		MaxParticipants:  20,
		CalendarPeriodID: &s.periodID,
		RosterValidFrom:  monday.AddDays(-30),
		GradeLevelMax:    4,
		WeekdayAssignments: []timetable.WeekdayRosterAssignment{
			{
				Weekday:        activityContract.WeekdayMonday,
				StaffIDs:       []int64{s.staffB},
				PrimaryStaffID: &s.staffB,
				StudentIDs:     []int64{s.studentB},
			},
			{Weekday: activityContract.WeekdayTuesday},
		},
	})
	require.NoError(t, err)
	s.registerTemplate(t, result.TemplateID, result.TimeframeID)

	repos := api.NewUnobservedTimetableRepositories(testpkg.MustDatabase(s.db))
	require.NoError(t, repos.ActivitySupervisor.Create(s.ctx, &activityContract.SupervisorPlanned{
		StaffID:   s.staffA,
		GroupID:   result.TemplateID,
		IsPrimary: true,
		ValidFrom: activityContract.Date(monday.AddDays(-30)),
	}))
	require.NoError(t, repos.StudentEnrollment.Create(s.ctx, &activityContract.StudentEnrollment{
		StudentID:       s.studentA,
		ActivityGroupID: result.TemplateID,
		ValidFrom:       activityContract.Date(monday.AddDays(-30)),
	}))

	rows, err := repos.ActivityGroup.ListTemplateWeekdayRoster(s.ctx, &result.TemplateID, &s.periodID)
	require.NoError(t, err)
	type dayRoster struct {
		staff    []int64
		students []int64
		primary  int64
	}
	byWeekday := make(map[int]*dayRoster)
	for _, row := range rows {
		roster := byWeekday[row.Weekday]
		if roster == nil {
			roster = &dayRoster{}
			byWeekday[row.Weekday] = roster
		}
		switch row.Kind {
		case timetable.TemplateWeekdayRosterKindStaff:
			roster.staff = append(roster.staff, row.PersonID)
			if row.IsPrimary {
				roster.primary = row.PersonID
			}
		case timetable.TemplateWeekdayRosterKindStudent:
			roster.students = append(roster.students, row.PersonID)
		}
	}

	require.Contains(t, byWeekday, activityContract.WeekdayMonday)
	assert.ElementsMatch(t, []int64{s.staffA, s.staffB}, byWeekday[activityContract.WeekdayMonday].staff)
	assert.ElementsMatch(t, []int64{s.studentA, s.studentB}, byWeekday[activityContract.WeekdayMonday].students)
	assert.Equal(t, s.staffB, byWeekday[activityContract.WeekdayMonday].primary,
		"the weekday-scoped primary must override the shared fallback")
	require.Contains(t, byWeekday, activityContract.WeekdayTuesday)
	assert.Equal(t, []int64{s.staffA}, byWeekday[activityContract.WeekdayTuesday].staff)
	assert.Equal(t, []int64{s.studentA}, byWeekday[activityContract.WeekdayTuesday].students)
	assert.Equal(t, s.staffA, byWeekday[activityContract.WeekdayTuesday].primary)

	_, err = s.materialize.MaterializeForTenant(s.ctx, monday, tuesday, timetable.MaterializationSourceManual)
	require.NoError(t, err)
	mondayInstance := s.singleInstance(t, result.TemplateID, monday)
	tuesdayInstance := s.singleInstance(t, result.TemplateID, tuesday)
	assert.ElementsMatch(t, byWeekday[activityContract.WeekdayMonday].staff, s.instanceStaffIDs(t, mondayInstance))
	assert.ElementsMatch(t, byWeekday[activityContract.WeekdayMonday].students, s.instanceStudentIDs(t, mondayInstance))
	assert.True(t, s.instanceHasPrimary(t, mondayInstance, byWeekday[activityContract.WeekdayMonday].primary))
	assert.ElementsMatch(t, byWeekday[activityContract.WeekdayTuesday].staff, s.instanceStaffIDs(t, tuesdayInstance))
	assert.ElementsMatch(t, byWeekday[activityContract.WeekdayTuesday].students, s.instanceStudentIDs(t, tuesdayInstance))
	assert.True(t, s.instanceHasPrimary(t, tuesdayInstance, byWeekday[activityContract.WeekdayTuesday].primary))
}

func TestTemplateWeekdayRosterRead_ExposesProtectedCoverageForSharedRoster(t *testing.T) {
	t.Parallel()

	monday := calendar.NewDate(2026, time.April, 20)
	s := makeWeekdayRosterScenario(t, monday)
	defer s.teardown(t)

	result, err := s.timetableData.Templates.CreateTemplate(s.ctx, timetable.CreateTemplateCommand{
		Name:            fmt.Sprintf("Read-Protected-Shared-%d", time.Now().UnixNano()),
		Type:            activityContract.GroupTypeCare,
		Weekdays:        []int{activityContract.WeekdayMonday, activityContract.WeekdayTuesday},
		StartTime:       clockTime(14, 0),
		EndTime:         clockTime(15, 0),
		RoomID:          s.roomID,
		CategoryID:      s.categoryID,
		MaxParticipants: 20,
		RosterValidFrom: monday.AddDays(-30),
		GradeLevelMax:   4,
		StudentIDs:      []int64{s.studentB},
	})
	require.NoError(t, err)
	s.registerTemplate(t, result.TemplateID, result.TimeframeID)

	protected := &activityContract.StudentEnrollment{
		StudentID:        s.studentA,
		ActivityGroupID:  result.TemplateID,
		ValidFrom:        activityContract.Date(monday.AddDays(-30)),
		SelectedWeekdays: []int{activityContract.WeekdayMonday},
	}
	require.NoError(t, ownedStudentEnrollmentRepository(t, s.db).Create(s.ctx, protected))

	rows, err := api.NewUnobservedTimetableRepositories(testpkg.MustDatabase(s.db)).ActivityGroup.ListTemplateWeekdayRoster(s.ctx, &result.TemplateID, nil)
	require.NoError(t, err)
	protectedWeekdays := make([]int, 0)
	for _, row := range rows {
		if row.Kind == timetable.TemplateWeekdayRosterKindProtectedStudent && row.PersonID == s.studentA {
			protectedWeekdays = append(protectedWeekdays, row.Weekday)
		}
	}
	assert.Equal(t, []int{activityContract.WeekdayMonday}, protectedWeekdays)
}

func TestTemplateWeekdayRosterRead_IsolatesCalendarPeriods(t *testing.T) {
	t.Parallel()

	monday := calendar.NewDate(2026, time.April, 20)
	s := makeWeekdayRosterScenario(t, monday)
	defer s.teardown(t)

	result, err := s.timetableData.Templates.CreateTemplate(s.ctx, timetable.CreateTemplateCommand{
		Name:             fmt.Sprintf("Period-Roster-%d", time.Now().UnixNano()),
		Type:             activityContract.GroupTypeCare,
		Weekdays:         []int{activityContract.WeekdayMonday},
		StartTime:        clockTime(14, 0),
		EndTime:          clockTime(15, 0),
		RoomID:           s.roomID,
		CategoryID:       s.categoryID,
		MaxParticipants:  20,
		CalendarPeriodID: &s.periodID,
		RosterValidFrom:  monday.AddDays(-30),
		GradeLevelMax:    4,
		WeekdayAssignments: []timetable.WeekdayRosterAssignment{{
			Weekday:    activityContract.WeekdayMonday,
			StaffIDs:   []int64{s.staffA},
			StudentIDs: []int64{s.studentA},
		}},
	})
	require.NoError(t, err)
	s.registerTemplate(t, result.TemplateID, result.TimeframeID)

	periodB := &scheduleModels.CalendarPeriod{
		Name:            fmt.Sprintf("Roster-B-%d", time.Now().UnixNano()),
		PeriodType:      scheduleModels.PeriodTypeCustom,
		StartDate:       calendar.NewDate(2026, 1, 1),
		EndDate:         calendar.NewDate(2026, 12, 31),
		WeekCycleLength: 1,
		IsActive:        true,
	}
	createCalendarPeriodRow(t, s.db, s.ctx, periodB)
	s.cleanup = append(s.cleanup, func() {
	})

	// An unpinned template can be used in either period. Its open rosters are
	// still period-specific and must not be merged by the editor read.
	_, err = s.db.NewUpdate().Table("activities.groups").
		Set("calendar_period_id = NULL").
		Where("tenant_id = ?", s.tenantID).
		Where("id = ?", result.TemplateID).
		Exec(s.ctx)
	require.NoError(t, err)
	_, err = s.db.NewUpdate().Table("activities.schedules").
		Set("calendar_period_id = NULL").
		Where("tenant_id = ?", s.tenantID).
		Where("activity_group_id = ?", result.TemplateID).
		Exec(s.ctx)
	require.NoError(t, err)

	weekday := activityContract.WeekdayMonday
	repos := api.NewUnobservedTimetableRepositories(testpkg.MustDatabase(s.db))
	require.NoError(t, repos.ActivitySupervisor.Create(s.ctx, &activityContract.SupervisorPlanned{
		StaffID:          s.staffB,
		GroupID:          result.TemplateID,
		ValidFrom:        activityContract.Date(monday.AddDays(-30)),
		CalendarPeriodID: &periodB.ID,
		Weekday:          &weekday,
	}))
	require.NoError(t, repos.StudentEnrollment.Create(s.ctx, &activityContract.StudentEnrollment{
		StudentID:        s.studentB,
		ActivityGroupID:  result.TemplateID,
		ValidFrom:        activityContract.Date(monday.AddDays(-30)),
		CalendarPeriodID: &periodB.ID,
		Weekday:          &weekday,
	}))

	periodARows, err := repos.ActivityGroup.ListTemplateWeekdayRoster(s.ctx, &result.TemplateID, &s.periodID)
	require.NoError(t, err)
	periodBRows, err := repos.ActivityGroup.ListTemplateWeekdayRoster(s.ctx, &result.TemplateID, &periodB.ID)
	require.NoError(t, err)
	unscopedRows, err := repos.ActivityGroup.ListTemplateWeekdayRoster(s.ctx, &result.TemplateID, nil)
	require.NoError(t, err)

	assertWeekdayRosterPeople(t, periodARows, []int64{s.staffA}, []int64{s.studentA})
	assertWeekdayRosterPeople(t, periodBRows, []int64{s.staffB}, []int64{s.studentB})
	assert.Empty(t, unscopedRows, "a nil period must not merge the two period-specific rosters")
}

func assertWeekdayRosterPeople(
	t *testing.T,
	rows []timetable.TemplateWeekdayRosterRow,
	wantStaff, wantStudents []int64,
) {
	t.Helper()
	staff := make([]int64, 0)
	students := make([]int64, 0)
	for _, row := range rows {
		switch row.Kind {
		case timetable.TemplateWeekdayRosterKindStaff:
			staff = append(staff, row.PersonID)
		case timetable.TemplateWeekdayRosterKindStudent:
			students = append(students, row.PersonID)
		}
	}
	assert.ElementsMatch(t, wantStaff, staff)
	assert.ElementsMatch(t, wantStudents, students)
}

// -----------------------------------------------------------------------------
// Scenario plumbing
// -----------------------------------------------------------------------------

func clockTime(hour, minute int) time.Time {
	return time.Date(2000, time.January, 1, hour, minute, 0, 0, time.UTC)
}

func makeWeekdayRosterScenario(t *testing.T, anchor calendar.Date) *weekdayRosterScenario {
	t.Helper()
	db := testpkg.SetupTestDB(t)

	ttLifecycle, ttPlanning, ttMaterialization, ttActive, ttSchedules, ttCareOfferings, ttStatusDays, err := api.NewTestTimetable(testpkg.NewDatabase(t, db), slog.Default())
	require.NoError(t, err)

	tenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, tenantID)
	ctx := testpkg.TenantContext(tenantID)
	suffix := time.Now().UnixNano()

	period := &scheduleModels.CalendarPeriod{
		Name:            fmt.Sprintf("Schuljahr-%d", suffix),
		PeriodType:      scheduleModels.PeriodTypeSchoolYear,
		StartDate:       calendar.NewDate(anchor.Year()-1, 8, 1),
		EndDate:         calendar.NewDate(anchor.Year()+1, 7, 31),
		WeekCycleLength: 1,
		IsActive:        true,
	}
	createCalendarPeriodRow(t, db, ctx, period)

	room := testpkg.CreateTestRoomForTenant(t, db, tenantID, fmt.Sprintf("Raum-%d", suffix))
	category := testpkg.CreateTestActivityCategoryForTenant(t, db, tenantID, fmt.Sprintf("Kat-%d", suffix))
	staffA := testpkg.CreateTestStaffForTenant(t, db, tenantID, "Anna", fmt.Sprintf("Montag-%d", suffix))
	staffB := testpkg.CreateTestStaffForTenant(t, db, tenantID, "Bea", fmt.Sprintf("Dienstag-%d", suffix))
	studentA := testpkg.CreateTestStudentForTenant(t, db, tenantID, "Alice", fmt.Sprintf("Eins-%d", suffix), "3a")
	studentB := testpkg.CreateTestStudentForTenant(t, db, tenantID, "Bob", fmt.Sprintf("Zwei-%d", suffix), "3a")

	s := &weekdayRosterScenario{
		db:              db,
		ctx:             ctx,
		tenantID:        tenantID,
		instance:        ttLifecycle.Instances,
		timetableData:   ttPlanning.Planning,
		active:          ttActive,
		materialization: ttMaterialization,
		careDay:         ttSchedules.CareDay,
		autoStart:       ttLifecycle.AutoStart,
		careOffering:    ttCareOfferings,
		statusDays:      ttStatusDays,
		materialize:     ttMaterialization,
		roomID:          room.ID,
		categoryID:      category.ID,
		staffA:          staffA.ID,
		staffB:          staffB.ID,
		studentA:        studentA.ID,
		studentB:        studentB.ID,
		periodID:        period.ID,
	}
	s.cleanup = append(s.cleanup, func() {
	})
	return s
}

// registerTemplate schedules teardown for everything one CreateTemplate call
// produced, children first so the FKs hold.
func (s *weekdayRosterScenario) registerTemplate(t *testing.T, templateID, timeframeID int64) {
	t.Helper()
	s.templateIDs = append(s.templateIDs, templateID)
	t.Cleanup(func() {
		instanceIDs := s.templateInstanceIDs(t, templateID)
		for _, table := range []string{"schedule.instance_students", "schedule.instance_staff"} {
			s.deleteByInstanceIDs(t, table, instanceIDs)
		}
		s.deleteByTemplate(t, "activities.student_enrollments", "activity_group_id", templateID)
		s.deleteByTemplate(t, "activities.supervisors", "group_id", templateID)
		s.deleteByTemplate(t, "activities.schedules", "activity_group_id", templateID)
	})
}

func (s *weekdayRosterScenario) teardown(t *testing.T) {
	t.Helper()
	for _, fn := range s.cleanup {
		fn()
	}
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

func (s *weekdayRosterScenario) singleInstance(t *testing.T, templateID int64, date calendar.Date) int64 {
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

func (s *weekdayRosterScenario) supervisorRows(t *testing.T, templateID int64) []*activityContract.SupervisorPlanned {
	t.Helper()
	var rows []*SupervisorPlanned
	require.NoError(t, s.db.NewSelect().Model(&rows).
		ModelTableExpr(`activities.supervisors AS "supervisor_planned"`).
		Where(`"supervisor_planned".group_id = ?`, templateID).
		Where(`"supervisor_planned".tenant_id = ?`, s.tenantID).
		Scan(s.ctx))
	return nativeSupervisorPlannedRows(rows)
}

func (s *weekdayRosterScenario) openSupervisorRows(t *testing.T, templateID int64) []*activityContract.SupervisorPlanned {
	t.Helper()
	open := make([]*activityContract.SupervisorPlanned, 0)
	for _, row := range s.supervisorRows(t, templateID) {
		if row.ValidUntil == nil {
			open = append(open, row)
		}
	}
	require.NotEmpty(t, open, "expected at least one open supervision row")
	return open
}

func (s *weekdayRosterScenario) enrollmentRows(t *testing.T, templateID int64) []*activityContract.StudentEnrollment {
	t.Helper()
	var rows []*StudentEnrollment
	require.NoError(t, s.db.NewSelect().Model(&rows).
		ModelTableExpr(`activities.student_enrollments AS "student_enrollment"`).
		Where(`"student_enrollment".activity_group_id = ?`, templateID).
		Where(`"student_enrollment".tenant_id = ?`, s.tenantID).
		Scan(s.ctx))
	return nativeStudentEnrollmentRows(rows)
}
