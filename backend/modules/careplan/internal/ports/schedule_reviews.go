package ports

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/domain"
)

type ScheduleReviewDirectory interface {
	FindStudents(context.Context, []int64) (map[int64]ReviewStudent, error)
	PersonNames(context.Context, []int64) (map[int64]PersonName, error)
	ReviewerNames(context.Context, []int64) (map[int64]PersonName, error)
	DepartureModes(context.Context, []int64) (map[int64]map[string][]string, error)
}

type PickupReviewImpact struct {
	StudentID        int64
	Date             domain.Date
	From             time.Time
	Enrolled         bool
	AutoExceptionIDs []int64
}

type PickupReviewBlock struct {
	ID        int64
	Title     string
	StartTime time.Time
	EndTime   time.Time
}

type PickupReviewBlocks interface {
	PreviewPickupBlocks(context.Context, PickupReviewImpact) ([]PickupReviewBlock, error)
}
