package studentpresence

import (
	"context"
	"errors"
	"time"
)

var ErrVisitNotFound = errors.New("visit not found")

// Visit records an interval in a school-owned active group. The student's
// home school can differ for an authorized cross-school holiday visit.
type Visit struct {
	ID, TenantID             int64
	CreatedAt, UpdatedAt     time.Time
	StudentID, ActiveGroupID int64
	EntryTime                time.Time
	ExitTime                 *time.Time
}

// VisitFilter selects recorded intervals. Nil ID lists are unrestricted;
// non-nil empty lists select nothing. Overlap includes both endpoints.
type VisitFilter struct {
	// ClosedSince leaves current visits intact while restricting completed history.
	ClosedSince                                                *time.Time
	IDs, StudentIDs, ActiveGroupIDs                            []int64
	EnteredFrom, EnteredUntil                                  *time.Time
	OverlapFrom, OverlapUntil                                  *time.Time
	OpenOnly, ClosedOnly, NewestFirst, StudentOrder, ForUpdate bool
	Limit, Offset                                              int
}

type VisitQuery interface {
	ListOpenVisitStudentIDs(context.Context, int64) ([]int64, error)
	VisitLocationQuery
	VisitRetentionQuery
	FindVisit(context.Context, int64) (*Visit, error)
	ListVisits(context.Context, VisitFilter) ([]Visit, error)
}

type VisitCommand interface {
	VisitMaintenance
	RecordVisit(context.Context, Visit) (Visit, error)
	ReviseVisit(context.Context, Visit) (Visit, error)
	DeleteVisit(context.Context, int64) error
	CloseVisits(context.Context, []int64, time.Time) ([]Visit, error)
}
