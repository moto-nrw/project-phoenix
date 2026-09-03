package scheduler

import (
	"context"

	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
)

func (r *fakeInstanceRepo) FindByActiveGroupIDs(context.Context, []int64) ([]*scheduleModel.ActivityInstance, error) {
	return nil, nil
}
