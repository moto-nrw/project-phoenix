package application

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/careplan/carerequests"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/ports"
	calendar "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// RequestSubmissions runs inside the caller's authorized tenant transaction.
// Persistence, ledger, and durable notification errors abort that transaction.
type RequestSubmissions struct {
	Records ports.RequestSubmissionRecords
	Pickup  ports.RequestSubmissionPickup
	Effects ports.RequestSubmissionEffects
	Today   func() calendar.Date
}

func (s *RequestSubmissions) CreateRequest(ctx context.Context, studentID, guardianID int64, payload json.RawMessage) (*carerequests.Request, error) {
	canonical, err := carerequests.CanonicalizeWeekly(payload)
	if err != nil {
		return nil, err
	}
	return s.create(ctx, studentID, guardianID, "weekly_schedule", "care request", canonical)
}

func (s *RequestSubmissions) CreatePickupChange(ctx context.Context, input carerequests.PickupChangeCreateInput) (*carerequests.Request, error) {
	payload, err := pickupRequestPayload(ctx, s.Pickup, s.Today(), input)
	if err != nil {
		return nil, err
	}
	return s.create(ctx, input.StudentID, input.GuardianAccountID, "pickup_change", "pickup change request", payload)
}

func pickupRequestPayload(ctx context.Context, pickup ports.RequestSubmissionPickup, today calendar.Date, input carerequests.PickupChangeCreateInput) (json.RawMessage, error) {
	reason, err := carerequests.ValidatePickupChange(input, today)
	if err != nil {
		return nil, err
	}
	payload := map[string]any{"date": input.Date.String(), "pickup_time": input.PickupTime.Format("15:04"), "reason": reason}
	if pickup != nil {
		previous, readErr := pickup.PickupTime(ctx, input.StudentID, input.Date)
		if readErr != nil {
			return nil, fmt.Errorf("schedule: resolve current pickup time: %w", readErr)
		}
		if previous != nil {
			payload["previous_pickup_time"] = previous.Format("15:04")
		}
	}
	return json.Marshal(payload)
}

func (s *RequestSubmissions) create(ctx context.Context, studentID, guardianID int64, kind, operation string, payload json.RawMessage) (*carerequests.Request, error) {
	request, err := s.Records.Create(ctx, carerequests.Request{StudentID: studentID, SubmittedBy: guardianID, RequestKind: kind, Payload: payload, Status: "pending"})
	if err != nil {
		if s.Records.IsPendingConflict(err) {
			return nil, carerequests.ErrAlreadyPending
		}
		return nil, fmt.Errorf("schedule: create %s: %w", operation, err)
	}
	if err := s.Effects.RecordSubmission(ctx, &request); err != nil {
		return nil, err
	}
	if err := s.Effects.NotifySubmission(ctx, &request); err != nil {
		return nil, err
	}
	s.Effects.WakeGuardians(ctx, &request)
	return &request, nil
}
