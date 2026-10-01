package test

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/stretchr/testify/require"
)

// CreateTestBookingExpiredCareWithdrawalOn stores the pending withdrawal task
// the system opens when the child's last booking expired, with firstGap as
// the first day without a booking. Upserting again moves the same task to a
// new day.
func CreateTestBookingExpiredCareWithdrawalOn(
	tb testing.TB,
	withdrawals CareWithdrawalWriter,
	studentID int64,
	firstGap timezone.Date,
) *users.CareWithdrawalCompletion {
	tb.Helper()
	row := &users.CareWithdrawalCompletion{
		StudentID:               &studentID,
		FirstBookinglessDay:     firstGap,
		Trigger:                 users.CareWithdrawalTriggerBookingExpired,
		WithdrawalConfirmedRole: "system",
		WithdrawalConfirmedAt:   time.Now(),
	}
	require.NoError(tb, withdrawals.UpsertPending(Ctx(tb), row))
	return row
}

// CreateTestCareWithdrawalCompletionConfirmedAt is
// CreateTestCareWithdrawalCompletion with an explicit confirmation instant.
func CreateTestCareWithdrawalCompletionConfirmedAt(
	tb testing.TB,
	withdrawals CareWithdrawalWriter,
	studentID, actorID int64,
	firstGap timezone.Date,
	confirmedAt time.Time,
) *users.CareWithdrawalCompletion {
	tb.Helper()
	row := &users.CareWithdrawalCompletion{
		StudentID:               &studentID,
		FirstBookinglessDay:     firstGap,
		Trigger:                 users.CareWithdrawalTriggerDirectSchool,
		WithdrawalConfirmedBy:   &actorID,
		WithdrawalConfirmedRole: "admin",
		WithdrawalConfirmedAt:   confirmedAt,
	}
	require.NoError(tb, withdrawals.UpsertPending(Ctx(tb), row))
	return row
}
