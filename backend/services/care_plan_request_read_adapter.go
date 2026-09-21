package services

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan/carerequests"
)

func (s *careScheduleRequestService) GetPendingForStudent(ctx context.Context, id int64) (*carerequests.Request, []carerequests.DiffEntry, error) {
	return s.requests.reads.GetPendingForStudent(ctx, id)
}
func (s *careScheduleRequestService) ListPickupChangeRequests(ctx context.Context, id int64, since time.Time) ([]carerequests.Request, error) {
	return s.requests.reads.ListPickupChangeRequests(ctx, id, since)
}
