package calendar

import (
	"context"
	"time"
)

type StaffFeedTombstone struct {
	Model `bun:"schema:calendar,table:staff_feed_tombstones"`
	TenantModel
	StaffID     int64     `bun:"staff_id,notnull"`
	Source      string    `bun:"source,notnull"`
	SourceID    int64     `bun:"source_id,notnull"`
	Title       string    `bun:"title,notnull"`
	EventDate   Date      `bun:"event_date,notnull,type:date"`
	StartTime   time.Time `bun:"start_time,notnull"`
	EndTime     time.Time `bun:"end_time,notnull"`
	CancelledAt time.Time `bun:"cancelled_at,notnull"`
}

type StaffFeedTombstoneRepository interface {
	ListForStaffSince(ctx context.Context, staffID int64, since time.Time) ([]*StaffFeedTombstone, error)
	DeleteBefore(ctx context.Context, before time.Time) (int, error)
}
