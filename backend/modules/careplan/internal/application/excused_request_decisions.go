package application

import (
	"context"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/ports"
)

// decisionReason trims the staff reason and checks it against the verdict's
// requirements.
func decisionReason(input careplan.ExcusedRequestDecideInput) (string, error) {
	reason := strings.TrimSpace(input.Reason)
	if !input.Approve && reason == "" {
		return "", careplan.ErrExcusedRequestRejectReasonRequired
	}
	// An approval needs a reason only while the school's policy asks staff
	// for one; a rejection always does.
	if input.Approve && input.ReasonRequired && reason == "" {
		return "", careplan.ErrParentRequestReasonRequired
	}
	// The length bound applies to both verdicts: an approval reason is stored
	// and shown in the history exactly like a rejection reason.
	if utf8.RuneCountInString(reason) > excusedRequestMaxReasonLen {
		return "", careplan.ErrExcusedRequestRejectReasonTooLong
	}
	return reason, nil
}

// lockDecidableRequest locks the pending request and its child and checks the
// caller may still decide it.
func (w *ExcusedRequests) lockDecidableRequest(ctx context.Context, input careplan.ExcusedRequestDecideInput) (careplan.ExcusedAbsenceRequest, careplan.ReviewStudent, error) {
	// Lock the row and re-verify it is still pending so two staff deciding
	// the same request (or a decide racing the guardian's withdrawal)
	// serialize.
	req, err := w.carePlan.FindPendingExcusedAbsenceRequest(ctx, input.RequestID)
	if err != nil {
		return careplan.ExcusedAbsenceRequest{}, careplan.ReviewStudent{}, err
	}
	if input.ExpectedVersion != "" && careplan.ParentRequestVersion(req.UpdatedAt) != input.ExpectedVersion {
		return careplan.ExcusedAbsenceRequest{}, careplan.ReviewStudent{}, careplan.ErrParentRequestStale
	}
	req.AbsenceStatus = normalizedAbsenceRequestStatus(req.AbsenceStatus)

	// Per-child write authorization: decided by the parent-request review
	// scope. Taken FOR UPDATE so the alumnus gate below decides on a state a
	// concurrent grade transition cannot change underneath it. This is the
	// only student row this transaction locks, so the ascending-id order
	// every student-row locker follows is preserved.
	student, err := w.students.LockStudent(ctx, req.StudentID)
	if err != nil {
		return careplan.ExcusedAbsenceRequest{}, careplan.ReviewStudent{}, fmt.Errorf("care plan: load student for absence request decision: %w", err)
	}
	today := w.today()
	// The child graduated or left care after filing this request. Approving
	// would write absence status days for a child no longer in care: same 404
	// the rest of the child surface returns.
	if student.Alumnus || student.CareEndedOn(today) {
		return careplan.ExcusedAbsenceRequest{}, careplan.ReviewStudent{}, careplan.ErrExcusedRequestNotFound
	}
	if err := w.requireReviewer(ctx, &student); err != nil {
		return careplan.ExcusedAbsenceRequest{}, careplan.ReviewStudent{}, err
	}
	return req, student, nil
}

// absenceDecision is the stored status and the chat pill of one verdict.
type absenceDecision struct {
	status      string
	pillStatus  string
	pillBody    string
	requestType string
}

func absenceDecisionOutcome(absenceStatus string, approve bool, reason string) absenceDecision {
	_, pillBody, requestType := absenceRequestCopy(absenceStatus)
	if !approve {
		return absenceDecision{
			status:      careplan.ExcusedRequestStatusRejected,
			pillStatus:  careplan.ParentMessageRequestStatusReject,
			pillBody:    absenceRequestRejectedBody(absenceStatus) + ": " + reason,
			requestType: requestType,
		}
	}
	return absenceDecision{
		status:      careplan.ExcusedRequestStatusApproved,
		pillStatus:  careplan.ParentMessageRequestStatusDone,
		pillBody:    pillBody,
		requestType: requestType,
	}
}

