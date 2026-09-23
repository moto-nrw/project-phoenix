package application

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/carerequests"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/ports"
	calendar "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

type RequestEdits struct {
	Records ports.RequestEditRecords
	Pickup  ports.RequestSubmissionPickup
	Events  ports.RequestEditEvents
	Today   func() calendar.Date
}

// EditRequest runs in the caller's tenant transaction. Ownership precedes
// status and version checks, so another family's terminal row stays hidden.
func (s *RequestEdits) EditRequest(ctx context.Context, input carerequests.EditInput) (*carerequests.Request, error) {
	request, err := s.Records.Find(ctx, input.RequestID, true)
	if err != nil {
		return nil, err
	}
	if request.SubmittedBy != input.GuardianAccountID || request.StudentID != input.StudentID {
		return nil, careplan.ErrCareScheduleRequestNotFound
	}
	if request.Status != "pending" {
		return nil, careplan.ErrCareScheduleRequestNotPending
	}
	if input.ExpectedVersion != "" && careplan.ParentRequestVersion(request.UpdatedAt) != input.ExpectedVersion {
		return nil, careplan.ErrParentRequestStale
	}
	payload, err := s.payload(ctx, request, input)
	if err != nil {
		return nil, err
	}
	if err := s.Records.UpdatePending(ctx, request.ID, payload); err != nil {
		return nil, err
	}
	row, err := s.Records.Find(ctx, request.ID, false)
	if err != nil {
		return nil, fmt.Errorf("schedule: reload edited care request: %w", err)
	}
	if err := s.Events.RecordGuardianEdit(ctx, &row, input.GuardianAccountID); err != nil {
		return nil, err
	}
	return &row, nil
}

func (s *RequestEdits) payload(ctx context.Context, request carerequests.Request, input carerequests.EditInput) (json.RawMessage, error) {
	if request.RequestKind != "pickup_change" {
		return carerequests.CanonicalizeWeekly(input.Payload)
	}
	var stored map[string]any
	if err := json.Unmarshal(request.Payload, &stored); err != nil {
		return nil, err
	}
	if current, ok := stored["date"].(string); ok && input.Cutoff != nil {
		if day, err := calendar.ParseDate(current); err == nil && input.Cutoff.Closed(day) {
			return nil, carerequests.ErrPickupChangeCutoffPassed
		}
	}
	return pickupRequestPayload(ctx, s.Pickup, s.Today(), carerequests.PickupChangeCreateInput{
		StudentID: input.StudentID, GuardianAccountID: input.GuardianAccountID, Date: input.Date,
		PickupTime: input.PickupTime, Reason: input.Reason, ReasonRequired: input.ReasonRequired, Cutoff: input.Cutoff,
	})
}
