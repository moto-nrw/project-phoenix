package application

import (
	"context"
	"encoding/json"

	"github.com/moto-nrw/project-phoenix/modules/careplan/carerequests"
	careRecordPorts "github.com/moto-nrw/project-phoenix/modules/careplan/ports"
)

type RequestEditRecordsAdapter struct {
	careRecordPorts.RequestEditRecords
}

func (r RequestEditRecordsAdapter) Find(ctx context.Context, id int64, lock bool) (carerequests.Request, error) {
	return r.FindCareScheduleRequest(ctx, id, lock)
}
func (r RequestEditRecordsAdapter) UpdatePending(ctx context.Context, id int64, payload json.RawMessage) error {
	return r.UpdatePendingCareScheduleRequest(ctx, id, payload)
}
