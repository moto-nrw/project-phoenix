package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar/internal/domain"
	"github.com/uptrace/bun"
)

type feedCancellationRow struct {
	bun.BaseModel `bun:"table:staff_feed_tombstones,alias:staff_feed_tombstone"`
	Source        string       `bun:"source"`
	SourceID      int64        `bun:"source_id"`
	Title         string       `bun:"title"`
	EventDate     calendarDate `bun:"event_date,type:date"`
	StartTime     time.Time    `bun:"start_time"`
	EndTime       time.Time    `bun:"end_time"`
	CancelledAt   time.Time    `bun:"cancelled_at"`
}

func (s *Store) ListFeedCancellations(ctx context.Context, staffID int64, since time.Time) ([]domain.FeedCancellation, error) {
	db, tenantID, err := s.tenantDatabase(ctx, "list staff feed tombstones")
	if err != nil {
		return nil, err
	}
	rows := make([]feedCancellationRow, 0)
	err = db.NewSelect().Model(&rows).
		ModelTableExpr(`calendar.staff_feed_tombstones AS "staff_feed_tombstone"`).
		Where(`"staff_feed_tombstone".tenant_id = ?`, tenantID).
		Where(`"staff_feed_tombstone".staff_id = ?`, staffID).
		Where(`"staff_feed_tombstone".cancelled_at >= ?`, since).
		OrderExpr(`"staff_feed_tombstone".cancelled_at ASC`).Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("list staff feed tombstones: %w", err)
	}
	values := make([]domain.FeedCancellation, 0, len(rows))
	for _, row := range rows {
		values = append(values, domain.FeedCancellation{
			Source: row.Source, SourceID: row.SourceID, Title: row.Title,
			EventDate: string(row.EventDate), StartTime: feedClock(row.StartTime),
			EndTime: feedClock(row.EndTime), CancelledAt: row.CancelledAt,
		})
	}
	return values, nil
}

func (s *Store) DeleteFeedCancellationsBefore(ctx context.Context, before time.Time) (int, error) {
	db, tenantID, err := s.tenantDatabase(ctx, "delete staff feed tombstones")
	if err != nil {
		return 0, err
	}
	result, err := db.NewDelete().Model((*feedCancellationRow)(nil)).
		ModelTableExpr(`calendar.staff_feed_tombstones AS "staff_feed_tombstone"`).
		Where(`"staff_feed_tombstone".tenant_id = ?`, tenantID).
		Where(`"staff_feed_tombstone".cancelled_at < ?`, before).Exec(ctx)
	if err != nil {
		return 0, fmt.Errorf("delete expired staff feed tombstones: %w", err)
	}
	count, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("delete expired staff feed tombstones: rows affected: %w", err)
	}
	return int(count), nil
}

// PostgreSQL TIME carries clock fields, not an instant to convert between zones.
func feedClock(value time.Time) time.Time {
	return time.Date(1, time.January, 1, value.Hour(), value.Minute(), value.Second(), value.Nanosecond(), time.UTC)
}
