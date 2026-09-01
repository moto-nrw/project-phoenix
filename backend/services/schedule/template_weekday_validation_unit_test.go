package schedule

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/schoolclass"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activitiesModel "github.com/moto-nrw/project-phoenix/models/activities"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateTemplateCreateInputRejectsWeekendWeekday(t *testing.T) {
	t.Parallel()

	err := validateTemplateCreateInput(CreateTemplateInput{
		Name:     "Wochenende",
		Weekdays: []int{activitiesModel.WeekdaySaturday},
	})

	assert.ErrorContains(t, err, "Monday to Friday")
}

func TestValidateSplitRecurrenceRejectsWeekendWeekday(t *testing.T) {
	t.Parallel()

	err := validateSplitRecurrence(TemplateSplitInput{
		Weekdays: []int{activitiesModel.WeekdaySunday},
	}, timezone.NewDate(2026, 8, 24))

	assert.ErrorContains(t, err, "Monday to Friday")
}

func TestValidateSplitInputAcceptsTargetsWithoutLegacyMirror(t *testing.T) {
	t.Parallel()

	gradeTwo := int16(2)
	gradeThree := int16(3)
	in := TemplateSplitInput{
		TemplateID:      1,
		EffectiveDate:   timezone.NewDate(2026, 8, 25),
		Name:            "Lernzeit",
		Type:            activitiesModel.GroupTypeActivity,
		Weekdays:        []int{activitiesModel.WeekdayMonday},
		StartTime:       time.Date(2000, 1, 1, 14, 0, 0, 0, time.UTC),
		EndTime:         time.Date(2000, 1, 1, 15, 0, 0, 0, time.UTC),
		RoomID:          1,
		CategoryID:      1,
		GradeLevelMax:   schoolclass.MaxGradeLevel,
		TargetGroupType: activitiesModel.TargetGroupTypeJahrgang,
		Targets: []*activitiesModel.GroupTarget{
			{TargetGroupType: activitiesModel.TargetGroupTypeJahrgang, TargetGradeLevel: &gradeTwo},
			{TargetGroupType: activitiesModel.TargetGroupTypeJahrgang, TargetGradeLevel: &gradeThree},
		},
	}

	require.NoError(t, validateSplitInput(&in, timezone.NewDate(2026, 8, 24)))
	require.NotNil(t, in.TargetGradeLevel)
	assert.Equal(t, gradeTwo, *in.TargetGradeLevel)
	assert.Len(t, in.Targets, 2)
}

func TestResolveTemplateRosterRejectsPrimaryOutsideWeekdayStaff(t *testing.T) {
	t.Parallel()

	primaryID := int64(22)
	_, err := resolveTemplateRoster(
		[]int{activitiesModel.WeekdayMonday},
		nil,
		[]int64{11},
		nil,
		[]WeekdayRosterAssignment{{
			Weekday:        activitiesModel.WeekdayMonday,
			StaffIDs:       []int64{11},
			PrimaryStaffID: &primaryID,
		}},
	)

	assert.ErrorIs(t, err, ErrWeekdayAssignmentPrimaryStaffMissing)
}

func TestValidateSplitWeekdayAssignmentsRequiresBothExplicitRosters(t *testing.T) {
	t.Parallel()

	assignment := WeekdayRosterAssignment{Weekday: activitiesModel.WeekdayMonday}

	for _, input := range []TemplateSplitInput{
		{
			Weekdays:           []int{activitiesModel.WeekdayMonday},
			StudentIDs:         []int64{1},
			StaffIDs:           nil,
			WeekdayAssignments: []WeekdayRosterAssignment{assignment},
		},
		{
			Weekdays:           []int{activitiesModel.WeekdayMonday},
			StudentIDs:         nil,
			StaffIDs:           []int64{2},
			WeekdayAssignments: []WeekdayRosterAssignment{assignment},
		},
	} {
		err := validateSplitWeekdayAssignments(input)
		assert.ErrorIs(t, err, ErrSplitInvalidInput)
		assert.ErrorContains(t, err, "explicit student and staff rosters")
	}
}
