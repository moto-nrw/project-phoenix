package application

import (
	"testing"
	"time"

	activityContract "github.com/moto-nrw/project-phoenix/modules/timetable/activitycontract"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateTemplateCreateInputRejectsWeekendWeekday(t *testing.T) {
	t.Parallel()

	err := validateTemplateCreateInput(CreateTemplateInput{
		Name:     "Wochenende",
		Weekdays: []int{activityContract.WeekdaySaturday},
	}, testSchoolClassRules)

	assert.ErrorContains(t, err, "Monday to Friday")
}

func TestValidateSplitRecurrenceRejectsWeekendWeekday(t *testing.T) {
	t.Parallel()

	err := validateSplitRecurrence(TemplateSplitInput{
		Weekdays: []int{activityContract.WeekdaySunday},
	}, calendar.NewDate(2026, 8, 24))

	assert.ErrorContains(t, err, "Monday to Friday")
}

func TestValidateSplitInputAcceptsTargetsWithoutLegacyMirror(t *testing.T) {
	t.Parallel()

	gradeTwo := int16(2)
	gradeThree := int16(3)
	in := TemplateSplitInput{
		TemplateID:      1,
		EffectiveDate:   calendar.NewDate(2026, 8, 25),
		Name:            "Lernzeit",
		Type:            activityContract.GroupTypeActivity,
		Weekdays:        []int{activityContract.WeekdayMonday},
		StartTime:       time.Date(2000, 1, 1, 14, 0, 0, 0, time.UTC),
		EndTime:         time.Date(2000, 1, 1, 15, 0, 0, 0, time.UTC),
		RoomID:          1,
		CategoryID:      1,
		GradeLevelMax:   testSchoolClassRules.MaxGradeLevel,
		TargetGroupType: activityContract.TargetGroupTypeJahrgang,
		Targets: []*activityContract.GroupTarget{
			{TargetGroupType: activityContract.TargetGroupTypeJahrgang, TargetGradeLevel: &gradeTwo},
			{TargetGroupType: activityContract.TargetGroupTypeJahrgang, TargetGradeLevel: &gradeThree},
		},
	}

	require.NoError(t, validateSplitInput(&in, calendar.NewDate(2026, 8, 24), testSchoolClassRules))
	require.NotNil(t, in.TargetGradeLevel)
	assert.Equal(t, gradeTwo, *in.TargetGradeLevel)
	assert.Len(t, in.Targets, 2)
}

func TestResolveTemplateRosterRejectsPrimaryOutsideWeekdayStaff(t *testing.T) {
	t.Parallel()

	primaryID := int64(22)
	_, err := resolveTemplateRoster(
		[]int{activityContract.WeekdayMonday},
		nil,
		[]int64{11},
		nil,
		[]timetable.WeekdayRosterAssignment{{
			Weekday:        activityContract.WeekdayMonday,
			StaffIDs:       []int64{11},
			PrimaryStaffID: &primaryID,
		}},
	)

	assert.ErrorIs(t, err, timetable.ErrWeekdayAssignmentPrimaryStaffMissing)
}

func TestValidateSplitWeekdayAssignmentsRequiresBothExplicitRosters(t *testing.T) {
	t.Parallel()

	assignment := timetable.WeekdayRosterAssignment{Weekday: activityContract.WeekdayMonday}

	for _, input := range []TemplateSplitInput{
		{
			Weekdays:           []int{activityContract.WeekdayMonday},
			StudentIDs:         []int64{1},
			StaffIDs:           nil,
			WeekdayAssignments: []timetable.WeekdayRosterAssignment{assignment},
		},
		{
			Weekdays:           []int{activityContract.WeekdayMonday},
			StudentIDs:         nil,
			StaffIDs:           []int64{2},
			WeekdayAssignments: []timetable.WeekdayRosterAssignment{assignment},
		},
	} {
		err := validateSplitWeekdayAssignments(input)
		assert.ErrorIs(t, err, timetable.ErrSplitInvalidInput)
		assert.ErrorContains(t, err, "explicit student and staff rosters")
	}
}
