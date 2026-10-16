package test

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/uptrace/bun"
)

// CreateTestBookingExpiredCareWithdrawalOn stores the pending withdrawal task
// the system opens when the child's last booking expired, with firstGap as
// the first day without a booking. Upserting again moves the same task to a
// new day.
func CreateTestBookingExpiredCareWithdrawalOn(
	tb testing.TB,
	db bun.IDB,
	studentID int64,
	firstGap calendar.Date,
) *CareWithdrawalCompletion {
	tb.Helper()
	return upsertTestPendingCareWithdrawal(tb, db, &CareWithdrawalCompletion{
		StudentID:               &studentID,
		FirstBookinglessDay:     firstGap,
		Trigger:                 "booking_expired",
		WithdrawalConfirmedRole: "system",
		WithdrawalConfirmedAt:   time.Now(),
	})
}

// CreateTestCareWithdrawalCompletionConfirmedAt is
// CreateTestCareWithdrawalCompletion with an explicit confirmation instant.
func CreateTestCareWithdrawalCompletionConfirmedAt(
	tb testing.TB,
	db bun.IDB,
	studentID, actorID int64,
	firstGap calendar.Date,
	confirmedAt time.Time,
) *CareWithdrawalCompletion {
	tb.Helper()
	return upsertTestPendingCareWithdrawal(tb, db, &CareWithdrawalCompletion{
		StudentID:               &studentID,
		FirstBookinglessDay:     firstGap,
		Trigger:                 "direct_school",
		WithdrawalConfirmedBy:   &actorID,
		WithdrawalConfirmedRole: "admin",
		WithdrawalConfirmedAt:   confirmedAt,
	})
}
