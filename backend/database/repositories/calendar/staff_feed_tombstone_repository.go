package calendar

import (
	"context"
	"fmt"
	"time"

	calendarModels "github.com/moto-nrw/project-phoenix/models/calendar"
)

type StaffFeedTombstoneRepository struct {
	runtime Runtime
}

func NewStaffFeedTombstoneRepository(runtime Runtime) calendarModels.StaffFeedTombstoneRepository {
	runtime.validate()
	return &StaffFeedTombstoneRepository{runtime: runtime}
}

// ListForStaffSince keeps the ordered timestamp range and PostgreSQL TIME
// normalization together. The calendar repository cannot import the generic
// QueryOptions domain across its architecture boundary.
func (r *StaffFeedTombstoneRepository) ListForStaffSince(ctx context.Context, staffID int64, since time.Time) ([]*calendarModels.StaffFeedTombstone, error) {
	rows := make([]*calendarModels.StaffFeedTombstone, 0)
	query := r.runtime.Database(ctx).NewSelect().
		Model(&rows).
		ModelTableExpr(`calendar.staff_feed_tombstones AS "staff_feed_tombstone"`).
		Where(`"staff_feed_tombstone".staff_id = ?`, staffID).
		Where(`"staff_feed_tombstone".cancelled_at >= ?`, since).
		OrderExpr(`"staff_feed_tombstone".cancelled_at ASC`)
	query = withTenantFilter(r.runtime, ctx, query, "staff_feed_tombstone")
	if err := query.Scan(ctx); err != nil {
		return nil, fmt.Errorf("list staff feed tombstones: %w", err)
	}
	for _, row := range rows {
		row.StartTime = normalizeWallClock(row.StartTime)
		row.EndTime = normalizeWallClock(row.EndTime)
	}
	return rows, nil
}

// DeleteBefore removes tenant-scoped tombstones older than the retention
// cutoff and reports the number deleted.
func (r *StaffFeedTombstoneRepository) DeleteBefore(ctx context.Context, before time.Time) (int, error) {
	query := r.runtime.Database(ctx).NewDelete().
		Model((*calendarModels.StaffFeedTombstone)(nil)).
		ModelTableExpr(`calendar.staff_feed_tombstones AS "staff_feed_tombstone"`).
		Where(`"staff_feed_tombstone".cancelled_at < ?`, before)
	query = withTenantFilter(r.runtime, ctx, query, "staff_feed_tombstone")
	result, err := query.Exec(ctx)
	if err != nil {
		return 0, fmt.Errorf("delete expired staff feed tombstones: %w", err)
	}
	count, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("delete expired staff feed tombstones: rows affected: %w", err)
	}
	return int(count), nil
}
