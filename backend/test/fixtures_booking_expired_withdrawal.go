package test

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/stretchr/testify/require"
)

// CreateTestBookingExpiredCareWithdrawal stores the pending withdrawal task
// the system opens when a booking-led school's last booking of the child
// expired: today is the first day without a booking. Switching booking-led
// care off discards exactly these tasks, so the fixture is the starting state
// of that side effect.
func CreateTestBookingExpiredCareWithdrawal(
	tb testing.TB,
	withdrawals CareWithdrawalWriter,
	studentID int64,
) *users.CareWithdrawalCompletion {
	tb.Helper()
	row := &users.CareWithdrawalCompletion{
		StudentID:               &studentID,
		FirstBookinglessDay:     timezone.TodayDate(),
		Trigger:                 users.CareWithdrawalTriggerBookingExpired,
		WithdrawalConfirmedRole: "system",
		WithdrawalConfirmedAt:   time.Now(),
	}
	require.NoError(tb, withdrawals.UpsertPending(Ctx(tb), row))
	return row
}
