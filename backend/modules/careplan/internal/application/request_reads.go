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
	rows, err := s.Records.ListCareRequests(ctx, studentID, []string{"weekly_schedule"}, []string{"pending"}, time.Time{})
	if err != nil || len(rows) == 0 {
		return nil, nil, err
	}
	request := &rows[0]
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
	return s.Records.ListCareRequests(ctx, studentID, []string{"pickup_change"}, nil, since)
}

func (s *RequestReads) ListPickupPermissionRequests(ctx context.Context, studentID int64, since time.Time) ([]carerequests.Request, error) {
	return s.Records.ListCareRequests(ctx, studentID, []string{carerequests.KindPickupPermission}, nil, since)
}

func (s *RequestReads) FindStudentRequest(ctx context.Context, studentID, requestID int64) (*carerequests.Request, error) {
	rows, err := s.Records.ListCareRequests(ctx, studentID, nil, nil, time.Time{})
	if err != nil {
		return nil, err
	}
	for i := range rows {
		if rows[i].ID == requestID {
			return &rows[i], nil
		}
	}
	return nil, carerequests.ErrNotFound
}
