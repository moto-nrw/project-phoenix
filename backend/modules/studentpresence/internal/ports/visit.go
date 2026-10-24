package ports

import (
	"context"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
)

// ErrOpenVisitExists reports that the store refused a second open visit of a
// student: the partial unique index uniq_active_visits_open_per_student.
var ErrOpenVisitExists = errors.New("student already has an open visit")

type VisitStore interface {
	ListOpenVisitStudentIDs(context.Context, int64) ([]int64, Stats, error)
	VisitLocationStore
	VisitRetentionStore
	CloseGroupVisits(context.Context, []int64) (Stats, error)
	TransferOpenVisits(context.Context, int64, int64) (Stats, error)
	TransferRecentDeviceVisits(context.Context, int64, int64) (Stats, error)
	DeleteCompletedVisitsBefore(context.Context, int64, studentpresence.VisitRetentionPolicy, time.Time) (Stats, error)
	FindVisit(context.Context, int64) (*studentpresence.Visit, Stats, error)
	ListVisits(context.Context, studentpresence.VisitFilter) ([]*studentpresence.Visit, Stats, error)
	RecordVisit(context.Context, *studentpresence.Visit) (Stats, error)
	ReviseVisit(context.Context, *studentpresence.Visit) (Stats, error)
	DeleteVisit(context.Context, int64) (Stats, error)
	CloseVisits(context.Context, []int64, time.Time) ([]*studentpresence.Visit, Stats, error)
}
