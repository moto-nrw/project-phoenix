package application

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
)

// GetExcusedBulkCandidate reads the named requests of a bulk approval
// before any lock (#5804): one request read, one student read and the
// queue page's batched status and partial-absence facts. Each request is
// still judged on its own, in the order of requestIDs. A request that is
// gone, decided, outside the caller's scope or of a child no longer in care
// is absent from the result.
func (w *ExcusedRequests) GetExcusedBulkCandidate(ctx context.Context, requestIDs []int64) (map[int64]careplan.ExcusedBulkCandidate, error) {
	candidates := make(map[int64]careplan.ExcusedBulkCandidate, len(requestIDs))
	if len(requestIDs) == 0 {
		return candidates, nil
	}
	rows, err := w.carePlan.ListExcusedAbsenceRequests(ctx, careplan.ExcusedAbsenceRequestFilter{
		IDs: requestIDs, Statuses: []string{careplan.ExcusedRequestStatusPending},
	})
	if err != nil || len(rows) == 0 {
		return candidates, err
	}
	studentIDs := make([]int64, 0, len(rows))
	for _, row := range rows {
		studentIDs = append(studentIDs, row.StudentID)
	}
	students, err := w.students.FindStudents(ctx, uniquePositive(studentIDs))
	if err != nil {
		return nil, err
	}
	scope, err := w.reviewScope(ctx)
	if err != nil {
		return nil, err
	}
	today := w.today()
	visible := visibleExcusedRequests(rows, students, scope, today)
	facts := w.loadAbsenceReviewFacts(ctx, visible, students, today)
	byID := make(map[int64]careplan.ExcusedAbsenceRequest, len(visible))
	for _, row := range visible {
		byID[row.ID] = row
	}
	for _, id := range requestIDs {
		req, ok := byID[id]
		if !ok {
			continue
		}
		view, err := facts.statusView(ctx, w, &req)
		if err != nil {
			return nil, err
		}
		eligibility, err := w.excusedBulkEligibility(ctx, &req, students[req.StudentID], view, today, facts)
		if err != nil {
			return nil, err
		}
		candidates[id] = careplan.ExcusedBulkCandidate{ID: req.ID, StudentID: req.StudentID, UpdatedAt: req.UpdatedAt, Eligible: eligibility.eligible}
	}
	return candidates, nil
}

func (w *ExcusedRequests) LockExcusedBulkRequest(ctx context.Context, requestID int64) error {
	if _, err := w.lockRequestChild(ctx, requestID); err != nil {
		return err
	}
	_, err := w.carePlan.FindPendingExcusedAbsenceRequest(ctx, requestID)
	return err
}

func (w *ExcusedRequests) ApproveExcusedBulk(ctx context.Context, requestID int64, reason string, reviewerID int64, expectedVersion string) error {
	_, err := w.Decide(ctx, careplan.ExcusedRequestDecideInput{
		RequestID: requestID, Approve: true, Reason: reason, ReviewedBy: reviewerID, ExpectedVersion: expectedVersion,
	})
	if errors.Is(err, careplan.ErrExcusedRequestNotPending) {
		return careplan.ErrParentRequestDecisionRace
	}
	return err
}

// Conflict side: two open requests for the same day contradict each other,
// and the cross-kind resolver closes the whole day at once. This workflow
// only supplies the payload.

func (w *ExcusedRequests) ConflictCandidate(ctx context.Context, requestID int64) (*careplan.ExcusedConflictCandidate, error) {
	req, err := w.carePlan.FindExcusedAbsenceRequest(ctx, requestID, false)
	if err != nil {
		return nil, err
	}
	if req.Status != careplan.ExcusedRequestStatusPending {
		return nil, careplan.ErrExcusedRequestNotPending
	}
	return &careplan.ExcusedConflictCandidate{StudentID: req.StudentID, UpdatedAt: req.UpdatedAt}, nil
}

func (w *ExcusedRequests) LockConflictRequest(ctx context.Context, requestID int64) error {
	return w.LockExcusedBulkRequest(ctx, requestID)
}

func (w *ExcusedRequests) DecideConflictRequest(ctx context.Context, decision careplan.ExcusedConflictDecision) error {
	_, err := w.Decide(ctx, careplan.ExcusedRequestDecideInput{
		RequestID: decision.RequestID, Approve: decision.Approve, Reason: decision.Reason,
		ReviewedBy: decision.ReviewerID, ExpectedVersion: decision.ExpectedVersion,
	})
	return err
}

// WriteStaffValue records the staff member's own verdict for exactly the
// days the rejected requests fought over, as a manual entry. The days come
// from the requests, never from the client.
func (w *ExcusedRequests) WriteStaffValue(ctx context.Context, write careplan.ExcusedStaffValueWrite) (err error) {
	started := time.Now()
	defer func() { w.done("excused_request_write_staff_value", started, err) }()
	status, err := staffAbsenceStatus(write.Status)
	if err != nil {
		return err
	}
	dates, err := w.conflictGroupDates(ctx, write.RequestIDs)
	if err != nil {
		return err
	}
	if len(dates) == 0 {
		return careplan.ErrStaffValueUnsupported
	}
	now := time.Now()
	// Clear every status the staff verdict is not, then write the one it is.
	// Clearing first keeps a day from carrying two active statuses for the
	// instant between the two writes.
	for _, other := range careplan.StudentStatusDayStatusesExcept(status) {
		if err := w.carePlan.ClearStudentStatusDays(ctx, write.StudentID, other, dates, now, careplan.StudentStatusSourceManual); err != nil {
			return fmt.Errorf("care plan: clear status days for staff value: %w", err)
		}
	}
	if status == careplan.StudentStatusDayPresent {
		return nil
	}
	reason := write.Reason
	for _, date := range dates {
		if _, err := w.carePlan.UpsertStudentStatusDay(ctx, careplan.StudentStatusDay{
			StudentID: write.StudentID, Date: date, Status: status, ReportedAt: now,
			Source: careplan.StudentStatusSourceManual, Note: &reason,
		}); err != nil {
			return fmt.Errorf("care plan: write status day for staff value: %w", err)
		}
	}
	return nil
}

// conflictGroupDates is the union of the days the group's requests covered,
// deduplicated and sorted so the writes run in a stable order.
func (w *ExcusedRequests) conflictGroupDates(ctx context.Context, requestIDs []int64) ([]careplan.Date, error) {
	if len(requestIDs) == 0 {
		return nil, nil
	}
	rows, err := w.carePlan.ListExcusedAbsenceRequests(ctx, careplan.ExcusedAbsenceRequestFilter{IDs: requestIDs})
	if err != nil {
		return nil, fmt.Errorf("care plan: load conflict group dates: %w", err)
	}
	requests := make(map[int64]*careplan.ExcusedAbsenceRequest, len(rows))
	for i := range rows {
		requests[rows[i].ID] = &rows[i]
	}
	var dates []careplan.Date
	for _, requestID := range requestIDs {
		if req, ok := requests[requestID]; ok {
			dates = append(dates, req.Dates...)
		}
	}
	return dedupeSortedDates(dates), nil
}

func staffAbsenceStatus(value string) (string, error) {
	switch value {
	case careplan.StudentStatusDayPresent, careplan.StudentStatusDaySick, careplan.StudentStatusDayExcused, careplan.StudentStatusDayClassTrip:
		return value, nil
	default:
		return "", careplan.ErrAbsenceRequestInvalidStatus
	}
}
