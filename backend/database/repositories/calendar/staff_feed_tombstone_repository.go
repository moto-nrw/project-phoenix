package calendar

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	calendarModels "github.com/moto-nrw/project-phoenix/models/calendar"
	"github.com/uptrace/bun"
)

type StaffFeedTombstoneRepository struct {
	db *bun.DB
}

func NewStaffFeedTombstoneRepository(db *bun.DB) calendarModels.StaffFeedTombstoneRepository {
	return &StaffFeedTombstoneRepository{db: db}
}

func (r *StaffFeedTombstoneRepository) ListForStaffSince(ctx context.Context, staffID int64, since time.Time) ([]*calendarModels.StaffFeedTombstone, error) {
	rows := make([]*calendarModels.StaffFeedTombstone, 0)
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&rows).
		ModelTableExpr(`calendar.staff_feed_tombstones AS "staff_feed_tombstone"`).
		Where(`"staff_feed_tombstone".staff_id = ?`, staffID).
		Where(`"staff_feed_tombstone".cancelled_at >= ?`, since).
		OrderExpr(`"staff_feed_tombstone".cancelled_at ASC`)
	query = base.WithTenantFilter(ctx, query, "staff_feed_tombstone")
	if err := query.Scan(ctx); err != nil {
		return nil, fmt.Errorf("list staff feed tombstones: %w", err)
	}
	for _, row := range rows {
		row.StartTime = timezone.WallClock(row.StartTime)
		row.EndTime = timezone.WallClock(row.EndTime)
	}
	return rows, nil
}

func (r *StaffFeedTombstoneRepository) DeleteBefore(ctx context.Context, before time.Time) (int, error) {
	query := base.GetDB(ctx, r.db).NewDelete().
		Model((*calendarModels.StaffFeedTombstone)(nil)).
		ModelTableExpr(`calendar.staff_feed_tombstones AS "staff_feed_tombstone"`).
		Where(`"staff_feed_tombstone".cancelled_at < ?`, before)
	if where, value, ok := base.TenantWhere(ctx, "staff_feed_tombstone"); ok {
		query = query.Where(where, value)
	}
	result, err := query.Exec(ctx)
	if err != nil {
		return 0, fmt.Errorf("delete expired staff feed tombstones: %w", err)
	}
	count, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("count deleted staff feed tombstones: %w", err)
	}
	return int(count), nil
}
