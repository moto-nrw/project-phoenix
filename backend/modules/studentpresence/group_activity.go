package studentpresence

import (
	"context"
	"errors"
	"time"
)

var ErrGroupNotOpen = errors.New("group is missing or already ended")

// RecordGroupActivity stamps activity only while the tenant's session remains open.
func (m *Module) RecordGroupActivity(ctx context.Context, id int64, at time.Time) error {
	return m.engine.RecordGroupActivity(ctx, id, at)
}

func (m *Module) DeleteGroup(ctx context.Context, id int64) error {
	return m.engine.DeleteGroup(ctx, id)
}

func (m *Module) ReviseGroup(ctx context.Context, group LiveGroup) (LiveGroup, error) {
	return m.engine.ReviseGroup(ctx, group)
}

func (m *Module) RecordGroup(ctx context.Context, group LiveGroup) (LiveGroup, error) {
	return m.engine.RecordGroup(ctx, group)
}

func (m *Module) EndSupervisionOn(ctx context.Context, id int64, date string) (int, error) {
	return m.engine.EndSupervisionOn(ctx, id, date)
}

func (m *Module) EndStaffSupervisionsOn(ctx context.Context, id int64, date string) (int, error) {
	return m.engine.EndStaffSupervisionsOn(ctx, id, date)
}
