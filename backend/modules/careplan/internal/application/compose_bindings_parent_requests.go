package application

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/absencerecords"
	"github.com/moto-nrw/project-phoenix/modules/careplan/excusedrequests"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/ports"
	"github.com/moto-nrw/project-phoenix/modules/careplan/parentrequests"
)

// ExcusedCrossKindRequests is the part of the excused-absence workflow the
// cross-kind bulk approval and conflict resolution use.
type ExcusedCrossKindRequests interface {
	excusedrequests.BulkApproval
	excusedrequests.ConflictResolution
}

// excusedParentRequests presents the excused-absence workflow to the
// cross-kind commands in the coordinator's vocabulary.
type ExcusedParentRequests struct {
	Requests ExcusedCrossKindRequests
}

func (p ExcusedParentRequests) GetExcusedBulkCandidate(ctx context.Context, requestID int64) (*ports.ExcusedBulkCandidate, error) {
	candidate, err := p.Requests.GetExcusedBulkCandidate(ctx, requestID)
	if err != nil || candidate == nil {
		return nil, ExcusedParentRequestError(err)
	}
	return &ports.ExcusedBulkCandidate{ID: candidate.ID, StudentID: candidate.StudentID, UpdatedAt: candidate.UpdatedAt, Eligible: candidate.Eligible}, nil
}
func (p ExcusedParentRequests) LockExcusedBulkRequest(ctx context.Context, requestID int64) error {
	return ExcusedParentRequestError(p.Requests.LockExcusedBulkRequest(ctx, requestID))
}
func (p ExcusedParentRequests) ApproveExcusedBulk(ctx context.Context, requestID int64, reason string, reviewerID int64, expectedVersion string) error {
	return ExcusedParentRequestError(p.Requests.ApproveExcusedBulk(ctx, requestID, reason, reviewerID, expectedVersion))
}
func (p ExcusedParentRequests) ConflictCandidate(ctx context.Context, requestID int64) (*ports.ConflictCandidate, error) {
	candidate, err := p.Requests.ConflictCandidate(ctx, requestID)
	if err != nil {
		return nil, ExcusedParentRequestError(err)
	}
	return &ports.ConflictCandidate{StudentID: candidate.StudentID, UpdatedAt: candidate.UpdatedAt}, nil
}
func (p ExcusedParentRequests) LockConflictRequest(ctx context.Context, requestID int64) error {
	return ExcusedParentRequestError(p.Requests.LockConflictRequest(ctx, requestID))
}
func (p ExcusedParentRequests) DecideConflictRequest(ctx context.Context, decision ports.ConflictDecision) error {
	return ExcusedParentRequestError(p.Requests.DecideConflictRequest(ctx, careplan.ExcusedConflictDecision{
		RequestID: decision.RequestID, Approve: decision.Approve, Reason: decision.Reason,
		ReviewerID: decision.ReviewerID, ExpectedVersion: decision.ExpectedVersion,
	}))
}

// WriteStaffValue reads {"value": "<status>"} from the resolve payload;
// anything else is the workflow's invalid-status sentinel, never a 500.
func (p ExcusedParentRequests) WriteStaffValue(ctx context.Context, write ports.StaffValueWrite) error {
	status, ok := write.Value["value"].(string)
	if !ok {
		return careplan.ErrAbsenceRequestInvalidStatus
	}
	return ExcusedParentRequestError(p.Requests.WriteStaffValue(ctx, careplan.ExcusedStaffValueWrite{
		StudentID: write.StudentID, RequestIDs: write.RequestIDs, Reason: write.Reason, Status: status,
	}))
}

// excusedParentRequestError translates the workflow's sentinels into the ones
// the cross-kind commands and their routes match on. Domain-specific
// sentinels pass through unchanged.
func ExcusedParentRequestError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, careplan.ErrExcusedRequestNotFound):
		return absencerecords.ErrExcusedRequestNotFound
	case errors.Is(err, careplan.ErrExcusedRequestNotPending):
		return absencerecords.ErrExcusedRequestNotPending
	case errors.Is(err, careplan.ErrExcusedRequestNotDecided):
		return absencerecords.ErrExcusedRequestNotDecided
	default:
		return ParentRequestLifecycleError(err)
	}
}

// parentRequestLifecycleError maps Care Plan's cross-kind lifecycle
// sentinels onto the parent-request contract.
func ParentRequestLifecycleError(err error) error {
	for _, pair := range [][2]error{
		{careplan.ErrParentRequestStale, parentrequests.ErrStale},
		{careplan.ErrParentRequestDecisionRace, parentrequests.ErrDecisionRace},
		{careplan.ErrParentRequestReasonRequired, parentrequests.ErrReasonRequired},
		{careplan.ErrParentRequestPast, parentrequests.ErrPast},
		{careplan.ErrParentRequestNotPast, parentrequests.ErrNotPast},
		{careplan.ErrParentRequestNotDecided, parentrequests.ErrNotDecided},
		{careplan.ErrParentRequestCorrectionUnsupported, parentrequests.ErrCorrectionUnsupported},
		{careplan.ErrStaffValueUnsupported, parentrequests.ErrStaffValueUnsupported},
	} {
		if errors.Is(err, pair[0]) {
			return pair[1]
		}
	}
	return err
}
