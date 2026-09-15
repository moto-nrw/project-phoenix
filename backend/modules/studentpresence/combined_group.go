package studentpresence

import (
	"context"
	"errors"
	"time"
)

var ErrCombinedGroupNotFound = errors.New("combined group not found")

type CombinedGroup struct {
	ID, TenantID         int64
	CreatedAt, UpdatedAt time.Time
	StartTime            time.Time
	EndTime              *time.Time
}

// CombinedGroupFilter selects open combinations or those overlapping an inclusive time range.
type CombinedGroupFilter struct {
	// Active uses the current database time; future end times still count as active.
	Active        *bool
	ID            *int64
	Limit, Offset int
	OpenOnly      bool
	From, Until   *time.Time
}

func (m *Module) ListCombinedGroups(ctx context.Context, filter CombinedGroupFilter) ([]CombinedGroup, error) {
	return m.engine.ListCombinedGroups(ctx, filter)
}
func (m *Module) EndCombination(ctx context.Context, id int64, at time.Time) error {
	return m.engine.EndCombination(ctx, id, at)
}

func (m *Module) GetCombinedGroup(ctx context.Context, id int64) (CombinedGroup, error) {
	return m.engine.GetCombinedGroup(ctx, id)
}
func (m *Module) RecordCombination(ctx context.Context, start time.Time, end *time.Time) (CombinedGroup, error) {
	return m.engine.RecordCombination(ctx, start, end)
}
func (m *Module) ReviseCombination(ctx context.Context, id int64, start time.Time, end *time.Time) (CombinedGroup, error) {
	return m.engine.ReviseCombination(ctx, id, start, end)
}

// DeleteCombination removes a combination if present; an absent identity is a no-op.
func (m *Module) DeleteCombination(ctx context.Context, id int64) error {
	return m.engine.DeleteCombination(ctx, id)
}
