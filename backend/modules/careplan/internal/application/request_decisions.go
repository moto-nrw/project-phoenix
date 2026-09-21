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
	Records     ports.RequestDecisionRecords
	People      ports.RequestDecisionPeople
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
	request, err := s.Records.Find(ctx, id, true)
	if err != nil {
		return nil, err
	}
	if request.Status != "pending" {
		return nil, careplan.ErrCareScheduleRequestNotPending
	}
	if version != "" && careplan.ParentRequestVersion(request.UpdatedAt) != version {
		return nil, careplan.ErrParentRequestStale
	}
	student, err := s.People.LockStudent(ctx, request.StudentID)
	if err != nil {
		return nil, fmt.Errorf("schedule: load student for care request decision: %w", err)
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
	companionsChanged, exceptionID, err := s.apply(ctx, request, input)
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
	if exceptionID > 0 {
		payload["pickup_exception_id"] = exceptionID
	}
	if err := s.Effects.RecordDecision(ctx, item.Request, input.ReviewedBy, payload); err != nil {
		return nil, err
	}
	return item, nil
}

func (s *RequestDecisions) apply(ctx context.Context, request *carerequests.Request, input carerequests.DecideInput) (bool, int64, error) {
	if !input.Approve {
		return false, 0, nil
	}
	allowed, err := s.People.GuardianHasAccess(ctx, request.StudentID, request.SubmittedBy)
	if err != nil {
		return false, 0, fmt.Errorf("schedule: care request guardian link check: %w", err)
	}
	if !allowed {
		return false, 0, carerequests.ErrGuardianAccessRevoked
	}
	if request.RequestKind == "pickup_change" {
		id, err := s.Plans.ApplyPickup(ctx, request, input.ExpectedImpactToken, input.RequireImpactToken)
		return false, id, err
	}
	changed, err := s.Plans.ApplyWeekly(ctx, request, input.ReviewedBy)
	return changed, 0, err
}
