package application

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
)

func (w *ExcusedRequests) ListHistory(ctx context.Context, filter careplan.RequestQueueFilter) (items []*careplan.ExcusedRequestHistoryItem, next *careplan.RequestCursor, err error) {
	started := time.Now()
	defer func() { w.done("excused_request_list_history", started, err) }()
	// limit+1 probes for an older page without a second count query.
	probe := probeLimit(filter)
	rows, err := w.carePlan.ListExcusedAbsenceRequests(ctx, careplan.ExcusedAbsenceRequestFilter{
		Statuses: careplan.DecidedExcusedRequestStatuses(),
		Queue:    &probe,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("care plan: list decided excused requests: %w", err)
	}
	// The cursor points at the last DB row, not the last visible item: the
	// per-child scope filters after the DB limit, so a cursor built from the
	// filtered page would skip rows.
	rows, next = nextCursor(rows, filter.Limit, func(r careplan.ExcusedAbsenceRequest) (time.Time, int64) {
		return r.UpdatedAt, r.ID
	})
	if len(rows) == 0 {
		return []*careplan.ExcusedRequestHistoryItem{}, next, nil
	}

	studentIDs, reviewerIDs := excusedHistoryParticipantIDs(rows)
	students, persons, err := w.loadStudentNames(ctx, studentIDs, "excused request history")
	if err != nil {
		return nil, nil, err
	}
	reviewers, err := w.students.ReviewerNames(ctx, reviewerIDs)
	if err != nil {
		return nil, nil, fmt.Errorf("care plan: load reviewers for excused request history: %w", err)
	}

	// Same per-child scope as ListPending: review scope plus alumnus skip.
	scope, err := w.reviewScope(ctx)
	if err != nil {
		return nil, nil, err
	}
	items = make([]*careplan.ExcusedRequestHistoryItem, 0, len(rows))
	for i := range rows {
		r := &rows[i]
		st, ok := students[r.StudentID]
		if !ok || !scope.Allows(&st) || st.Alumnus {
			continue
		}
		item := &careplan.ExcusedRequestHistoryItem{Request: r, ReviewerName: reviewerDisplayName(reviewers, r.ReviewedBy)}
		if p, ok := persons[st.PersonID]; ok {
			item.FirstName, item.LastName = p.FirstName, p.LastName
		}
		items = append(items, item)
	}
	return items, next, nil
}

func (w *ExcusedRequests) ListPending(ctx context.Context, filter careplan.RequestQueueFilter) (items []*careplan.ExcusedRequestReviewItem, next *careplan.RequestCursor, err error) {
	started := time.Now()
	defer func() { w.done("excused_request_list_pending", started, err) }()
	probe := probeLimit(filter)
	rows, err := w.listPendingForTenant(ctx, &probe)
	if err != nil {
		return nil, nil, fmt.Errorf("care plan: list pending excused requests: %w", err)
	}
	rows, next = nextCursor(rows, filter.Limit, func(r careplan.ExcusedAbsenceRequest) (time.Time, int64) {
		return r.CreatedAt, r.ID
	})
	if len(rows) == 0 {
		return []*careplan.ExcusedRequestReviewItem{}, next, nil
	}

	students, persons, err := w.loadStudentNames(ctx, excusedRequestStudentIDs(rows), "excused requests")
	if err != nil {
		return nil, nil, err
	}

	// Scope the queue to children whose absences the caller may decide, so a
	// staffer only ever sees requests they can act on. This also scopes the
	// sidebar badge.
	scope, err := w.reviewScope(ctx)
	if err != nil {
		return nil, nil, err
	}
	today, dayErr := careplan.ParseDate(filter.UrgentDate)
	if dayErr != nil {
		today = w.today()
	}
	items, err = w.pendingReviewItems(ctx, rows, students, persons, scope, today)
	if err != nil {
		return nil, nil, err
	}
	return items, next, nil
}

// excusedRequestStudentIDs lists each request's student once, in first-seen
// order.
func excusedRequestStudentIDs(rows []careplan.ExcusedAbsenceRequest) []int64 {
	studentIDs := make([]int64, 0, len(rows))
	seen := make(map[int64]struct{}, len(rows))
	for _, r := range rows {
		if _, ok := seen[r.StudentID]; !ok {
			seen[r.StudentID] = struct{}{}
			studentIDs = append(studentIDs, r.StudentID)
		}
	}
	return studentIDs
}

// pendingReviewItems builds the review queue rows for the requests the
// caller may decide.
func (w *ExcusedRequests) pendingReviewItems(
	ctx context.Context,
	rows []careplan.ExcusedAbsenceRequest,
	students map[int64]careplan.ReviewStudent,
	persons map[int64]careplan.PersonName,
	scope careplan.ReviewScope,
	today careplan.Date,
) ([]*careplan.ExcusedRequestReviewItem, error) {
	items := make([]*careplan.ExcusedRequestReviewItem, 0, len(rows))
	for i := range rows {
		r := &rows[i]
		st, ok := students[r.StudentID]
		if !ok || !scope.Allows(&st) {
			continue
		}
		// A graduated child is soft-deleted: their pending requests survive
		// the graduation, so without this they would sit in the staff queue
		// and could still be approved onto an alumnus. A child whose care has
		// ended leaves the queue the same way.
		if st.Alumnus || st.CareEndedOn(today) {
			continue
		}
		item, err := w.pendingReviewItem(ctx, r, st, today)
		if err != nil {
			return nil, err
		}
		if p, ok := persons[st.PersonID]; ok {
			item.FirstName, item.LastName = p.FirstName, p.LastName
		}
		items = append(items, item)
	}
	return items, nil
}

// pendingReviewItem resolves the current statuses and bulk eligibility of one
// pending request.
func (w *ExcusedRequests) pendingReviewItem(ctx context.Context, r *careplan.ExcusedAbsenceRequest, st careplan.ReviewStudent, today careplan.Date) (*careplan.ExcusedRequestReviewItem, error) {
	view, viewErr := w.currentStatusView(ctx, r)
	if viewErr != nil {
		return nil, fmt.Errorf("care plan: resolve current absence statuses: %w", viewErr)
	}
	eligible, reasonCode, reasonText, eligibilityErr := w.excusedBulkEligibility(ctx, r, st, view, today)
	if eligibilityErr != nil {
		return nil, fmt.Errorf("care plan: resolve absence bulk eligibility: %w", eligibilityErr)
	}
	currentValueChanged := view.changedSinceRequest
	return &careplan.ExcusedRequestReviewItem{
		Request: r, BulkEligible: eligible,
		BulkIneligibleReason: reasonCode, BulkIneligibleText: reasonText,
		CurrentStatusByDate: view.byDate, CurrentValueChanged: &currentValueChanged,
	}, nil
}

func (w *ExcusedRequests) PendingByStudentForDate(ctx context.Context, date careplan.Date) (result map[int64]*careplan.ExcusedAbsenceRequest, err error) {
	started := time.Now()
	defer func() { w.done("excused_request_pending_by_student", started, err) }()
	rows, err := w.listPendingForTenant(ctx, &careplan.RequestQueueFilter{})
	if err != nil {
		return nil, err
	}
	// rows are newest-first; keep the first (newest) pending request per
	// student that covers the date.
	candidates := make(map[int64]*careplan.ExcusedAbsenceRequest, len(rows))
	studentIDs := make([]int64, 0, len(rows))
	for i := range rows {
		r := &rows[i]
		if _, ok := candidates[r.StudentID]; ok {
			continue
		}
		if containsDate(r.Dates, date) {
			candidates[r.StudentID] = r
			studentIDs = append(studentIDs, r.StudentID)
		}
	}
	if len(candidates) == 0 {
		return candidates, nil
	}
	// Scope to children the caller may decide, the same gate as the review
	// queue, so a read-only supervisor never sees a pending-approval badge for
	// a child they cannot act on.
	students, err := w.students.FindStudents(ctx, studentIDs)
	if err != nil {
		return nil, fmt.Errorf("care plan: load students for pending excused badges: %w", err)
	}
	scope, err := w.reviewScope(ctx)
	if err != nil {
		return nil, err
	}
	today := w.today()
	out := make(map[int64]*careplan.ExcusedAbsenceRequest, len(candidates))
	for studentID, req := range candidates {
		st, ok := students[studentID]
		if ok && scope.Allows(&st) && !st.Alumnus && !st.CareEndedOn(today) {
			out[studentID] = req
		}
	}
	return out, nil
}
