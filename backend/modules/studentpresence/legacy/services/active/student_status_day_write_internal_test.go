package active

import (
	"context"
	"errors"
	"testing"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModels "github.com/moto-nrw/project-phoenix/modules/studentpresence/legacy/models/active"
	"github.com/stretchr/testify/assert"
)

func TestStatusDayLocksKeepOrderAndStopOnFailure(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	first := timezone.NewDate(2026, 3, 28)
	dates := []timezone.Date{first.AddDays(2), first, first.AddDays(1)}
	wantErr := errors.New("day lock failed")
	var locked []string
	service := NewStudentStatusDayServiceWithPartialAbsences(nil, nil, nil, func(received context.Context, studentID int64, date string) error {
		assert.Equal(t, ctx, received)
		assert.EqualValues(t, 7, studentID)
		locked = append(locked, date)
		if date == first.AddDays(1).String() {
			return wantErr
		}
		return nil
	})
	assert.ErrorIs(t, service.lockStudentStatusDates(ctx, 7, dates), wantErr)
	assert.Equal(t, []string{first.String(), first.AddDays(1).String()}, locked)
	assert.Equal(t, []timezone.Date{first.AddDays(2), first, first.AddDays(1)}, dates)
}

func TestDedupeStudentIDsPreservesFirstOccurrenceOrder(t *testing.T) {
	t.Parallel()

	assert.Equal(t, []int64{7, 3, 9}, dedupeStudentIDs([]int64{7, 3, 7, 9, 3}))
	assert.Empty(t, dedupeStudentIDs(nil))
}

func TestIsNewReportableAbsence(t *testing.T) {
	t.Parallel()

	today := timezone.NewDate(2026, 7, 29)
	yesterday := timezone.NewDate(2026, 7, 28)
	trueValue := true
	falseValue := false

	assert.True(t, isNewReportableAbsence(
		&StudentRecord{Sick: &falseValue},
		activeModels.StudentStatusDaySick,
		[]timezone.Date{today},
		today,
	))
	assert.False(t, isNewReportableAbsence(
		&StudentRecord{Sick: &trueValue},
		activeModels.StudentStatusDaySick,
		[]timezone.Date{today},
		today,
	), "re-saving the current status must not notify again")
	assert.True(t, isNewReportableAbsence(
		&StudentRecord{Sick: &trueValue, Excused: &falseValue},
		activeModels.StudentStatusDayExcused,
		[]timezone.Date{today},
		today,
	), "changing the reportable absence type must notify")
	assert.False(t, isNewReportableAbsence(
		&StudentRecord{},
		activeModels.StudentStatusDayClassTrip,
		[]timezone.Date{today},
		today,
	))
	assert.False(t, isNewReportableAbsence(
		&StudentRecord{},
		activeModels.StudentStatusDaySick,
		[]timezone.Date{yesterday},
		today,
	))
}
