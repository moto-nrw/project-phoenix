package application

import (
	"context"
	"errors"
	"fmt"
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
	return s.studentRequests(ctx, studentID, []string{"pickup_change"}, since)
}

// studentRequests is the one read of a child's requests of the given kinds
// (all kinds for nil) created since the instant, newest first.
func (s *RequestReads) studentRequests(ctx context.Context, studentID int64, kinds []string, since time.Time) ([]carerequests.Request, error) {
	return s.Records.ListCareRequests(ctx, studentID, kinds, nil, since)
}

func (s *RequestReads) ListPickupPermissionRequests(ctx context.Context, studentID int64, since time.Time) ([]carerequests.Request, error) {
	return s.studentRequests(ctx, studentID, []string{carerequests.KindPickupPermission}, since)
}

func (s *RequestReads) FindStudentRequest(ctx context.Context, studentID, requestID int64) (*carerequests.Request, error) {
	if studentID <= 0 || requestID <= 0 {
		return nil, carerequests.ErrNotFound
	}
	request, err := s.Records.FindCareRequest(ctx, studentID, requestID)
	if errors.Is(err, carerequests.ErrNotFound) {
		return nil, carerequests.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("find care request for student: %w", err)
	}
	if request == nil || request.ID != requestID {
		return nil, carerequests.ErrNotFound
	}
	return request, nil
}
