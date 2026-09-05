package schedule

import (
	"context"
	"time"
)

// repository is the model-owned persistence contract shared by schedule
// repositories. Query shapes stay with their callers instead of coupling the
// schedule model to the generic base repository package.
type repository[T any] interface {
	Create(context.Context, T) error
	FindByID(context.Context, any) (T, error)
	Update(context.Context, T) error
	Delete(context.Context, any) error
}

type crudRepository[T any] interface {
	repository[T]
	List(context.Context, map[string]any) ([]T, error)
}

// RequestQueueFilters is the legacy care-request repository's paging shape.
// The owner facade converts it to its public RequestQueueFilter at the adapter
// boundary.
type RequestQueueFilters struct {
	UrgentOnly    *bool
	UrgentDate    string
	StudentIDs    []int64
	StudentID     int64
	Search        string
	BeforeInstant time.Time
	BeforeID      int64
	Limit         int
}
