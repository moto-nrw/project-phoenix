package application

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/ports"
)

type pickupReviewStore interface {
	ListPartialAbsenceBlocks(context.Context, int64, string, time.Time, bool, []int64) ([]domain.PartialAbsenceBlock, domain.OperationStats, error)
}

type PickupReviewQueries struct {
	store   pickupReviewStore
	observe ports.Observer
}

func NewPickupReviewQueries(store pickupReviewStore, observe ports.Observer) *PickupReviewQueries {
	return &PickupReviewQueries{store: store, observe: observe}
}

func (q *PickupReviewQueries) PreviewPickupBlocks(ctx context.Context, studentID int64, date string, from time.Time, enrolled bool, autoIDs []int64) ([]domain.PartialAbsenceBlock, error) {
	if studentID <= 0 {
		return nil, fmt.Errorf("pickup review: positive student ID is required")
	}
	parsed, err := time.Parse("2006-01-02", date)
	if err != nil || parsed.Format("2006-01-02") != date {
		return nil, fmt.Errorf("pickup review: invalid calendar date %q", date)
	}
	started := time.Now()
	blocks, stats, err := q.store.ListPartialAbsenceBlocks(ctx, studentID, date, from, enrolled, autoIDs)
	q.observe(ports.Observation{Operation: "preview_pickup_blocks", Duration: time.Since(started), Stats: stats, Err: err})
	return blocks, err
}
