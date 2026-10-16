package test

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/uptrace/bun"
)

// CreateTestBookingExpiredCareWithdrawal stores the pending withdrawal task
// the system opens when a booking-led school's last booking of the child
// expired: today is the first day without a booking. Switching booking-led
// care off discards exactly these tasks, so the fixture is the starting state
// of that side effect.
func CreateTestBookingExpiredCareWithdrawal(tb testing.TB, db bun.IDB, studentID int64) *CareWithdrawalCompletion {
	tb.Helper()
	return CreateTestBookingExpiredCareWithdrawalOn(tb, db, studentID, calendar.TodayDate())
}
