package schedule_test

import (
	"context"

	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
)

func (f *fakeInstanceRepo) FindByActiveGroupIDs(_ context.Context, activeGroupIDs []int64) ([]*scheduleModel.ActivityInstance, error) {
	if f.findPanic != nil {
		panic(f.findPanic)
	}
	if f.instance == nil || f.findErr != nil || len(activeGroupIDs) == 0 {
		return nil, f.findErr
	}
	instance := *f.instance
	if instance.ActiveGroupID == nil {
		instance.ActiveGroupID = &activeGroupIDs[0]
	}
	return []*scheduleModel.ActivityInstance{&instance}, nil
}
