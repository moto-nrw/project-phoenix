package timetable

import (
	"context"
	"time"
)

// PickupReviewInput supplies child and exception facts from their owners.
// The query reads only Timetable's attendance and activity records.
type PickupReviewInput struct {
	StudentID        int64
	Date             string
	From             time.Time
	Enrolled         bool
	AutoExceptionIDs []int64
}

type PickupReviewQuery interface {
	PreviewPickupBlocks(context.Context, PickupReviewInput) ([]PartialAbsenceBlock, error)
}