// storeDecision applies an approval and stores the verdict. Apply, status
// update and after-commit hooks all run in the ambient tenant transaction. A
// mid-apply failure propagates so the whole transaction rolls back rather
// than committing a half-applied absence.
func (w *ExcusedRequests) storeDecision(ctx context.Context, req *careplan.ExcusedAbsenceRequest, student *careplan.ReviewStudent, input careplan.ExcusedRequestDecideInput, reason string) (absenceDecision, error) {
	if input.Approve {
		if err := w.validateAbsenceApproval(ctx, req, student); err != nil {
			return absenceDecision{}, err
		}
		if err := w.applyAbsenceRequest(ctx, req); err != nil {
			return absenceDecision{}, err
		}
	}
	outcome := absenceDecisionOutcome(req.AbsenceStatus, input.Approve, reason)
	reviewedBy := input.ReviewedBy
	if err := w.carePlan.DecideExcusedAbsenceRequest(ctx, careplan.ExcusedAbsenceDecision{
		// A staff reason is kept for both verdicts: without it the history
		// shows an approval with no explanation.
		ID: req.ID, Status: outcome.status, Reason: optionalReason(reason), ReviewedBy: &reviewedBy, Applied: input.Approve,
	}); err != nil {
		return absenceDecision{}, err
	}
	return outcome, nil
}

// event is the staff chat pill announcing the verdict.
func (d absenceDecision) event(reviewedBy int64, reason string) ports.RequestEvent {
	return ports.RequestEvent{
		EventType: careplan.ParentMessageEventRequestStatus, ActorKind: careplan.ParentMessageActorStaff,
		ActorAccountID: reviewedBy, Body: d.pillBody, RequestType: d.requestType,
		RequestStatus: d.pillStatus, DecisionReason: reason,
	}
}

// notifyApprovedAbsence enqueues the co-recipient absence notice for an
// approval.
func (w *ExcusedRequests) notifyApprovedAbsence(ctx context.Context, tenantID int64, req *careplan.ExcusedAbsenceRequest, reviewedBy int64) error {
	if w.notifier == nil {
		return nil
	}
	if err := w.notifier.NotifyAbsenceReported(ctx, ports.AbsenceReport{
		TenantID: tenantID, StudentIDs: []int64{req.StudentID}, Status: req.AbsenceStatus, Dates: req.Dates,
		FromParent: true, ActorAccountID: reviewedBy, ExcludedAccountIDs: []int64{req.SubmittedBy},
	}); err != nil {
		return fmt.Errorf("enqueue absence notification: %w", err)
	}
	return nil
}

// recordDecision reloads the decided request, records the decision in the
// ledger and returns the review item.
func (w *ExcusedRequests) recordDecision(ctx context.Context, requestID, personID int64, input careplan.ExcusedRequestDecideInput, reason string) (*careplan.ExcusedRequestReviewItem, error) {
	row, err := w.carePlan.FindExcusedAbsenceRequest(ctx, requestID, false)
	if err != nil {
		return nil, fmt.Errorf("care plan: reload decided absence request: %w", err)
	}
	// Inside the decision's own transaction: a rolled-back decision must not
	// leave a "decided" event behind.
	if err := w.record(ctx, ports.RequestLedgerEntry{
		StudentID: row.StudentID, RequestType: careplan.ParentRequestTypeExcusedAbsence, RequestID: row.ID,
		EventType: careplan.ParentRequestEventDecided, ActorAccountID: input.ReviewedBy, UpdatedAt: row.UpdatedAt,
		Payload: map[string]any{"approve": input.Approve, "reason": reason},
	}); err != nil {
		return nil, fmt.Errorf("care plan: record absence decision event: %w", err)
	}
	item := &careplan.ExcusedRequestReviewItem{Request: &row}
	if names, nameErr := w.students.PersonNames(ctx, []int64{personID}); nameErr == nil {
		if p, ok := names[personID]; ok {
			item.FirstName, item.LastName = p.FirstName, p.LastName
		}
	}
	return item, nil
}

