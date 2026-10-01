package application

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"unicode/utf8"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
)

// Decide approves (and applies) or rejects a pending request. Staleness is
// decided under the row lock and before any authorization or apply work, so
// a decision taken on an outdated view never lands (#2267).
func (s *OfferingChanges) Decide(ctx context.Context, input careplan.OfferingChangeDecisionInput) error {
	reason, err := validateDecisionInput(input)
	if err != nil {
		return err
	}
	row, err := s.lockPendingRow(ctx, input.RequestID, input.ExpectedVersion)
	if err != nil {
		return err
	}
	if err := s.authorizeDecision(ctx, row.StudentID); err != nil {
		return err
	}
	if !input.Approve {
		return s.reject(ctx, row, input, reason)
	}
	return s.approve(ctx, &row, input, reason)
}

func validateDecisionInput(input careplan.OfferingChangeDecisionInput) (string, error) {
	if input.RequestID <= 0 || input.ReviewedBy <= 0 {
		return "", fmt.Errorf("%w: request and reviewer are required", careplan.ErrOfferingChangeInvalid)
	}
	reason := strings.TrimSpace(input.Reason)
	if utf8.RuneCountInString(reason) > offeringChangeMaxNoteLen {
		return "", fmt.Errorf("%w: reason is too long", careplan.ErrOfferingChangeInvalid)
	}
	if !input.Approve && reason == "" {
		// A rejection the family cannot understand generates the phone call
		// the whole feature exists to avoid.
		return "", fmt.Errorf("%w: a rejection needs a reason", careplan.ErrOfferingChangeInvalid)
	}
	// An approval needs a reason only while the school's policy asks staff
	// for one (#2267, story 28).
	if input.Approve && input.ReasonRequired && reason == "" {
		return "", careplan.ErrParentRequestReasonRequired
	}
	return reason, nil
}

// lockPendingRow locks the request and refuses a decided or outdated one.
func (s *OfferingChanges) lockPendingRow(ctx context.Context, requestID int64, expectedVersion string) (careplan.OfferingChangeRequest, error) {
	row, err := s.deps.Rows.FindForUpdate(ctx, requestID)
	if err != nil {
		return careplan.OfferingChangeRequest{}, err
	}
	if offeringChangeTerminal(row) {
		return careplan.OfferingChangeRequest{}, careplan.ErrOfferingChangeNotPending
	}
	if expectedVersion != "" && careplan.ParentRequestVersion(row.UpdatedAt) != expectedVersion {
		return careplan.OfferingChangeRequest{}, careplan.ErrParentRequestStale
	}
	return row, nil
}

// authorizeDecision locks the child and applies the review scope. A child
// who left the OGS after filing is reported as missing (#2487).
func (s *OfferingChanges) authorizeDecision(ctx context.Context, studentID int64) error {
	student, err := s.deps.Students.LockStudent(ctx, studentID)
	if err != nil {
		return fmt.Errorf("offering change: load student for decision: %w", err)
	}
	if (student != nil && student.Alumnus) || careEnded(student, s.todayDate()) {
		return careplan.ErrOfferingChangeNotFound
	}
	allowed, err := s.reviewAllows(ctx, student)
	if err != nil {
		return err
	}
	if !allowed {
		return careplan.ErrOfferingChangeForbidden
	}
	return nil
}

func (s *OfferingChanges) reject(ctx context.Context, row careplan.OfferingChangeRequest, input careplan.OfferingChangeDecisionInput, reason string) error {
	if len(input.ExcludedAutoOfferingIDs) > 0 {
		return fmt.Errorf("%w: co-booking overrides only apply to an approval", careplan.ErrOfferingChangeInvalid)
	}
	if input.EffectiveFrom != nil {
		return fmt.Errorf("%w: a confirmed date only applies to an approval", careplan.ErrOfferingChangeInvalid)
	}
	diff, err := s.rejectionDecisionDiff(ctx, row)
	if err != nil {
		return err
	}
	if err := s.deps.Rows.Decide(ctx, row.ID, careplan.OfferingChangeRejected, &reason, &input.ReviewedBy, false); err != nil {
		return err
	}
	if err := s.storeDecisionSnapshot(ctx, row.ID, diff); err != nil {
		return err
	}
	if err := s.recordOfferingDecision(ctx, row.ID, input.ReviewedBy, false, reason); err != nil {
		return err
	}
	return s.emitDecisionPill(ctx, row, decisionPill{
		reviewedBy: input.ReviewedBy, body: offeringChangeRejectedBody,
		status: careplan.ParentMessageRequestStatusReject, reason: reason,
	})
}

