package studentpresence

import (
	"context"
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

// PresenceHistory writes the presence of past care days for the demo
// seeder (#5031). No route reaches it: only the dev tools bind it, since no
// production flow records presence after the fact. It requires a
// tenant-scoped context and writes every day in one transaction, with each
// row created at its own instant so the retention windows count from then.
type PresenceHistory interface {
	RecordPresenceHistory(ctx context.Context, days []PastPresenceDay) (int, error)
}
