package application

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/carerequests"
	careRecordPorts "github.com/moto-nrw/project-phoenix/modules/careplan/ports"
)

// requestReadRecords translates the application's list selection into the public query filter.
type RequestReadRecordsAdapter struct {
	careRecordPorts.RequestReadRecords
}

func (r RequestReadRecordsAdapter) ListCareRequests(ctx context.Context, studentID int64, kinds, statuses []string, since time.Time) ([]carerequests.Request, error) {
	return r.ListCareScheduleRequests(ctx, careplan.CareScheduleRequestFilter{StudentID: studentID, RequestKinds: kinds, Statuses: statuses, RecentSince: since})
}
