package users

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/stretchr/testify/assert"
)

func TestCareWithdrawalCompletionUrgencyIsDerived(t *testing.T) {
	t.Parallel()
	reference := calendar.NewDate(2026, time.August, 23)

	planned := CareWithdrawalCompletion{FirstBookinglessDay: reference.AddDays(1)}
	overdue := CareWithdrawalCompletion{FirstBookinglessDay: reference}

	assert.Equal(t, CareWithdrawalUrgencyPlanned, planned.UrgencyOn(reference))
	assert.Equal(t, CareWithdrawalUrgencyOverdue, overdue.UrgencyOn(reference))
}
