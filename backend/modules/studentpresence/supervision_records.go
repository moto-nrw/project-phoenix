package studentpresence

import (
	"context"
	"errors"
	"time"
)

var ErrSupervisionNotFound = errors.New("supervision not found")

func (m *Module) RecordSupervision(ctx context.Context, row GroupSupervision) (GroupSupervision, error) {
	return m.engine.RecordSupervision(ctx, row)
}

func (m *Module) ReviseSupervision(ctx context.Context, row GroupSupervision) (GroupSupervision, error) {
	return m.engine.ReviseSupervision(ctx, row)
}

func (m *Module) RemoveSupervision(ctx context.Context, id int64) error {
	return m.engine.RemoveSupervision(ctx, id)
}

func (m *Module) SetSupervisionEnd(ctx context.Context, id int64, endDate string, at time.Time) (int64, error) {
	return m.engine.SetSupervisionEnd(ctx, id, endDate, at)
}

// EndOpenGroupSupervisions ends open assignments that started by eligibleOn, using the database's current date.
func (m *Module) EndOpenGroupSupervisions(ctx context.Context, groupID, staffID int64, eligibleOn string) (int, error) {
	return m.engine.EndOpenGroupSupervisions(ctx, groupID, staffID, eligibleOn)
}
