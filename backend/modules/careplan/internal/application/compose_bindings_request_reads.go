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

func (r RequestReadRecordsAdapter) FindCareRequest(ctx context.Context, studentID, requestID int64) (*carerequests.Request, error) {
	if studentID <= 0 || requestID <= 0 {
		return nil, carerequests.ErrNotFound
	}
	filter := careplan.CareScheduleRequestFilter{
		IDs:       []int64{requestID},
		StudentID: studentID,
	}
	rows, err := r.ListCareScheduleRequests(ctx, filter)
	if err != nil {
		return nil, err
	}
	switch len(rows) {
	case 0:
		return nil, carerequests.ErrNotFound
	case 1:
		return &rows[0], nil
	default:
		return nil, carerequests.ErrNotFound
	}
}
