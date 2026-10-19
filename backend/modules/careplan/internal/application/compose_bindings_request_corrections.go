package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/carerequests"
	careRecordPorts "github.com/moto-nrw/project-phoenix/modules/careplan/ports"
)

type RequestCorrectionRecordsAdapter struct {
	careRecordPorts.RequestCorrectionRecords
}

func (r RequestCorrectionRecordsAdapter) Find(ctx context.Context, id int64, lock bool) (carerequests.Request, error) {
	return r.FindCareScheduleRequest(ctx, id, lock)
}
func (r RequestCorrectionRecordsAdapter) Redecide(ctx context.Context, id int64, status string, reason *string, actorID int64, applied bool) error {
	return r.RedecideCareScheduleRequest(ctx, careplan.CareScheduleRequestDecision{ID: id, Status: status, Reason: reason, ReviewedBy: &actorID, Applied: applied})
}
func (r RequestCorrectionRecordsAdapter) FindException(ctx context.Context, id int64) (*careplan.PickupException, error) {
	row, err := r.FindPickupException(ctx, id, true)
	if err != nil {
		return nil, err
	}
	return &row, nil
}
func (r RequestCorrectionRecordsAdapter) DeleteException(ctx context.Context, id int64) error {
	return r.DeletePickupException(ctx, id)
}
func (r RequestCorrectionRecordsAdapter) FindDepartureException(ctx context.Context, id int64) (*careplan.DepartureException, error) {
	rows, err := r.ListDepartureExceptions(ctx, careplan.StudentScheduleFilter{IDs: []int64{id}, LockForUpdate: true})
	if err != nil || len(rows) == 0 {
		return nil, err
	}
	return &rows[0], nil
}
