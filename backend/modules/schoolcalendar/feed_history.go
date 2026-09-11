package schoolcalendar

import (
	"context"
	"time"
)

// FeedCancellation keeps a removed timetable event visible to subscribed
// calendars until the retention window ends. It contains no private notes.
type FeedCancellation struct {
	Source      string
	SourceID    int64
	Title       string
	EventDate   string
	StartTime   time.Time
	EndTime     time.Time
	CancelledAt time.Time
}

// FeedHistory owns retention of calendar.staff_feed_tombstones. Both operations
// require a tenant and participate in its ambient transaction.
type FeedHistory interface {
	ListForStaffSince(context.Context, int64, time.Time) ([]FeedCancellation, error)
	DeleteBefore(context.Context, time.Time) (int, error)
}
