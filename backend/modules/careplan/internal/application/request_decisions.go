package application

import (
	"context"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/carerequests"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/ports"
	calendar "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

const maxDecisionReasonRunes = 2000

type RequestDecisions struct {
	Records ports.RequestDecisionRecords
	People  ports.RequestDecisionPeople
	// Guardians answers whether the submitting guardian still holds the
	// child. Nil skips the check: a graph without parent messaging has no
	// portal guardians, as in the excused-absence twin.
	Guardians   ports.GuardianAccess
	Plans       ports.RequestDecisionPlans
	Effects     ports.RequestDecisionEffects
	Transaction ports.RequestDecisionTransaction
	Today       func() calendar.Date
}

func (s *RequestDecisions) Decide(ctx context.Context, input carerequests.DecideInput) (*carerequests.ReviewItem, error) {
	reason, err := validateRequestDecision(input)
	if err != nil {
		return nil, err
	}
	var result *carerequests.ReviewItem
	err = s.Transaction.RunInTx(ctx, func(txCtx context.Context) error {
		var decisionErr error
		result, decisionErr = s.decide(txCtx, input, reason)
		return decisionErr
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func validateRequestDecision(input carerequests.DecideInput) (string, error) {
	if input.RequestID <= 0 {
		return "", careplan.ErrCareScheduleRequestNotFound
	}
	reason := strings.TrimSpace(input.Reason)
	if !input.Approve && reason == "" {
		return "", carerequests.ErrRejectReasonRequired
	}
	if input.Approve && input.ReasonRequired && reason == "" {
		return "", careplan.ErrParentRequestReasonRequired
	}
	if utf8.RuneCountInString(reason) > maxDecisionReasonRunes {
		return "", carerequests.ErrRejectReasonTooLong
	}
	return reason, nil
}

func (s *RequestDecisions) loadAuthorized(ctx context.Context, id int64, version string) (*carerequests.Request, error) {
	request, student, err := lockCareRequestStudent(ctx, s.Records, s.People, id, func(row carerequests.Request) error { return validatePendingCareRequest(row, version) })
	if err != nil {
		return nil, err
	}
	if student.Alumnus || student.CareEndedOn(domain.Date(s.Today())) {
		return nil, careplan.ErrCareScheduleRequestNotFound
	}
	allowed, err := s.People.CanReview(ctx, student)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, carerequests.ErrCareRequestForbidden
	}
	return &request, nil
}

func (s *RequestDecisions) decide(ctx context.Context, input carerequests.DecideInput, reason string) (*carerequests.ReviewItem, error) {
	request, err := s.loadAuthorized(ctx, input.RequestID, input.ExpectedVersion)
	if err != nil {
		return nil, err
	}
	snapshot := s.Plans.Snapshot(ctx, request)
	companionsChanged, applied, err := s.apply(ctx, request, input)
	if err != nil {
		return nil, err
	}
	status := "rejected"
	if input.Approve {
		status = "approved"
	}
	var reasonPtr *string
	if reason != "" {
		reasonPtr = &reason
	}
	if err := s.Records.Decide(ctx, request.ID, status, reasonPtr, input.ReviewedBy, input.Approve); err != nil {
		return nil, err
	}
	if snapshot != nil {
		if err := s.Records.StoreSnapshot(ctx, request.ID, snapshot); err != nil {
			return nil, fmt.Errorf("schedule: store care request decision snapshot: %w", err)
		}
	}
	if err := s.Effects.RegisterDecision(ctx, request, input, reason, companionsChanged); err != nil {
		return nil, err
	}
	item, err := s.Effects.ReloadDecision(ctx, request.ID)
	if err != nil {
		return nil, err
	}
	payload := map[string]any{"approve": input.Approve, "reason": reason}
	appliedLedgerPayload(payload, applied)
	if err := s.Effects.RecordDecision(ctx, item.Request, input.ReviewedBy, payload); err != nil {
		return nil, err
	}
	return item, nil
}

// appliedLedgerPayload records the rows an approval wrote; a correction
// reads them back from the ledger to take exactly those back.
func appliedLedgerPayload(payload map[string]any, applied carerequests.PickupApplied) {
	if applied.PickupExceptionID > 0 {
		payload["pickup_exception_id"] = applied.PickupExceptionID
	}
	if applied.DepartureExceptionID > 0 {
		payload["departure_exception_id"] = applied.DepartureExceptionID
	}
}

func (s *RequestDecisions) apply(ctx context.Context, request *carerequests.Request, input carerequests.DecideInput) (bool, carerequests.PickupApplied, error) {
	var none carerequests.PickupApplied
	if !input.Approve {
		return false, none, nil
	}
	if err := s.guardianStillHoldsChild(ctx, request); err != nil {
		return false, none, err
	}
	if request.RequestKind == "pickup_change" {
		applied, err := s.Plans.ApplyPickup(ctx, request, input.ExpectedImpactToken, input.RequireImpactToken)
		return false, applied, err
	}
	changed, err := s.Plans.ApplyWeekly(ctx, request, input.ReviewedBy)
	return changed, none, err
}

// guardianStillHoldsChild refuses to apply a request whose guardian lost the
// child since filing: approving would write a plan and post a pill for a
// recipient the parent APIs now hide. Staff wind such a request down by
// rejecting it.
func (s *RequestDecisions) guardianStillHoldsChild(ctx context.Context, request *carerequests.Request) error {
	if s.Guardians == nil {
		return nil
	}
	allowed, err := s.Guardians.GuardianHasChildAccess(ctx, request.StudentID, request.SubmittedBy)
	if err != nil {
		return fmt.Errorf("schedule: care request guardian link check: %w", err)
	}
	if !allowed {
		return carerequests.ErrGuardianAccessRevoked
	}
	return nil
}

// lockCareRequestStudent acquires class(shared) -> child -> request, matching
// care end. The unlocked read supplies only the immutable child ID. Callers
// check status, version and authorization against the locked state.
func lockCareRequestStudent(ctx context.Context, records ports.RequestConflictRecords, people ports.RequestDecisionPeople, id int64, validate func(carerequests.Request) error) (carerequests.Request, careplan.ReviewStudent, error) {
	snapshot, err := records.Find(ctx, id, false)
	if err != nil {
		return carerequests.Request{}, careplan.ReviewStudent{}, err
	}
	if err := validate(snapshot); err != nil {
		return carerequests.Request{}, careplan.ReviewStudent{}, err
	}
	student, err := people.LockStudent(ctx, snapshot.StudentID)
	if err != nil {
		return carerequests.Request{}, careplan.ReviewStudent{}, fmt.Errorf("schedule: lock care request student: %w", err)
	}
	row, err := records.Find(ctx, id, true)
	if err != nil {
		return carerequests.Request{}, careplan.ReviewStudent{}, err
	}
	return row, student, validate(row)
}

func validatePendingCareRequest(row carerequests.Request, version string) error {
	if row.Status != "pending" {
		return careplan.ErrCareScheduleRequestNotPending
	}
	if version != "" && careplan.ParentRequestVersion(row.UpdatedAt) != version {
		return careplan.ErrParentRequestStale
	}
	return nil
}
