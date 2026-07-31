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