// MarkDone closes a request whose days have all passed. Approving it would
// write absence days into the past and rejecting it would tell the family
// their wish was refused; neither is what happened, so this is its own
// terminal state. It applies nothing.
func (w *ExcusedRequests) MarkDone(ctx context.Context, requestID int64, expectedVersion, reason string, reviewedBy int64) (err error) {
	started := time.Now()
	defer func() { w.done("excused_request_mark_done", started, err) }()
	if requestID <= 0 {
		return careplan.ErrExcusedRequestNotFound
	}
	req, err := w.carePlan.FindPendingExcusedAbsenceRequest(ctx, requestID)
	if err != nil {
		return err
	}
	if expectedVersion != "" && careplan.ParentRequestVersion(req.UpdatedAt) != expectedVersion {
		return careplan.ErrParentRequestStale
	}
	student, err := w.students.LockStudent(ctx, req.StudentID)
	if err != nil {
		return fmt.Errorf("care plan: load student for absence request completion: %w", err)
	}
	if err := w.requireReviewer(ctx, &student); err != nil {
		return err
	}
	if !parentRequestIsPast(excusedScopeEnd(&req), w.today()) {
		return careplan.ErrParentRequestNotPast
	}
	trimmed := strings.TrimSpace(reason)
	if utf8.RuneCountInString(trimmed) > excusedRequestMaxReasonLen {
		return careplan.ErrExcusedRequestRejectReasonTooLong
	}
	var reasonPtr *string
	if trimmed != "" {
		reasonPtr = &trimmed
	}
	reviewer := reviewedBy
	if err := w.carePlan.DecideExcusedAbsenceRequest(ctx, careplan.ExcusedAbsenceDecision{
		ID: req.ID, Status: careplan.ExcusedRequestStatusDone, Reason: reasonPtr, ReviewedBy: &reviewer, Applied: false,
	}); err != nil {
		return err
	}
	if err := w.record(ctx, ports.RequestLedgerEntry{
		StudentID: req.StudentID, RequestType: careplan.ParentRequestTypeExcusedAbsence, RequestID: req.ID,
		EventType: careplan.ParentRequestEventMarkedDone, ActorAccountID: reviewedBy, UpdatedAt: req.UpdatedAt,
		Payload: map[string]any{"reason": trimmed},
	}); err != nil {
		return fmt.Errorf("care plan: record absence marked-done event: %w", err)
	}
	w.broadcastRequestTransition(ctx, req.TenantID, req.StudentID)
	_, _, requestType := absenceRequestCopy(req.AbsenceStatus)
	return w.emitRequestPill(ctx, &req, ports.RequestEvent{
		EventType: careplan.ParentMessageEventRequestStatus, ActorKind: careplan.ParentMessageActorStaff,
		ActorAccountID: reviewedBy, Body: parentRequestDoneBody, RequestType: requestType,
		RequestStatus: careplan.ParentMessageRequestStatusDone, DecisionReason: trimmed,
	})
}

