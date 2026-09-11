package compose

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar"
	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar/internal/adapters/postgres"
	"github.com/uptrace/bun"
)

// NewFeedHistory binds feed retention to School Calendar's tenant runtime.
func NewFeedHistory(db *bun.DB) schoolcalendar.FeedHistory {
	runtime := PersistenceRuntimeFor(db)
	return feedHistory{store: postgres.New(func(ctx context.Context) (bun.IDB, int64, error) {
		return runtime.Database(ctx), runtime.TenantID(ctx), nil
	})}
}

type feedHistory struct{ store *postgres.Store }

func (h feedHistory) ListForStaffSince(ctx context.Context, staffID int64, since time.Time) ([]schoolcalendar.FeedCancellation, error) {
	rows, err := h.store.ListFeedCancellations(ctx, staffID, since)
	if err != nil {
		return nil, err
	}
	values := make([]schoolcalendar.FeedCancellation, 0, len(rows))
	for _, row := range rows {
		values = append(values, schoolcalendar.FeedCancellation{
			Source: row.Source, SourceID: row.SourceID, Title: row.Title,
			EventDate: row.EventDate, StartTime: row.StartTime,
			EndTime: row.EndTime, CancelledAt: row.CancelledAt,
		})
	}
	return values, nil
}

func (h feedHistory) DeleteBefore(ctx context.Context, before time.Time) (int, error) {
	return h.store.DeleteFeedCancellationsBefore(ctx, before)
}
