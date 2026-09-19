package compose

import (
	"context"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence/internal/ports"
)

func (e engine) RecordGroupActivity(ctx context.Context, id int64, at time.Time) error {
	err := e.Service.RecordGroupActivity(ctx, id, at)
	if errors.Is(err, ports.ErrGroupNotOpen) {
		return studentpresence.ErrGroupNotOpen
	}
	return err
}

func (e engine) DeleteGroup(ctx context.Context, id int64) error {
	return e.Service.DeleteGroup(ctx, id)
}

func (e engine) ReviseGroup(ctx context.Context, group studentpresence.LiveGroup) (studentpresence.LiveGroup, error) {
	row, err := e.Service.ReviseGroup(ctx, ports.LiveGroup(group))
	return liveGroupToPublic(row), err
}

func (e engine) RecordGroup(ctx context.Context, group studentpresence.LiveGroup) (studentpresence.LiveGroup, error) {
	row, err := e.Service.RecordGroup(ctx, ports.LiveGroup(group))
	return liveGroupToPublic(row), err
}
