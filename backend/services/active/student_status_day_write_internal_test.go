package active

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/stretchr/testify/assert"
)

func TestDedupeStudentIDsPreservesFirstOccurrenceOrder(t *testing.T) {
	assert.Equal(t, []int64{7, 3, 9}, dedupeStudentIDs([]int64{7, 3, 7, 9, 3}))
	assert.Empty(t, dedupeStudentIDs(nil))
}

func TestIsNewReportableAbsence(t *testing.T) {
	today := timezone.NewDate(2026, 7, 29)
	yesterday := timezone.NewDate(2026, 7, 28)
	trueValue := true
	falseValue := false

	assert.True(t, isNewReportableAbsence(
		&userModels.Student{Sick: &falseValue},
		activeModels.StudentStatusDaySick,
		[]timezone.Date{today},
		today,
	))
	assert.False(t, isNewReportableAbsence(
		&userModels.Student{Sick: &trueValue},
		activeModels.StudentStatusDaySick,
		[]timezone.Date{today},
		today,
	), "re-saving the current status must not notify again")
	assert.True(t, isNewReportableAbsence(
		&userModels.Student{Sick: &trueValue, Excused: &falseValue},
		activeModels.StudentStatusDayExcused,
		[]timezone.Date{today},
		today,
	), "changing the reportable absence type must notify")
	assert.False(t, isNewReportableAbsence(
		&userModels.Student{},
		activeModels.StudentStatusDayClassTrip,
		[]timezone.Date{today},
		today,
	))
	assert.False(t, isNewReportableAbsence(
		&userModels.Student{},
		activeModels.StudentStatusDaySick,
		[]timezone.Date{yesterday},
		today,
	))
}
