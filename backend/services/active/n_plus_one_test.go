package active

import (
	"context"

	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
)

func (m *wsMockStaffShiftRepository) FindByOriginShiftIDs(context.Context, []int64) ([]*scheduleModels.StaffShift, error) {
	return nil, nil
}
