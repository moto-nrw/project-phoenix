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
	IDs, StudentIDs, ActiveGroupIDs                            []int64
	EnteredFrom, EnteredUntil                                  *time.Time
	OverlapFrom, OverlapUntil                                  *time.Time
	OpenOnly, ClosedOnly, NewestFirst, StudentOrder, ForUpdate bool
	Limit, Offset                                              int
}

type VisitQuery interface {
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

func (m *Module) FindVisit(ctx context.Context, id int64) (*Visit, error) {
	return m.engine.FindVisit(ctx, id)
}
func (m *Module) ListVisits(ctx context.Context, filter VisitFilter) ([]Visit, error) {
	return m.engine.ListVisits(ctx, filter)
}
func (m *Module) RecordVisit(ctx context.Context, value Visit) (Visit, error) {
	return m.engine.RecordVisit(ctx, value)
}
func (m *Module) ReviseVisit(ctx context.Context, value Visit) (Visit, error) {
	return m.engine.ReviseVisit(ctx, value)
}
func (m *Module) DeleteVisit(ctx context.Context, id int64) error {
	return m.engine.DeleteVisit(ctx, id)
}
func (m *Module) CloseVisits(ctx context.Context, ids []int64, at time.Time) ([]Visit, error) {
	return m.engine.CloseVisits(ctx, ids, at)
}