// approve applies the switch and records the decision. Approving a switch
// whose effective date has passed would book offerings into a settled past:
// staff reject it or mark it done (#2267, story 14), or move the date
// forward with an explicit EffectiveFrom.
func (s *OfferingChanges) approve(ctx context.Context, row *careplan.OfferingChangeRequest, input careplan.OfferingChangeDecisionInput, reason string) error {
	if input.EffectiveFrom == nil && parentRequestIsPast(careplan.Date(row.EffectiveFrom), careplan.Date(s.todayDate())) {
		return careplan.ErrParentRequestPast
	}
	applied, err := s.applyApproved(ctx, row, input)
	if err != nil {
		return err
	}
	diff, err := materializedDecisionDiff(applied)
	if err != nil {
		return err
	}
	if err := s.deps.Rows.UpdateApprovedCompleteWithdrawal(ctx, row.ID, applied.CompleteWithdrawal); err != nil {
		return err
	}
	if err := s.deps.Rows.Decide(ctx, row.ID, careplan.OfferingChangeApproved, optionalReason(reason), &input.ReviewedBy, true); err != nil {
		return err
	}
	if err := s.storeDecisionSnapshot(ctx, row.ID, diff); err != nil {
		return err
	}
	if err := s.recordOfferingDecision(ctx, row.ID, input.ReviewedBy, true, reason); err != nil {
		return err
	}
	effectiveFrom := offeringChangeEffectiveFrom(*row)
	if err := s.emitDecisionPill(ctx, *row, decisionPill{
		reviewedBy: input.ReviewedBy, body: offeringChangeApprovedBody(effectiveFrom),
		status: careplan.ParentMessageRequestStatusDone, reason: reason,
		payload: map[string]any{"effective_from": effectiveFrom.String()},
	}); err != nil {
		return err
	}
	s.logger().Info("offering change request approved",
		slog.Int64("request_id", row.ID),
		slog.Int64("student_id", row.StudentID),
		slog.String("effective_from", effectiveFrom.String()),
	)
	return nil
}

// MarkDone closes an offering-change request whose effective date has
// passed. It applies nothing.
func (s *OfferingChanges) MarkDone(ctx context.Context, requestID int64, expectedVersion, reason string, reviewedBy int64) error {
	if requestID <= 0 || reviewedBy <= 0 {
		return fmt.Errorf("%w: request and reviewer are required", careplan.ErrOfferingChangeInvalid)
	}
	trimmed := strings.TrimSpace(reason)
	if utf8.RuneCountInString(trimmed) > offeringChangeMaxNoteLen {
		return fmt.Errorf("%w: reason is too long", careplan.ErrOfferingChangeInvalid)
	}
	row, err := s.lockPendingRow(ctx, requestID, expectedVersion)
	if err != nil {
		return err
	}
	student, err := s.deps.Students.LockStudent(ctx, row.StudentID)
	if err != nil {
		return fmt.Errorf("offering change: load student for completion: %w", err)
	}
	allowed, err := s.reviewAllows(ctx, student)
	if err != nil {
		return err
	}
	if !allowed {
		return careplan.ErrOfferingChangeForbidden
	}
	if !parentRequestIsPast(careplan.Date(row.EffectiveFrom), careplan.Date(s.todayDate())) {
		return careplan.ErrParentRequestNotPast
	}
	if err := s.deps.Rows.Decide(ctx, row.ID, careplan.OfferingChangeDone, optionalReason(trimmed), &reviewedBy, false); err != nil {
		return err
	}
	if err := s.recordOfferingRequestEvent(ctx, row, careplan.ParentRequestEventMarkedDone, reviewedBy,
		map[string]any{"reason": trimmed}); err != nil {
		return err
	}
	return s.emitDecisionPill(ctx, row, decisionPill{
		reviewedBy: reviewedBy, body: parentRequestDoneBody,
		status: careplan.ParentMessageRequestStatusDone, reason: trimmed,
	})
}
