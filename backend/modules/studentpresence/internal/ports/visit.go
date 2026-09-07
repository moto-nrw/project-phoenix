package ports

import (
	"context"
	"errors"
	"time"
)

// Visit is the owner's persistence-independent visit record.
type Visit struct {
	ID, TenantID             int64
	CreatedAt, UpdatedAt     time.Time
	StudentID, ActiveGroupID int64
	EntryTime                time.Time
	ExitTime                 *time.Time
}

func (v *Visit) Validate() error {
	if v.StudentID <= 0 {
		return errors.New("student ID is required")
	}
	if v.ActiveGroupID <= 0 {
		return errors.New("active group ID is required")
	}
	if v.EntryTime.IsZero() {
		return errors.New("entry time is required")
	}
	if v.ExitTime != nil && v.EntryTime.After(*v.ExitTime) {
		return errors.New("entry time must be before exit time")
	}
	return nil
}

type VisitFilter struct {
	IDs, StudentIDs, ActiveGroupIDs                            []int64
	EnteredFrom, EnteredUntil                                  *time.Time
	OverlapFrom, OverlapUntil                                  *time.Time
	OpenOnly, ClosedOnly, NewestFirst, StudentOrder, ForUpdate bool
	Limit, Offset                                              int
}

type VisitStore interface {
	VisitLocationStore
	VisitRetentionStore
	CloseGroupVisits(context.Context, []int64) (Stats, error)
	TransferOpenVisits(context.Context, int64, int64) (Stats, error)
	TransferRecentDeviceVisits(context.Context, int64, int64) (Stats, error)
	DeleteCompletedVisitsBefore(context.Context, int64, time.Time) (Stats, error)
	FindVisit(context.Context, int64) (*Visit, Stats, error)
	ListVisits(context.Context, VisitFilter) ([]*Visit, Stats, error)
	RecordVisit(context.Context, *Visit) (Stats, error)
	ReviseVisit(context.Context, *Visit) (Stats, error)
	DeleteVisit(context.Context, int64) (Stats, error)
	CloseVisits(context.Context, []int64, time.Time) ([]*Visit, Stats, error)
}
