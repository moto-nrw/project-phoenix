package postgres

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar"
)

func NewNativeFeedHistory(store *Store) schoolcalendar.FeedHistory { return feedHistory{store: store} }

type feedHistory struct{ store *Store }

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
