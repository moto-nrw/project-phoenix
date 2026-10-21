package postgres

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar"
)

func NewNativeFeedHistory(store *Store) schoolcalendar.FeedHistory { return feedHistory{Store: store} }

// feedHistory promotes DeleteFeedCancellationsBefore from the store unchanged.
type feedHistory struct{ *Store }

func (h feedHistory) ListForStaffSince(ctx context.Context, staffID int64, since time.Time) ([]schoolcalendar.FeedCancellation, error) {
	rows, err := h.ListFeedCancellations(ctx, staffID, since)
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
