package schedule

import (
	"testing"

	activitiesModel "github.com/moto-nrw/project-phoenix/models/activities"
	"github.com/stretchr/testify/assert"
)

func TestValidateTemplateCreateInputRejectsWeekendWeekday(t *testing.T) {
	err := validateTemplateCreateInput(CreateTemplateInput{
		Name:     "Wochenende",
		Weekdays: []int{activitiesModel.WeekdaySaturday},
	})

	assert.ErrorContains(t, err, "Monday to Friday")
}

func TestValidateSplitRecurrenceRejectsWeekendWeekday(t *testing.T) {
	err := validateSplitRecurrence(TemplateSplitInput{
		Weekdays: []int{activitiesModel.WeekdaySunday},
	})

	assert.ErrorContains(t, err, "Monday to Friday")
}

func TestResolveTemplateRosterRejectsPrimaryOutsideWeekdayStaff(t *testing.T) {
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
