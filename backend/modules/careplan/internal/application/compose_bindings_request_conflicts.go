package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/careplan/carerequests"
	careRecordPorts "github.com/moto-nrw/project-phoenix/modules/careplan/ports"
)

type RequestConflictRecordsAdapter struct {
	careRecordPorts.RequestConflictRecords
}

func (r RequestConflictRecordsAdapter) Find(ctx context.Context, id int64, lock bool) (carerequests.Request, error) {
	return r.FindCareScheduleRequest(ctx, id, lock)
}
