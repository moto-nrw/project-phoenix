package application

import (
	"context"
	"log/slog"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan/carerequests"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/ports"
)

type RequestReads struct {
	Records ports.RequestReadRecords
	Diffs   carerequests.Diffs
	Logger  *slog.Logger
}

func (s *RequestReads) GetPendingForStudent(ctx context.Context, studentID int64) (*carerequests.Request, []carerequests.DiffEntry, error) {
	request, err := s.Records.PendingWeekly(ctx, studentID)
	if err != nil || request == nil {
		return nil, nil, err
	}
	diff, err := s.Diffs.Weekly(ctx, studentID, request.Payload)
	if err != nil {
		s.Logger.Warn("schedule: build care request diff failed",
			"request_id", request.ID,
			"error", err.Error(),
		)
		return request, nil, nil
	}
	return request, diff, nil
}

func (s *RequestReads) ListPickupChangeRequests(ctx context.Context, studentID int64, since time.Time) ([]carerequests.Request, error) {
	return s.Records.RecentPickup(ctx, studentID, since)
}
