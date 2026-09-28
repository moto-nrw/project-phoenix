package application

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
)

// The Angebote side of the parent-request conflict resolver (#2267, stories
// 6-10). Two open switch requests for the same offering always overlap:
// their validity ranges are open-ended, so they are resolved as one group.

// ConflictCandidate names the child and version of a pending request.
func (s *OfferingChanges) ConflictCandidate(ctx context.Context, requestID int64) (*careplan.OfferingConflictCandidate, error) {
	row, err := s.pendingRow(ctx, requestID)
	if err != nil {
		return nil, err
	}
	return &careplan.OfferingConflictCandidate{StudentID: row.StudentID, UpdatedAt: row.UpdatedAt}, nil
}

// pendingRow reads a request without locking it; a decided one is no longer
// part of any conflict group.
func (s *OfferingChanges) pendingRow(ctx context.Context, requestID int64) (careplan.OfferingChangeRequest, error) {
	row, err := s.deps.Rows.Find(ctx, requestID)
	if err != nil {
		return careplan.OfferingChangeRequest{}, err
	}
	if offeringChangeTerminal(row) {
		return careplan.OfferingChangeRequest{}, careplan.ErrOfferingChangeNotPending
	}
	return row, nil
}

// LockConflictRequest locks a pending request of the conflict group.
func (s *OfferingChanges) LockConflictRequest(ctx context.Context, requestID int64) error {
	_, err := s.lockPendingRow(ctx, requestID, "")
	return err
}

// DecideConflictRequest decides one request of the group like the review
// would.
func (s *OfferingChanges) DecideConflictRequest(ctx context.Context, decision careplan.OfferingConflictDecision) error {
	return s.Decide(ctx, careplan.OfferingChangeDecisionInput{
		RequestID: decision.RequestID, Approve: decision.Approve, Reason: decision.Reason,
		ReviewedBy: decision.ReviewerID, ActorRole: decision.ActorRole, ExpectedVersion: decision.ExpectedVersion,
	})
}

// WriteStaffValue books the offerings the staff member chose instead of any
// of the rejected wishes, through the same direct correction the office uses
// on the child's own screen, so it lands in the adjustment log with source
// "direct" and passes the live catalog validation.
func (s *OfferingChanges) WriteStaffValue(ctx context.Context, write careplan.OfferingStaffValueWrite) error {
	if write.EffectiveFrom.IsZero() {
		return fmt.Errorf("%w: effective_from is required", careplan.ErrOfferingChangeInvalid)
	}
	if write.Selections == nil {
		return fmt.Errorf("%w: selections are required", careplan.ErrOfferingChangeInvalid)
	}
	return s.ApplyDirectOfferingAdjustment(ctx, careplan.DirectOfferingAdjustmentInput{
		StudentID: write.StudentID, EffectiveFrom: write.EffectiveFrom, Selections: write.Selections,
		Reason: write.Reason, ActorAccountID: write.ReviewerID, ActorRole: write.ActorRole,
	})
}
