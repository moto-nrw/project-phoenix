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
	*base.Repository[*calendarModels.StaffFeedTombstone]
}

func NewStaffFeedTombstoneRepository(db *bun.DB) calendarModels.StaffFeedTombstoneRepository {
	repo := base.NewRepository[*calendarModels.StaffFeedTombstone](db, "calendar.staff_feed_tombstones", "StaffFeedTombstone")
	repo.TenantScoped = true
	return &StaffFeedTombstoneRepository{Repository: repo}
}

// ListForStaffSince keeps the ordered timestamp range and PostgreSQL TIME
// normalization together. The calendar repository cannot import the generic
// QueryOptions domain across its architecture boundary.
func (r *StaffFeedTombstoneRepository) ListForStaffSince(ctx context.Context, staffID int64, since time.Time) ([]*calendarModels.StaffFeedTombstone, error) {
	rows := make([]*calendarModels.StaffFeedTombstone, 0)
	query := base.GetDB(ctx, r.DB).NewSelect().
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

// DeleteBefore exposes the feed-retention operation while delegating its
// tenant-scoped timestamp deletion to the generic repository implementation.
func (r *StaffFeedTombstoneRepository) DeleteBefore(ctx context.Context, before time.Time) (int, error) {
	count, err := r.Repository.DeleteBefore(ctx, "cancelled_at", before, "delete expired staff feed tombstones")
	if err != nil {
		return 0, err
	}
	return int(count), nil
}
