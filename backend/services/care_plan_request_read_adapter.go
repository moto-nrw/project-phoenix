package services

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan/carerequests"
	"github.com/moto-nrw/project-phoenix/modules/careplan/compose"
)

func (s *careScheduleRequestService) reads() carerequests.Reads {
	service, err := compose.NewRequestReads(s.requestRecords, s.diffs(), s.logger)
	if err != nil {
		panic(err)
	}
	return service
}

func (s *careScheduleRequestService) GetPendingForStudent(ctx context.Context, id int64) (*carerequests.Request, []carerequests.DiffEntry, error) {
	return s.reads().GetPendingForStudent(ctx, id)
}
func (s *careScheduleRequestService) ListPickupChangeRequests(ctx context.Context, id int64, since time.Time) ([]carerequests.Request, error) {
	return s.reads().ListPickupChangeRequests(ctx, id, since)
}
