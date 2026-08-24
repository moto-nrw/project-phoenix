package users_test

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/stretchr/testify/assert"
)

func TestCareWithdrawalCompletionUrgencyIsDerived(t *testing.T) {
	t.Parallel()
	reference := timezone.NewDate(2026, time.August, 23)

	planned := userModels.CareWithdrawalCompletion{FirstBookinglessDay: reference.AddDays(1)}
	overdue := userModels.CareWithdrawalCompletion{FirstBookinglessDay: reference}

	assert.Equal(t, userModels.CareWithdrawalUrgencyPlanned, planned.UrgencyOn(reference))
	assert.Equal(t, userModels.CareWithdrawalUrgencyOverdue, overdue.UrgencyOn(reference))
}
