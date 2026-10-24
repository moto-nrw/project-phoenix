package application

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
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
	// The flow that stored a signature for the plan (#4780) passes its
	// reference along; canonicalizing would drop it.
	if canonical, err = carerequests.WithSignature(canonical, carerequests.StoredSignature(payload)); err != nil {
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

// CreatePickupPermission stores a signed pickup permission request (#4780).
// previousLabel names the days the person held before, so the review and the
// frozen decision compare with the state the family saw.
func (s *RequestSubmissions) CreatePickupPermission(ctx context.Context, input carerequests.PickupPermissionCreateInput) (*carerequests.Request, error) {
	payload, err := carerequests.PickupPermissionPayload(input)
	if err != nil {
		return nil, err
	}
	return s.create(ctx, input.StudentID, input.GuardianAccountID, carerequests.KindPickupPermission, "pickup permission request", payload)
}

func pickupRequestPayload(ctx context.Context, pickup ports.RequestSubmissionPickup, today calendar.Date, input carerequests.PickupChangeCreateInput) (json.RawMessage, error) {
	reason, err := carerequests.ValidatePickupChange(input, today)
	if err != nil {
		return nil, err
	}
	payload := map[string]any{"date": input.Date.String(), "reason": reason}
	if !input.PickupTime.IsZero() {
		payload["pickup_time"] = input.PickupTime.Format("15:04")
	}
	if input.DepartureMode != "" {
		payload["departure_mode"] = input.DepartureMode
	}
	addPickupPersonAndSignature(payload, input)
	if pickup == nil {
		return json.Marshal(payload)
	}
	if !input.PickupTime.IsZero() {
		previous, readErr := pickup.PickupTime(ctx, input.StudentID, input.Date)
		if readErr != nil {
			return nil, fmt.Errorf("schedule: resolve current pickup time: %w", readErr)
		}
		if previous != nil {
			payload["previous_pickup_time"] = previous.Format("15:04")
		}
	}
	if input.DepartureMode != "" {
		previous, readErr := previousDepartureModes(ctx, pickup, input.StudentID, input.Date)
		if readErr != nil {
			return nil, fmt.Errorf("schedule: resolve current departure modes: %w", readErr)
		}
		if len(previous) > 0 {
			payload["previous_departure_modes"] = previous
		}
	}
	return json.Marshal(payload)
}

// addPickupPersonAndSignature stores the day's collector and the signed
// wording (#4780).
func addPickupPersonAndSignature(payload map[string]any, input carerequests.PickupChangeCreateInput) {
	if input.PickupPerson != nil {
		payload["pickup_person"] = map[string]any{
			"guardian_profile_id": input.PickupPerson.GuardianProfileID, "name": strings.TrimSpace(input.PickupPerson.Name),
		}
	}
	if input.Signature != nil {
		payload["signature"] = map[string]any{"id": input.Signature.SignatureID, "statement": input.Signature.Statement}
	}
}

// previousDepartureModes is the Abholart of the day before a request changes
// it: the OGS's day value (#3945), else the weekly plan's set for the plan
// weekday. A weekday without a configured set is "alone", the weekly diff's
// reading. A day outside the weekly plan has none.
func previousDepartureModes(ctx context.Context, pickup ports.RequestSubmissionPickup, studentID int64, date calendar.Date) ([]string, error) {
	mode, found, err := pickup.DayDepartureMode(ctx, studentID, date)
	if err != nil {
		return nil, err
	}
	if found {
		return []string{mode}, nil
	}
	weekday, err := calendar.PlanWeekday(ctx, date)
	if err != nil || weekday < 1 || weekday > 5 {
		return nil, err
	}
	weekly, err := pickup.WeeklyDepartureModes(ctx, studentID)
	if err != nil {
		return nil, err
	}
	modes := make([]string, 0, 2)
	modes = append(modes, weekly[planWeekdayKeys[weekday]]...)
	if len(modes) == 0 {
		modes = append(modes, careplan.DepartureModeAlone)
	}
	return modes, nil
}

// planWeekdayKeys maps a plan weekday (1 = Monday) onto the key of the
// weekly Abholart plan.
var planWeekdayKeys = [...]string{"", "mon", "tue", "wed", "thu", "fri"}

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
