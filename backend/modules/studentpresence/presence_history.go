package studentpresence

import (
	"context"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// PastPresenceDay is one finished care day of one child in the past: the
// daily attendance from check-in to check-out and, optionally, one room
// visit during it.
type PastPresenceDay struct {
	StudentID int64
	Date      calendar.Date
	// DeviceID is the kiosk that booked the arrival and the departure.
	DeviceID          int64
	CheckIn, CheckOut time.Time
	Visit             *PastVisit
}

// PastVisit is a room visit inside a finished session. Visits naming the
// same day, activity and room share one session, so they must name the
// same session bounds.
type PastVisit struct {
	ActivityGroupID, RoomID  int64
	SessionStart, SessionEnd time.Time
	Entry, Exit              time.Time
}

// ErrPresenceHistoryInvalid rejects a past day that no finished care day
// could have produced; nothing of the batch is written then.
var ErrPresenceHistoryInvalid = errors.New("student presence: invalid presence history")

// PresenceHistory writes the presence of past care days for the demo
// seeder (#5031). No production flow records presence after the fact: the
// dev tools bind it, and the server mounts it only for the operator of the
// public demo (APP_ENV=demo, ADR 0127). It requires a tenant-scoped context
// and writes every day in one transaction, with each row created at its own
// instant so the retention windows count from then.
type PresenceHistory interface {
	RecordPresenceHistory(ctx context.Context, days []PastPresenceDay) (int, error)
}
