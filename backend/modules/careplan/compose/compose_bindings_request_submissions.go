package compose

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/careplan/carerequests"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/adapters/postgres"
	careRecordPorts "github.com/moto-nrw/project-phoenix/modules/careplan/ports"
)

type retainedRequestSubmissionRecordsAdapter struct {
	careRecordPorts.RequestSubmissionRecords
}

func (r retainedRequestSubmissionRecordsAdapter) Create(ctx context.Context, request carerequests.Request) (carerequests.Request, error) {
	return r.CreateCareScheduleRequest(ctx, request)
}
func (retainedRequestSubmissionRecordsAdapter) IsPendingConflict(err error) bool {
	return postgres.IsPendingCareScheduleConflict(err)
}
