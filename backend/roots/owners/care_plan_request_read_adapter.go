package owners

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan/carerequests"
)

func (s *CareScheduleRequestService) GetPendingForStudent(ctx context.Context, id int64) (*carerequests.Request, []carerequests.DiffEntry, error) {
	return s.Requests.reads.GetPendingForStudent(ctx, id)
}

func (s *CareScheduleRequestService) ListPickupChangeRequests(ctx context.Context, id int64, since time.Time) ([]carerequests.Request, error) {
	return s.Requests.reads.ListPickupChangeRequests(ctx, id, since)
}
