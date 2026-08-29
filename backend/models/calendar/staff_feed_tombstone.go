package calendar

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/base"
)

type StaffFeedTombstone struct {
	base.Model `bun:"schema:calendar,table:staff_feed_tombstones"`
	base.TenantModel
	StaffID     int64         `bun:"staff_id,notnull"`
	Source      string        `bun:"source,notnull"`
	SourceID    int64         `bun:"source_id,notnull"`
	Title       string        `bun:"title,notnull"`
	EventDate   timezone.Date `bun:"event_date,notnull,type:date"`
	StartTime   time.Time     `bun:"start_time,notnull"`
	EndTime     time.Time     `bun:"end_time,notnull"`
	CancelledAt time.Time     `bun:"cancelled_at,notnull"`
}

type StaffFeedTombstoneRepository interface {
	ListForStaffSince(ctx context.Context, staffID int64, since time.Time) ([]*StaffFeedTombstone, error)
	DeleteBefore(ctx context.Context, before time.Time) (int, error)
}
