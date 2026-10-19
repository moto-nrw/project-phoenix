package application

import (
	"context"
	"encoding/json"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/carerequests"
	careRecordPorts "github.com/moto-nrw/project-phoenix/modules/careplan/ports"
)

type RequestDecisionRecordsAdapter struct {
	careRecordPorts.RequestDecisionRecords
}

func (r RequestDecisionRecordsAdapter) Find(ctx context.Context, id int64, lock bool) (carerequests.Request, error) {
	return r.FindCareScheduleRequest(ctx, id, lock)
}
func (r RequestDecisionRecordsAdapter) Decide(ctx context.Context, id int64, status string, reason *string, actorID int64, applied bool) error {
	return r.DecideCareScheduleRequest(ctx, careplan.CareScheduleRequestDecision{ID: id, Status: status, Reason: reason, ReviewedBy: &actorID, Applied: applied})
}
func (r RequestDecisionRecordsAdapter) StoreSnapshot(ctx context.Context, id int64, snapshot *carerequests.DecisionSnapshot) error {
	raw, err := json.Marshal(snapshot)
	if err != nil {
		return err
	}
	return r.UpdateCareScheduleRequestSnapshot(ctx, id, raw)
}
