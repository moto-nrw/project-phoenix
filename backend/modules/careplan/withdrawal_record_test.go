package careplan

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/stretchr/testify/assert"
)

func TestCareWithdrawalCompletionUrgencyIsDerived(t *testing.T) {
	t.Parallel()
	reference := calendar.NewDate(2026, time.August, 23)

	planned := WithdrawalCompletion{FirstBookinglessDay: reference.AddDays(1)}
	overdue := WithdrawalCompletion{FirstBookinglessDay: reference}

	assert.Equal(t, WithdrawalUrgencyPlanned, planned.UrgencyOn(reference))
	assert.Equal(t, WithdrawalUrgencyOverdue, overdue.UrgencyOn(reference))
}
