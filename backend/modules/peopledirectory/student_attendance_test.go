package peopledirectory_test

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/stretchr/testify/assert"
)

func TestAttendsOnEndsWithGraduationAndAfterTheLastCareDay(t *testing.T) {
	t.Parallel()
	lastCareDay := calendar.Date("2026-10-19")

	assert.True(t, peopledirectory.AttendsOn(&peopledirectory.StudentState{}, lastCareDay), "a child without an end of care attends")
	assert.True(t, peopledirectory.AttendsOn(&peopledirectory.StudentState{EnrolledUntil: &lastCareDay}, lastCareDay), "the last care day still counts")
	assert.False(t, peopledirectory.AttendsOn(&peopledirectory.StudentState{EnrolledUntil: &lastCareDay}, lastCareDay.AddDays(1)), "care ended")
	assert.False(t, peopledirectory.AttendsOn(&peopledirectory.StudentState{Status: peopledirectory.StudentStatusAlumnus}, lastCareDay), "a graduate no longer attends")
}