// Correct rewrites a decision staff already took. The old decision is not
// erased, the ledger keeps it, but the child's record is brought in line
// with the new verdict. approved → rejected undoes the absence days this
// request wrote, and only when they are provably still its own; rejected →
// approved re-runs the ordinary apply.
func (w *ExcusedRequests) Correct(ctx context.Context, requestID int64, approve bool, expectedVersion, reason string, reviewedBy int64) (err error) {
	started := time.Now()
	defer func() { w.done("excused_request_correct", started, err) }()
	if requestID <= 0 {
		return careplan.ErrExcusedRequestNotFound
	}
	trimmed := strings.TrimSpace(reason)
	if utf8.RuneCountInString(trimmed) > excusedRequestMaxReasonLen {
		return careplan.ErrExcusedRequestRejectReasonTooLong
	}
	req, err := w.lockCorrectableRequest(ctx, requestID, expectedVersion, approve)
	if err != nil {
		return err
	}

	newStatus := careplan.ExcusedRequestStatusRejected
	if approve {
		newStatus = careplan.ExcusedRequestStatusApproved
	}
	reviewer := reviewedBy
	if err := w.carePlan.RedecideExcusedAbsenceRequest(ctx, careplan.ExcusedAbsenceDecision{
		ID: req.ID, Status: newStatus, Reason: optionalReason(trimmed), ReviewedBy: &reviewer, Applied: approve,
	}); err != nil {
		return err
	}
	// The ledger keeps both decisions: the correction is a new entry naming
	// what it replaced, never an edit of the old one.
	if err := w.record(ctx, ports.RequestLedgerEntry{
		StudentID: req.StudentID, RequestType: careplan.ParentRequestTypeExcusedAbsence, RequestID: req.ID,
		EventType: careplan.ParentRequestEventCorrected, ActorAccountID: reviewedBy, UpdatedAt: req.UpdatedAt,
		Payload: correctionEventPayload(approve, trimmed, req.Status, newStatus, req.ReviewedBy, req.DecisionReason),
	}); err != nil {
		return fmt.Errorf("care plan: record absence correction event: %w", err)
	}
	w.broadcastRequestTransition(ctx, req.TenantID, req.StudentID)
	pillBody, pillStatus := correctedAbsencePill(req.AbsenceStatus, approve)
	_, _, requestType := absenceRequestCopy(req.AbsenceStatus)
	return w.emitRequestPill(ctx, &req, ports.RequestEvent{
		EventType: careplan.ParentMessageEventRequestStatus, ActorKind: careplan.ParentMessageActorStaff,
		ActorAccountID: reviewedBy, Body: pillBody, RequestType: requestType,
		RequestStatus: pillStatus, DecisionReason: trimmed,
	})
}

// lockCorrectableRequest locks a decided request and its child, re-checks the
// caller's scope and brings the child's absence days in line with the new
// verdict.
func (w *ExcusedRequests) lockCorrectableRequest(ctx context.Context, requestID int64, expectedVersion string, approve bool) (careplan.ExcusedAbsenceRequest, error) {
	req, err := w.carePlan.FindExcusedAbsenceRequest(ctx, requestID, true)
	if err != nil {
		return careplan.ExcusedAbsenceRequest{}, err
	}
	if !isCorrectableExcusedStatus(req.Status) {
		return careplan.ExcusedAbsenceRequest{}, careplan.ErrParentRequestNotDecided
	}
	if expectedVersion != "" && careplan.ParentRequestVersion(req.UpdatedAt) != expectedVersion {
		return careplan.ExcusedAbsenceRequest{}, careplan.ErrParentRequestStale
	}
	req.AbsenceStatus = normalizedAbsenceRequestStatus(req.AbsenceStatus)
	// Re-authorize against the current scope: a group leader who has since
	// lost the group must not be able to correct its decisions.
	student, err := w.students.LockStudent(ctx, req.StudentID)
	if err != nil {
		return careplan.ExcusedAbsenceRequest{}, fmt.Errorf("care plan: load student for absence request correction: %w", err)
	}
	if err := w.requireReviewer(ctx, &student); err != nil {
		return careplan.ExcusedAbsenceRequest{}, err
	}
	if err := w.correctAbsenceDays(ctx, &req, &student, approve); err != nil {
		return careplan.ExcusedAbsenceRequest{}, err
	}
	return req, nil
}

// correctAbsenceDays re-runs the ordinary apply for rejected → approved and
// undoes the request's own absence days for approved → rejected.
func (w *ExcusedRequests) correctAbsenceDays(ctx context.Context, req *careplan.ExcusedAbsenceRequest, student *careplan.ReviewStudent, approve bool) error {
	if !approve {
		return w.revertApprovedAbsence(ctx, req)
	}
	if err := w.validateAbsenceApproval(ctx, req, student); err != nil {
		return err
	}
	return w.applyAbsenceRequest(ctx, req)
}
