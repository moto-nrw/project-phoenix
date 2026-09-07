package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/ports"
)

// excusedRequestMaxReasonLen bounds notes and staff reasons (in runes).
const excusedRequestMaxReasonLen = 2000

// excusedRequestRefTable names the request row for chat pills.
const excusedRequestRefTable = "active.excused_absence_requests"

// German pill texts. The staff portal renders these directly; the parents
// portal localizes from the structured event fields instead.
const (
	excusedRequestCreatedBody   = "Anfrage: Entschuldigte Abmeldung"
	excusedRequestConfirmedBody = "Abmeldung bestätigt"
	excusedRequestRejectedBody  = "Abmeldung abgelehnt"
	sickRequestCreatedBody      = "Anfrage: Krankmeldung"
	sickRequestConfirmedBody    = "Krankmeldung bestätigt"
	sickRequestRejectedBody     = "Krankmeldung abgelehnt"
	// parentRequestDoneBody is the neutral close for a request whose days have
	// passed: nothing was applied and nothing was refused.
	parentRequestDoneBody = "Anfrage abgeschlossen"
)

// ExcusedRequestDependencies wires the excused-absence workflow. CarePlan,
// Students, Scope, Hooks, and Today are required; the effect ports may be nil
// and are then skipped, exactly like the legacy service tolerated unwired
// effects.
type ExcusedRequestDependencies struct {
	CarePlan    careplan.Capability
	Students    ports.ExcusedRequestStudents
	Scope       ports.ReviewScopeResolver
	Hooks       ports.TransactionHooks
	Today       func() careplan.Date
	Observe     ports.Observer
	Logger      *slog.Logger
	Messenger   ports.RequestMessenger
	Broadcaster ports.StaffBroadcaster
	Notifier    ports.AbsenceNotifier
	Ledger      ports.RequestLedger
	Shares      ports.ShareVisibility
}

// ExcusedRequests is the student excused-absence request workflow. A pending
// request changes no status day; the child stays expected until staff
// approve it in the central request queue. Every method runs inside the
// ambient tenant transaction the caller opened.
type ExcusedRequests struct {
	carePlan    careplan.Capability
	students    ports.ExcusedRequestStudents
	scope       ports.ReviewScopeResolver
	hooks       ports.TransactionHooks
	today       func() careplan.Date
	observe     ports.Observer
	logger      *slog.Logger
	messenger   ports.RequestMessenger
	broadcaster ports.StaffBroadcaster
	notifier    ports.AbsenceNotifier
	ledger      ports.RequestLedger
	shares      ports.ShareVisibility
}

func NewExcusedRequests(deps ExcusedRequestDependencies) (*ExcusedRequests, error) {
	if deps.CarePlan == nil || deps.Students == nil || deps.Scope == nil || deps.Hooks == nil || deps.Today == nil {
		return nil, errors.New("care plan excused requests: care plan, student directory, review scope, transaction hooks, and clock are required")
	}
	if deps.Observe == nil {
		deps.Observe = func(ports.Observation) {}
	}
	if deps.Logger == nil {
		deps.Logger = slog.Default()
	}
	return &ExcusedRequests{
		carePlan: deps.CarePlan, students: deps.Students, scope: deps.Scope, hooks: deps.Hooks,
		today: deps.Today, observe: deps.Observe, logger: deps.Logger,
		messenger: deps.Messenger, broadcaster: deps.Broadcaster, notifier: deps.Notifier,
		ledger: deps.Ledger, shares: deps.Shares,
	}, nil
}

func (w *ExcusedRequests) done(operation string, started time.Time, err error) {
	w.observe(ports.Observation{Operation: operation, Duration: time.Since(started), Err: err})
}

var _ careplan.ExcusedAbsenceRequests = (*ExcusedRequests)(nil)

// --- create and edit

func (w *ExcusedRequests) CreateRequest(ctx context.Context, studentID, guardianAccountID int64, dates []careplan.Date, note string) (*careplan.ExcusedAbsenceRequest, error) {
	return w.CreateRequestForStatus(ctx, studentID, guardianAccountID, dates, note, careplan.StudentStatusDayExcused)
}

// CreateRequestForStatus keeps the mandatory-note behaviour every existing
// caller relies on. A caller that has resolved the reason policy uses Create
// instead and says whether the note is required.
func (w *ExcusedRequests) CreateRequestForStatus(ctx context.Context, studentID, guardianAccountID int64, dates []careplan.Date, note, absenceStatus string) (*careplan.ExcusedAbsenceRequest, error) {
	return w.Submit(ctx, careplan.ExcusedRequestCreateInput{
		StudentID: studentID, GuardianAccountID: guardianAccountID, Dates: dates, Note: note,
		AbsenceStatus: absenceStatus, NoteRequired: true,
	})
}

func (w *ExcusedRequests) Submit(ctx context.Context, input careplan.ExcusedRequestCreateInput) (result *careplan.ExcusedAbsenceRequest, err error) {
	started := time.Now()
	defer func() { w.done("excused_request_create", started, err) }()
	sorted, trimmed, err := validateAbsenceRequestInput(input.Dates, input.Note, input.AbsenceStatus, input.NoteRequired)
	if err != nil {
		return nil, err
	}
	if err := w.ensureNoPartialAbsence(ctx, input.StudentID, sorted); err != nil {
		return nil, err
	}
	existing, err := w.findMatchingPendingRequest(ctx, input.StudentID, sorted, input.AbsenceStatus)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return existing, nil
	}
	created, err := w.carePlan.CreateExcusedAbsenceRequest(ctx, careplan.ExcusedAbsenceRequest{
		StudentID: input.StudentID, SubmittedBy: input.GuardianAccountID, Dates: sorted, Note: trimmed,
		AbsenceStatus: input.AbsenceStatus, Status: careplan.ExcusedRequestStatusPending,
	})
	if err != nil {
		return nil, fmt.Errorf("care plan: create absence request: %w", err)
	}
	req := &created
	if err := w.record(ctx, ports.RequestLedgerEntry{
		StudentID: req.StudentID, RequestType: careplan.ParentRequestTypeExcusedAbsence, RequestID: req.ID,
		EventType: careplan.ParentRequestEventSubmitted, ActorAccountID: input.GuardianAccountID, UpdatedAt: req.UpdatedAt,
		Payload: map[string]any{"status": input.AbsenceStatus, "days": len(sorted)},
	}); err != nil {
		return nil, fmt.Errorf("care plan: record absence request event: %w", err)
	}
	// A new pending request adds the "Freigabe ausstehend" badge on the child;
	// wake staff tabs so planning/search views pick it up without a manual
	// refetch.
	w.broadcastRequestTransition(ctx, req.TenantID, req.StudentID)
	if err := w.emitCreatedRequestPill(ctx, req, input.GuardianAccountID); err != nil {
		return nil, err
	}
	return req, nil
}

// validateAbsenceRequestInput is the shared gate of the create and the
// guardian edit path. noteRequired carries the school's reason policy: a
// school that asks nobody for a reason accepts a blank note, everything else
// about the request is validated the same either way.
func validateAbsenceRequestInput(dates []careplan.Date, note, absenceStatus string, noteRequired bool) ([]careplan.Date, string, error) {
	if len(dates) == 0 {
		return nil, "", careplan.ErrExcusedRequestNoDates
	}
	if absenceStatus != careplan.StudentStatusDaySick && absenceStatus != careplan.StudentStatusDayExcused {
		return nil, "", careplan.ErrAbsenceRequestInvalidStatus
	}
	trimmed := strings.TrimSpace(note)
	if trimmed == "" && noteRequired {
		return nil, "", careplan.ErrExcusedRequestEmptyNote
	}
	if utf8.RuneCountInString(trimmed) > excusedRequestMaxReasonLen {
		return nil, "", careplan.ErrExcusedRequestNoteTooLong
	}
	return dedupeSortedDates(dates), trimmed, nil
}

func (w *ExcusedRequests) findMatchingPendingRequest(ctx context.Context, studentID int64, dates []careplan.Date, absenceStatus string) (*careplan.ExcusedAbsenceRequest, error) {
	// Lock after the care-day locks so concurrent partial and full-day writes use
	// one ordering. The advisory lock serializes the read-then-insert below.
	if err := w.carePlan.LockExcusedAbsenceRequests(ctx, studentID); err != nil {
		return nil, err
	}
	existing, err := w.listPendingForStudent(ctx, studentID)
	if err != nil {
		return nil, fmt.Errorf("care plan: check existing pending absence requests: %w", err)
	}
	for _, pending := range existing {
		sameStatus := normalizedAbsenceRequestStatus(pending.AbsenceStatus) == absenceStatus
		if sameStatus && sameDateSet(pending.Dates, dates) {
			return pending, nil
		}
		if datesIntersect(pending.Dates, dates) {
			return nil, careplan.ErrExcusedRequestOverlap
		}
	}
	return nil, nil
}

func (w *ExcusedRequests) emitCreatedRequestPill(ctx context.Context, req *careplan.ExcusedAbsenceRequest, guardianAccountID int64) error {
	createdBody, _, requestType := absenceRequestCopy(req.AbsenceStatus)
	return w.emitRequestPillAfterCommit(ctx, req, ports.RequestEvent{
		EventType: careplan.ParentMessageEventRequestCreated, ActorKind: careplan.ParentMessageActorGuardian,
		ActorAccountID: guardianAccountID, Body: createdBody, RequestType: requestType,
		RequestStatus: careplan.ParentMessageRequestStatusOpen,
	})
}

// EditRequest rewrites the submitter's own pending request. A request that is
// not the caller's own is reported as not found, never as forbidden: a
// stranger must not learn that the id exists.
func (w *ExcusedRequests) EditRequest(ctx context.Context, input careplan.ExcusedRequestEditInput) (result *careplan.ExcusedAbsenceRequest, err error) {
	started := time.Now()
	defer func() { w.done("excused_request_edit", started, err) }()
	// Lock regardless of status so ownership is checked BEFORE the
	// pending-status distinction, like the decide path.
	req, err := w.carePlan.FindExcusedAbsenceRequest(ctx, input.RequestID, true)
	if err != nil {
		return nil, err
	}
	if req.SubmittedBy != input.GuardianAccountID || req.StudentID != input.StudentID {
		return nil, careplan.ErrExcusedRequestNotFound
	}
	if req.Status != careplan.ExcusedRequestStatusPending {
		return nil, careplan.ErrExcusedRequestNotPending
	}
	if input.ExpectedVersion != "" && careplan.ParentRequestVersion(req.UpdatedAt) != input.ExpectedVersion {
		return nil, careplan.ErrParentRequestStale
	}
	absenceStatus := normalizedAbsenceRequestStatus(req.AbsenceStatus)
	// Same validators the create path runs: an edit may not produce a request
	// the create path would have refused.
	sorted, trimmed, err := validateAbsenceRequestInput(input.Dates, input.Note, absenceStatus, input.NoteRequired)
	if err != nil {
		return nil, err
	}
	if err := w.ensureNoPartialAbsence(ctx, req.StudentID, sorted); err != nil {
		return nil, err
	}
	if err := w.carePlan.UpdatePendingExcusedAbsenceRequest(ctx, req.ID, sorted, trimmed, absenceStatus); err != nil {
		return nil, err
	}
	row, err := w.carePlan.FindExcusedAbsenceRequest(ctx, req.ID, false)
	if err != nil {
		return nil, fmt.Errorf("care plan: reload edited absence request: %w", err)
	}
	if err := w.record(ctx, ports.RequestLedgerEntry{
		StudentID: row.StudentID, RequestType: careplan.ParentRequestTypeExcusedAbsence, RequestID: row.ID,
		EventType: careplan.ParentRequestEventGuardianEdit, ActorAccountID: input.GuardianAccountID, UpdatedAt: row.UpdatedAt,
		Payload: map[string]any{"days": len(sorted)},
	}); err != nil {
		return nil, fmt.Errorf("care plan: record absence edit event: %w", err)
	}
	// The pending badge does not change, but the dates staff see do: wake the
	// staff tabs the same way a create does.
	w.broadcastRequestTransition(ctx, row.TenantID, row.StudentID)
	return &row, nil
}

// --- reads

func (w *ExcusedRequests) ListForStudent(ctx context.Context, studentID int64, recentSince time.Time) (result []*careplan.ExcusedAbsenceRequest, err error) {
	started := time.Now()
	defer func() { w.done("excused_request_list_for_student", started, err) }()
	// Pending first (any age) then recently-decided, deduped by id. The parent
	// view shows pending requests indefinitely and rejected ones for a short
	// window so a parent learns their request was declined.
	pending, err := w.listPendingForStudent(ctx, studentID)
	if err != nil {
		return nil, err
	}
	recent, err := w.carePlan.ListExcusedAbsenceRequests(ctx, careplan.ExcusedAbsenceRequestFilter{StudentID: studentID, RecentSince: recentSince})
	if err != nil {
		return nil, err
	}
	seen := make(map[int64]struct{}, len(pending)+len(recent))
	out := make([]*careplan.ExcusedAbsenceRequest, 0, len(pending)+len(recent))
	for _, r := range pending {
		if _, ok := seen[r.ID]; ok {
			continue
		}
		seen[r.ID] = struct{}{}
		out = append(out, r)
	}
	for i := range recent {
		r := &recent[i]
		if _, ok := seen[r.ID]; ok {
			continue
		}
		// A withdrawn request was the parent's own cancellation: drop it. Keep
		// approved requests, though: the parent's status-day view fetches only
		// a bounded window, so an approval outside it would otherwise vanish.
		if r.Status == careplan.ExcusedRequestStatusWithdrawn {
			continue
		}
		seen[r.ID] = struct{}{}
		out = append(out, r)
	}
	return out, nil
}

func (w *ExcusedRequests) ListHistory(ctx context.Context, filter careplan.RequestQueueFilter) (items []*careplan.ExcusedRequestHistoryItem, next *careplan.RequestCursor, err error) {
	started := time.Now()
	defer func() { w.done("excused_request_list_history", started, err) }()
	// limit+1 probes for an older page without a second count query.
	probe := probeLimit(filter)
	rows, err := w.carePlan.ListExcusedAbsenceRequests(ctx, careplan.ExcusedAbsenceRequestFilter{
		Statuses: []string{careplan.ExcusedRequestStatusApproved, careplan.ExcusedRequestStatusRejected, careplan.ExcusedRequestStatusWithdrawn},
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

	studentIDs := make([]int64, 0, len(rows))
	seen := make(map[int64]struct{}, len(rows))
	reviewerIDs := make([]int64, 0, len(rows))
	seenReviewers := make(map[int64]struct{}, len(rows))
	for _, r := range rows {
		if _, ok := seen[r.StudentID]; !ok {
			seen[r.StudentID] = struct{}{}
			studentIDs = append(studentIDs, r.StudentID)
		}
		if r.ReviewedBy != nil && *r.ReviewedBy > 0 {
			if _, ok := seenReviewers[*r.ReviewedBy]; !ok {
				seenReviewers[*r.ReviewedBy] = struct{}{}
				reviewerIDs = append(reviewerIDs, *r.ReviewedBy)
			}
		}
	}
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

	studentIDs := make([]int64, 0, len(rows))
	seen := make(map[int64]struct{}, len(rows))
	for _, r := range rows {
		if _, ok := seen[r.StudentID]; !ok {
			seen[r.StudentID] = struct{}{}
			studentIDs = append(studentIDs, r.StudentID)
		}
	}
	students, persons, err := w.loadStudentNames(ctx, studentIDs, "excused requests")
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
	today := w.today()
	items = make([]*careplan.ExcusedRequestReviewItem, 0, len(rows))
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
		view, viewErr := w.currentStatusView(ctx, r)
		if viewErr != nil {
			return nil, nil, fmt.Errorf("care plan: resolve current absence statuses: %w", viewErr)
		}
		eligible, reasonCode, reasonText, eligibilityErr := w.excusedBulkEligibility(ctx, r, st, view)
		if eligibilityErr != nil {
			return nil, nil, fmt.Errorf("care plan: resolve absence bulk eligibility: %w", eligibilityErr)
		}
		currentValueChanged := view.changedSinceRequest
		item := &careplan.ExcusedRequestReviewItem{
			Request: r, BulkEligible: eligible,
			BulkIneligibleReason: reasonCode, BulkIneligibleText: reasonText,
			CurrentStatusByDate: view.byDate, CurrentValueChanged: &currentValueChanged,
		}
		if p, ok := persons[st.PersonID]; ok {
			item.FirstName, item.LastName = p.FirstName, p.LastName
		}
		items = append(items, item)
	}
	return items, next, nil
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

// --- decisions

func (w *ExcusedRequests) Decide(ctx context.Context, input careplan.ExcusedRequestDecideInput) (result *careplan.ExcusedRequestReviewItem, err error) {
	started := time.Now()
	defer func() { w.done("excused_request_decide", started, err) }()
	if input.RequestID <= 0 {
		return nil, careplan.ErrExcusedRequestNotFound
	}
	reason := strings.TrimSpace(input.Reason)
	if !input.Approve && reason == "" {
		return nil, careplan.ErrExcusedRequestRejectReasonRequired
	}
	// An approval needs a reason only while the school's policy asks staff
	// for one; a rejection always does.
	if input.Approve && input.ReasonRequired && reason == "" {
		return nil, careplan.ErrParentRequestReasonRequired
	}
	// The length bound applies to both verdicts: an approval reason is stored
	// and shown in the history exactly like a rejection reason.
	if utf8.RuneCountInString(reason) > excusedRequestMaxReasonLen {
		return nil, careplan.ErrExcusedRequestRejectReasonTooLong
	}

	// Lock the row and re-verify it is still pending so two staff deciding
	// the same request (or a decide racing the guardian's withdrawal)
	// serialize.
	req, err := w.carePlan.FindPendingExcusedAbsenceRequest(ctx, input.RequestID)
	if err != nil {
		return nil, err
	}
	if input.ExpectedVersion != "" && careplan.ParentRequestVersion(req.UpdatedAt) != input.ExpectedVersion {
		return nil, careplan.ErrParentRequestStale
	}
	req.AbsenceStatus = normalizedAbsenceRequestStatus(req.AbsenceStatus)

	// Per-child write authorization: decided by the parent-request review
	// scope. Taken FOR UPDATE so the alumnus gate below decides on a state a
	// concurrent grade transition cannot change underneath it. This is the
	// only student row this transaction locks, so the ascending-id order
	// every student-row locker follows is preserved.
	student, err := w.students.LockStudent(ctx, req.StudentID)
	if err != nil {
		return nil, fmt.Errorf("care plan: load student for absence request decision: %w", err)
	}
	today := w.today()
	// The child graduated or left care after filing this request. Approving
	// would write absence status days for a child no longer in care: same 404
	// the rest of the child surface returns.
	if student.Alumnus || student.CareEndedOn(today) {
		return nil, careplan.ErrExcusedRequestNotFound
	}
	if err := w.requireReviewer(ctx, &student); err != nil {
		return nil, err
	}

	if input.Approve {
		// Approving a request whose days have all passed would write absence
		// records into a settled past. Staff either reject it or mark it done.
		if parentRequestIsPast(excusedScopeEnd(&req), today) {
			return nil, careplan.ErrParentRequestPast
		}
		if requestExtendsBeyondCare(&req, &student) {
			return nil, careplan.ErrExcusedRequestNotFound
		}
		if err := w.ensureNoPartialAbsence(ctx, req.StudentID, req.Dates); err != nil {
			return nil, err
		}
		// Refuse to apply when the submitting guardian has lost access to the
		// child since the request was filed. Approving writes parent-sourced
		// status days and posts a parent-visible pill for a recipient the
		// parent APIs now hide; staff wind such a request down by rejecting.
		if w.messenger != nil {
			hasAccess, err := w.messenger.GuardianHasChildAccess(ctx, req.StudentID, req.SubmittedBy)
			if err != nil {
				return nil, fmt.Errorf("care plan: absence request guardian link check: %w", err)
			}
			if !hasAccess {
				return nil, careplan.ErrExcusedRequestGuardianAccessRevoked
			}
		}
		// Refuse to apply when the child's absence for a requested date was
		// created or changed after this request was filed: a blind approve
		// would silently overwrite that newer decision.
		if err := w.ensureNoNewerStatus(ctx, &req); err != nil {
			return nil, err
		}
		// Apply, status update and after-commit hooks all run in the ambient
		// tenant transaction. A mid-apply failure propagates so the whole
		// transaction rolls back rather than committing a half-applied absence.
		if err := w.applyAbsenceRequest(ctx, &req); err != nil {
			return nil, err
		}
	}

	newStatus := careplan.ExcusedRequestStatusApproved
	pillStatus := careplan.ParentMessageRequestStatusDone
	_, pillBody, requestType := absenceRequestCopy(req.AbsenceStatus)
	var reasonPtr *string
	// A staff reason is kept for both verdicts: without it the history shows
	// an approval with no explanation.
	if reason != "" {
		reasonPtr = &reason
	}
	if !input.Approve {
		newStatus = careplan.ExcusedRequestStatusRejected
		pillStatus = careplan.ParentMessageRequestStatusReject
		pillBody = absenceRequestRejectedBody(req.AbsenceStatus) + ": " + reason
	}
	reviewedBy := input.ReviewedBy
	if err := w.carePlan.DecideExcusedAbsenceRequest(ctx, careplan.ExcusedAbsenceDecision{
		ID: req.ID, Status: newStatus, Reason: reasonPtr, ReviewedBy: &reviewedBy, Applied: input.Approve,
	}); err != nil {
		return nil, err
	}

	tenantID := w.tenantOf(ctx, req.TenantID)
	if input.Approve {
		if w.notifier != nil {
			if err := w.notifier.NotifyAbsenceReported(ctx, ports.AbsenceReport{
				TenantID: tenantID, StudentIDs: []int64{req.StudentID}, Status: req.AbsenceStatus, Dates: req.Dates,
				FromParent: true, ActorAccountID: input.ReviewedBy, ExcludedAccountIDs: []int64{req.SubmittedBy},
			}); err != nil {
				return nil, fmt.Errorf("enqueue absence notification: %w", err)
			}
		}
		w.hooks.AfterCommit(ctx, func() {
			w.logger.Info("absence request approved",
				slog.Int64("request_id", req.ID),
				slog.Int64("student_id", req.StudentID),
				slog.Int64("tenant_id", req.TenantID),
				slog.Int64("reviewed_by", input.ReviewedBy),
				slog.String("status", req.AbsenceStatus),
				slog.Int("days", len(req.Dates)),
			)
		})
	} else {
		w.hooks.AfterCommit(ctx, func() {
			w.logger.Info("absence request rejected",
				slog.Int64("request_id", req.ID),
				slog.Int64("student_id", req.StudentID),
				slog.Int64("tenant_id", req.TenantID),
				slog.Int64("reviewed_by", input.ReviewedBy),
				slog.String("status", req.AbsenceStatus),
			)
		})
	}
	// Either decision clears the child's pending badge (approval also
	// surfaces the confirmed absence). Wake staff tabs for both outcomes.
	w.broadcastRequestTransition(ctx, req.TenantID, req.StudentID)
	decisionEvent := ports.RequestEvent{
		EventType: careplan.ParentMessageEventRequestStatus, ActorKind: careplan.ParentMessageActorStaff,
		ActorAccountID: input.ReviewedBy, Body: pillBody, RequestType: requestType,
		RequestStatus: pillStatus, DecisionReason: reason,
	}
	if err := w.emitRequestPillAfterCommit(ctx, &req, decisionEvent); err != nil {
		return nil, err
	}
	// Every other guardian of this child gets a neutral line: the care
	// changed, on these days. Explicit share recipients get the submitter's
	// pill verbatim.
	w.notifyOtherGuardiansAfterCommit(ctx, &req, decisionEvent)

	row, err := w.carePlan.FindExcusedAbsenceRequest(ctx, req.ID, false)
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
	if names, nameErr := w.students.PersonNames(ctx, []int64{student.PersonID}); nameErr == nil {
		if p, ok := names[student.PersonID]; ok {
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
	return w.emitRequestPillAfterCommit(ctx, &req, ports.RequestEvent{
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
	req, err := w.carePlan.FindExcusedAbsenceRequest(ctx, requestID, true)
	if err != nil {
		return err
	}
	if !isCorrectableExcusedStatus(req.Status) {
		return careplan.ErrParentRequestNotDecided
	}
	if expectedVersion != "" && careplan.ParentRequestVersion(req.UpdatedAt) != expectedVersion {
		return careplan.ErrParentRequestStale
	}
	req.AbsenceStatus = normalizedAbsenceRequestStatus(req.AbsenceStatus)
	// Re-authorize against the current scope: a group leader who has since
	// lost the group must not be able to correct its decisions.
	student, err := w.students.LockStudent(ctx, req.StudentID)
	if err != nil {
		return fmt.Errorf("care plan: load student for absence request correction: %w", err)
	}
	if err := w.requireReviewer(ctx, &student); err != nil {
		return err
	}
	// Correcting into an approval would write absence days into a settled
	// past, exactly as a fresh approval would.
	if approve && parentRequestIsPast(excusedScopeEnd(&req), w.today()) {
		return careplan.ErrParentRequestPast
	}
	if approve {
		if err := w.applyAbsenceRequest(ctx, &req); err != nil {
			return err
		}
	} else if err := w.revertApprovedAbsence(ctx, &req); err != nil {
		return err
	}

	var reasonPtr *string
	if trimmed != "" {
		reasonPtr = &trimmed
	}
	newStatus := careplan.ExcusedRequestStatusRejected
	if approve {
		newStatus = careplan.ExcusedRequestStatusApproved
	}
	reviewer := reviewedBy
	if err := w.carePlan.RedecideExcusedAbsenceRequest(ctx, careplan.ExcusedAbsenceDecision{
		ID: req.ID, Status: newStatus, Reason: reasonPtr, ReviewedBy: &reviewer, Applied: approve,
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
	return w.emitRequestPillAfterCommit(ctx, &req, ports.RequestEvent{
		EventType: careplan.ParentMessageEventRequestStatus, ActorKind: careplan.ParentMessageActorStaff,
		ActorAccountID: reviewedBy, Body: pillBody, RequestType: requestType,
		RequestStatus: pillStatus, DecisionReason: trimmed,
	})
}

// --- cross-kind coordinator ports

func (w *ExcusedRequests) GetExcusedBulkCandidate(ctx context.Context, requestID int64) (*careplan.ExcusedBulkCandidate, error) {
	req, err := w.carePlan.FindExcusedAbsenceRequest(ctx, requestID, false)
	if errors.Is(err, careplan.ErrExcusedRequestNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if req.Status != careplan.ExcusedRequestStatusPending {
		return nil, nil
	}
	students, err := w.students.FindStudents(ctx, []int64{req.StudentID})
	if err != nil {
		return nil, err
	}
	scope, err := w.reviewScope(ctx)
	if err != nil {
		return nil, err
	}
	student, ok := students[req.StudentID]
	if !ok || !scope.Allows(&student) || student.Alumnus || student.CareEndedOn(w.today()) {
		return nil, nil
	}
	view, err := w.currentStatusView(ctx, &req)
	if err != nil {
		return nil, err
	}
	eligible, _, _, err := w.excusedBulkEligibility(ctx, &req, student, view)
	if err != nil {
		return nil, err
	}
	return &careplan.ExcusedBulkCandidate{ID: req.ID, StudentID: req.StudentID, UpdatedAt: req.UpdatedAt, Eligible: eligible}, nil
}

func (w *ExcusedRequests) LockExcusedBulkRequest(ctx context.Context, requestID int64) error {
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

// --- guards

func (w *ExcusedRequests) reviewScope(ctx context.Context) (ports.ReviewScope, error) {
	scope, err := w.scope(ctx)
	if err != nil {
		return ports.ReviewScope{}, fmt.Errorf("care plan: resolve request reviewer scope: %w", err)
	}
	return scope, nil
}

func (w *ExcusedRequests) requireReviewer(ctx context.Context, student *ports.ReviewStudent) error {
	scope, err := w.reviewScope(ctx)
	if err != nil {
		return err
	}
	if !scope.Allows(student) {
		return careplan.ErrExcusedRequestForbidden
	}
	return nil
}

// excusedBulkEligibility answers whether this request can ride a bulk
// approval. It takes the already-resolved status view so the queue reads the
// child's current days exactly once per request, whichever branch decides.
func (w *ExcusedRequests) excusedBulkEligibility(ctx context.Context, req *careplan.ExcusedAbsenceRequest, student ports.ReviewStudent, view currentStatusView) (bool, string, string, error) {
	if !absenceBulkEligible(req.Dates, w.today()) {
		return false, careplan.BulkIneligiblePast, "Mindestens ein Tag ist vorbei.", nil
	}
	if requestExtendsBeyondCare(req, &student) {
		return false, careplan.BulkIneligibleChildUnavailable, "Mindestens ein Tag liegt nach dem Betreuungsende.", nil
	}
	partial, err := w.hasManualPartialAbsence(ctx, req.StudentID, req.Dates)
	if err != nil {
		return false, "", "", err
	}
	if partial {
		return false, careplan.BulkIneligibleConflict, "Für mindestens einen Tag ist bereits eine Teilabwesenheit eingetragen.", nil
	}
	if w.messenger != nil {
		hasAccess, accessErr := w.messenger.GuardianHasChildAccess(ctx, req.StudentID, req.SubmittedBy)
		if accessErr != nil {
			return false, "", "", accessErr
		}
		if !hasAccess {
			return false, careplan.BulkIneligibleAccessRevoked, "Der Zugang der einreichenden Person ist nicht mehr aktiv.", nil
		}
	}
	if view.changedSinceRequest {
		return false, careplan.BulkIneligibleStale, "Für mindestens einen Tag gibt es einen neueren Abwesenheitsstatus.", nil
	}
	return true, "", "", nil
}

func (w *ExcusedRequests) hasManualPartialAbsence(ctx context.Context, studentID int64, dates []careplan.Date) (bool, error) {
	if len(dates) == 0 {
		return false, nil
	}
	rows, err := w.carePlan.ListPickupExceptions(ctx, careplan.StudentScheduleFilter{
		StudentIDs: []int64{studentID}, From: dates[0], To: dates[len(dates)-1],
	})
	if err != nil {
		return false, err
	}
	requested := make(map[careplan.Date]struct{}, len(dates))
	for _, date := range dates {
		requested[date] = struct{}{}
	}
	for _, row := range rows {
		// Only manual partial absences conflict; auto-derived excusals
		// (pulled-forward pickup time) coexist with a full-day status.
		if _, ok := requested[row.ExceptionDate]; ok && row.ExcusedFrom != nil && !row.ExcusedAuto {
			return true, nil
		}
	}
	return false, nil
}

// ensureNoPartialAbsence locks every requested care day in the canonical
// order (student row first, then care-day) and refuses when a manual
// partial-day excusal already owns one of the dates.
func (w *ExcusedRequests) ensureNoPartialAbsence(ctx context.Context, studentID int64, dates []careplan.Date) error {
	if len(dates) == 0 {
		return nil
	}
	// Sort before locking so multi-day writers always take care-day keys in
	// the same order (avoids AB-BA deadlocks across concurrent requests).
	sortedDates := dedupeSortedDates(dates)
	for _, date := range sortedDates {
		if err := w.carePlan.LockStudentAndExceptionDay(ctx, studentID, date.String()); err != nil {
			return err
		}
	}
	partial, err := w.hasManualPartialAbsence(ctx, studentID, sortedDates)
	if err != nil {
		return err
	}
	if partial {
		return careplan.ErrExcusedRequestStatusConflict
	}
	return nil
}

// ensureNoNewerStatus refuses the approval when any active status day on a
// requested date was reported after the request was created: a newer
// absence decision that approving would clobber.
func (w *ExcusedRequests) ensureNoNewerStatus(ctx context.Context, req *careplan.ExcusedAbsenceRequest) error {
	view, err := w.currentStatusView(ctx, req)
	if err != nil {
		return err
	}
	if view.changedSinceRequest {
		return careplan.ErrExcusedRequestStatusConflict
	}
	return nil
}

// currentStatusView is what the child's days look like right now, for exactly
// the days this request asks about. The approve gate and the review queue
// share it, so they can never disagree.
type currentStatusView struct {
	byDate              map[string]string
	changedSinceRequest bool
}

func (w *ExcusedRequests) currentStatusView(ctx context.Context, req *careplan.ExcusedAbsenceRequest) (currentStatusView, error) {
	view := currentStatusView{byDate: make(map[string]string, len(req.Dates))}
	if len(req.Dates) == 0 {
		return view, nil
	}
	// Every requested day starts as "present"; the rows below overwrite the
	// days that carry a status, so a day with no record is still answered for.
	for _, date := range req.Dates {
		view.byDate[date.String()] = careplan.StudentStatusDayPresent
	}
	rows, err := w.activeStatusDays(ctx, req)
	if err != nil {
		return currentStatusView{}, err
	}
	for _, row := range rows {
		key := row.Date.String()
		if _, requested := view.byDate[key]; !requested {
			continue
		}
		view.byDate[key] = row.Status
		if row.ReportedAt.After(req.CreatedAt) {
			view.changedSinceRequest = true
		}
	}
	return view, nil
}

// activeStatusDays reads the child's active status rows across the request's
// date span. Dates are stored sorted ascending, so first/last bound the range.
func (w *ExcusedRequests) activeStatusDays(ctx context.Context, req *careplan.ExcusedAbsenceRequest) ([]careplan.StudentStatusDay, error) {
	return w.carePlan.ListStudentStatusDays(ctx, careplan.StudentStatusDayFilter{
		StudentIDs: []int64{req.StudentID}, From: req.Dates[0], To: req.Dates[len(req.Dates)-1], ActiveOnly: true,
	})
}

// applyAbsenceRequest writes the approved request using its requested status,
// mirroring the direct parent path. It also updates today's live sick flag.
func (w *ExcusedRequests) applyAbsenceRequest(ctx context.Context, req *careplan.ExcusedAbsenceRequest) error {
	if len(req.Dates) == 0 {
		return careplan.ErrExcusedRequestNoDates
	}
	now := time.Now()
	note := req.Note
	for _, other := range careplan.StudentStatusDayStatusesExcept(req.AbsenceStatus) {
		if err := w.carePlan.ClearStudentStatusDays(ctx, req.StudentID, other, req.Dates, now, careplan.StudentStatusSourceParent); err != nil {
			return err
		}
	}
	for _, d := range req.Dates {
		guardianAccountID := req.SubmittedBy
		if _, err := w.carePlan.UpsertStudentStatusDay(ctx, careplan.StudentStatusDay{
			StudentID: req.StudentID, Date: d, Status: req.AbsenceStatus, ReportedAt: now,
			Source: careplan.StudentStatusSourceParent, GuardianAccountID: &guardianAccountID, Note: &note,
		}); err != nil {
			return err
		}
	}
	if containsDate(req.Dates, w.today()) {
		if err := w.students.SetLiveAbsenceFlags(ctx, req.StudentID, req.AbsenceStatus, now); err != nil {
			return err
		}
	}
	return nil
}

// revertApprovedAbsence clears exactly the days this request wrote, and only
// while they are provably still its own. A day that was re-reported after
// the approval belongs to whoever wrote it last.
func (w *ExcusedRequests) revertApprovedAbsence(ctx context.Context, req *careplan.ExcusedAbsenceRequest) error {
	if req.Status != careplan.ExcusedRequestStatusApproved || len(req.Dates) == 0 {
		return nil
	}
	rows, err := w.activeStatusDays(ctx, req)
	if err != nil {
		return err
	}
	requested := make(map[careplan.Date]struct{}, len(req.Dates))
	for _, date := range req.Dates {
		requested[date] = struct{}{}
	}
	for i := range rows {
		row := &rows[i]
		if _, ok := requested[row.Date]; !ok || row.Status != req.AbsenceStatus {
			continue
		}
		if !absenceRowWrittenByRequest(row, req) {
			return fmt.Errorf("%w: der Eintrag für den %s wurde nach der Entscheidung geändert",
				careplan.ErrParentRequestCorrectionUnsupported, germanDate(row.Date))
		}
	}
	return w.carePlan.ClearStudentStatusDays(ctx, req.StudentID, req.AbsenceStatus, req.Dates, time.Now(), careplan.StudentStatusSourceParent)
}

// absenceRowWrittenByRequest reports whether this status day is still the one
// the request's approval produced: parent-sourced, from the same guardian, and
// not re-reported since the decision.
func absenceRowWrittenByRequest(row *careplan.StudentStatusDay, req *careplan.ExcusedAbsenceRequest) bool {
	if row.Source != careplan.StudentStatusSourceParent {
		return false
	}
	if row.GuardianAccountID == nil || *row.GuardianAccountID != req.SubmittedBy {
		return false
	}
	return req.ReviewedAt == nil || !row.ReportedAt.After(*req.ReviewedAt)
}

// --- effects

func (w *ExcusedRequests) record(ctx context.Context, entry ports.RequestLedgerEntry) error {
	if w.ledger == nil {
		return nil
	}
	return w.ledger.Record(ctx, entry)
}

func (w *ExcusedRequests) tenantOf(ctx context.Context, rowTenantID int64) int64 {
	if rowTenantID > 0 {
		return rowTenantID
	}
	return w.hooks.TenantID(ctx)
}

// emitRequestPillAfterCommit writes the durable decision intent in the ambient
// transaction, then schedules the best-effort chat pill after commit.
func (w *ExcusedRequests) emitRequestPillAfterCommit(ctx context.Context, req *careplan.ExcusedAbsenceRequest, ev ports.RequestEvent) error {
	if w.messenger == nil {
		return nil
	}
	tenantID := w.tenantOf(ctx, req.TenantID)
	ev.RefTable = excusedRequestRefTable
	ev.RefID = req.ID
	studentID, guardianAccountID := req.StudentID, req.SubmittedBy
	if err := w.messenger.EnqueueRequestDecision(ctx, tenantID, studentID, guardianAccountID, ev); err != nil {
		return fmt.Errorf("care plan: enqueue absence request decision: %w", err)
	}
	w.hooks.AfterCommit(ctx, func() {
		w.messenger.EmitChildEvent(tenantID, studentID, guardianAccountID, ev)
	})
	return nil
}

// broadcastRequestTransition wakes both audiences after any absence-request
// transition that changes the child's "Freigabe ausstehend" badge: staff
// tabs through the tenant-wide events, guardians through a message-
// independent invalidation. Both run after commit so a woken client never
// reads the pre-commit snapshot.
func (w *ExcusedRequests) broadcastRequestTransition(ctx context.Context, tenantID, studentID int64) {
	tenantID = w.tenantOf(ctx, tenantID)
	w.hooks.AfterCommit(ctx, func() {
		if w.broadcaster != nil {
			if err := w.broadcaster.StudentUpdated(tenantID); err != nil {
				w.logger.Warn("care plan: broadcast student_updated after excused request transition failed",
					slog.Int64("tenant_id", tenantID),
					slog.Int64("student_id", studentID),
					slog.String("error", err.Error()),
				)
			}
			if err := w.broadcaster.ChangeRequestsChanged(tenantID); err != nil {
				w.logger.Warn("care plan: broadcast change_requests_changed after excused request transition failed",
					slog.Int64("tenant_id", tenantID),
					slog.Int64("student_id", studentID),
					slog.String("error", err.Error()),
				)
			}
		}
		if w.messenger != nil {
			w.messenger.BroadcastChildUpdateToGuardians(tenantID, studentID)
		}
	})
}

// notifyOtherGuardiansAfterCommit posts the neutral „Betreuungsstand
// geändert“ line to every other portal guardian of the child. The guardians
// are resolved inside the transaction; only the pills are posted after commit.
func (w *ExcusedRequests) notifyOtherGuardiansAfterCommit(ctx context.Context, req *careplan.ExcusedAbsenceRequest, ev ports.RequestEvent) {
	if w.messenger == nil {
		return
	}
	audience, err := w.messenger.ResolveDecisionAudience(ctx, req.StudentID, req.SubmittedBy, w.sharedRecipients(ctx, req))
	if err != nil {
		w.logger.Warn("co-guardian notice: resolving guardians failed",
			slog.Int64("request_id", req.ID),
			slog.Int64("student_id", req.StudentID),
			slog.String("error", err.Error()),
		)
		return
	}
	if len(audience.Full) == 0 && len(audience.Neutral) == 0 {
		return
	}
	tenantID := w.tenantOf(ctx, req.TenantID)
	neutral := ev
	neutral.Body = coGuardianNoticeBody(req)
	studentID := req.StudentID
	w.hooks.AfterCommit(ctx, func() {
		w.messenger.EmitDecisionAudience(tenantID, studentID, audience, ev, neutral)
	})
}

// sharedRecipients resolves the explicit recipients of one request,
// tolerating an unwired resolver. An error is not fatal: the fallback is that
// everyone gets the neutral line, which is the safe direction.
func (w *ExcusedRequests) sharedRecipients(ctx context.Context, req *careplan.ExcusedAbsenceRequest) []int64 {
	if w.shares == nil {
		return nil
	}
	accountIDs, err := w.shares.SharedRecipientAccountIDs(ctx, req.StudentID, careplan.ParentRequestTypeExcusedAbsence, req.ID)
	if err != nil {
		w.logger.Warn("resolving explicit request recipients failed, falling back to neutral notices",
			slog.Int64("request_id", req.ID),
			slog.Int64("student_id", req.StudentID),
			slog.String("error", err.Error()),
		)
		return nil
	}
	return accountIDs
}

// --- store helpers

func (w *ExcusedRequests) listPendingForStudent(ctx context.Context, studentID int64) ([]*careplan.ExcusedAbsenceRequest, error) {
	rows, err := w.carePlan.ListExcusedAbsenceRequests(ctx, careplan.ExcusedAbsenceRequestFilter{
		StudentID: studentID, Statuses: []string{careplan.ExcusedRequestStatusPending},
	})
	if err != nil {
		return nil, err
	}
	result := make([]*careplan.ExcusedAbsenceRequest, 0, len(rows))
	for i := range rows {
		result = append(result, &rows[i])
	}
	return result, nil
}

func (w *ExcusedRequests) listPendingForTenant(ctx context.Context, queue *careplan.RequestQueueFilter) ([]careplan.ExcusedAbsenceRequest, error) {
	return w.carePlan.ListExcusedAbsenceRequests(ctx, careplan.ExcusedAbsenceRequestFilter{
		Statuses: []string{careplan.ExcusedRequestStatusPending}, Queue: queue,
	})
}

func (w *ExcusedRequests) loadStudentNames(ctx context.Context, studentIDs []int64, what string) (map[int64]ports.ReviewStudent, map[int64]ports.PersonName, error) {
	students, err := w.students.FindStudents(ctx, studentIDs)
	if err != nil {
		return nil, nil, fmt.Errorf("care plan: load students for %s: %w", what, err)
	}
	personIDs := make([]int64, 0, len(students))
	for _, st := range students {
		personIDs = append(personIDs, st.PersonID)
	}
	persons, err := w.students.PersonNames(ctx, personIDs)
	if err != nil {
		return nil, nil, fmt.Errorf("care plan: load persons for %s: %w", what, err)
	}
	return students, persons, nil
}

// --- pure helpers

// probeLimit asks the store for one row more than the caller wants, so a
// present extra row proves an older page exists. An unbounded page stays
// unbounded.
func probeLimit(filter careplan.RequestQueueFilter) careplan.RequestQueueFilter {
	if filter.Limit > 0 {
		filter.Limit++
	}
	return filter
}

// nextCursor trims the probe row off a page and returns the position to
// resume from, or nil on the last page.
func nextCursor(rows []careplan.ExcusedAbsenceRequest, limit int, key func(careplan.ExcusedAbsenceRequest) (time.Time, int64)) ([]careplan.ExcusedAbsenceRequest, *careplan.RequestCursor) {
	if limit <= 0 || len(rows) <= limit {
		return rows, nil
	}
	rows = rows[:limit]
	instant, id := key(rows[len(rows)-1])
	return rows, &careplan.RequestCursor{UpdatedAt: instant, ID: id}
}

// reviewerDisplayName resolves a nullable reviewer account id to a display
// name. A decided row whose reviewer account was deleted still shows up, as
// "Unbekannt", instead of losing the fact that somebody decided it.
func reviewerDisplayName(reviewers map[int64]ports.PersonName, reviewedBy *int64) string {
	if reviewedBy == nil || *reviewedBy <= 0 {
		return ""
	}
	if p, ok := reviewers[*reviewedBy]; ok {
		return strings.TrimSpace(p.FirstName + " " + p.LastName)
	}
	return "Unbekannt"
}

func requestExtendsBeyondCare(req *careplan.ExcusedAbsenceRequest, student *ports.ReviewStudent) bool {
	if req == nil || student == nil || student.EnrolledUntil.IsZero() {
		return false
	}
	for _, date := range req.Dates {
		if date.After(student.EnrolledUntil) {
			return true
		}
	}
	return false
}

// parentRequestIsPast reports whether a request whose scope ends on scopeEnd
// no longer covers any day from today on. A zero scopeEnd is never past.
func parentRequestIsPast(scopeEnd, today careplan.Date) bool {
	return !scopeEnd.IsZero() && scopeEnd.Before(today)
}

// absenceBulkEligible reports whether every requested day is today or later.
func absenceBulkEligible(dates []careplan.Date, today careplan.Date) bool {
	if len(dates) == 0 {
		return false
	}
	for _, date := range dates {
		if date.Before(today) {
			return false
		}
	}
	return true
}

// excusedScopeEnd is the last day this request covers.
func excusedScopeEnd(req *careplan.ExcusedAbsenceRequest) careplan.Date {
	var last careplan.Date
	for _, date := range req.Dates {
		if last.IsZero() || date.After(last) {
			last = date
		}
	}
	return last
}

func isCorrectableExcusedStatus(status string) bool {
	return status == careplan.ExcusedRequestStatusApproved || status == careplan.ExcusedRequestStatusRejected
}

func absenceRequestCopy(absenceStatus string) (created, confirmed, requestType string) {
	if absenceStatus == careplan.StudentStatusDaySick {
		return sickRequestCreatedBody, sickRequestConfirmedBody, careplan.ParentMessageRequestSick
	}
	return excusedRequestCreatedBody, excusedRequestConfirmedBody, careplan.ParentMessageRequestExcused
}

func absenceRequestRejectedBody(absenceStatus string) string {
	if absenceStatus == careplan.StudentStatusDaySick {
		return sickRequestRejectedBody
	}
	return excusedRequestRejectedBody
}

// correctedAbsencePill names the new outcome for the family. The wording says
// the decision changed, because "bestätigt" alone after a rejection reads
// like a duplicate message rather than a correction.
func correctedAbsencePill(absenceStatus string, approve bool) (string, string) {
	if approve {
		_, confirmed, _ := absenceRequestCopy(absenceStatus)
		return "Entscheidung geändert: " + confirmed, careplan.ParentMessageRequestStatusDone
	}
	return "Entscheidung geändert: " + absenceRequestRejectedBody(absenceStatus), careplan.ParentMessageRequestStatusReject
}

// coGuardianNoticeBody names what changed and when, and nothing else.
func coGuardianNoticeBody(req *careplan.ExcusedAbsenceRequest) string {
	kind := "Abmeldung"
	if req.AbsenceStatus == careplan.StudentStatusDaySick {
		kind = "Krankmeldung"
	}
	return "Betreuungsstand geändert: " + kind + " " + absenceDateRangeLabel(req.Dates)
}

// absenceDateRangeLabel renders one day as a date and several as a range.
func absenceDateRangeLabel(dates []careplan.Date) string {
	if len(dates) == 0 {
		return ""
	}
	first := germanDate(dates[0])
	if len(dates) == 1 {
		return first
	}
	return first + " bis " + germanDate(dates[len(dates)-1])
}

// germanDate renders a canonical date as DD.MM.YYYY; an unparsable value is
// shown as stored rather than hidden.
func germanDate(date careplan.Date) string {
	parsed, err := time.Parse(careplan.DateLayout, date.String())
	if err != nil {
		return date.String()
	}
	return parsed.Format("02.01.2006")
}

// correctionEventPayload is what a corrected ledger entry carries: the new
// verdict and the decision that stood before it, including who took it and
// why.
func correctionEventPayload(approve bool, reason, fromStatus, toStatus string, priorReviewer *int64, priorReason *string) map[string]any {
	payload := map[string]any{"approve": approve, "reason": reason, "from": fromStatus, "to": toStatus}
	if priorReviewer != nil {
		payload["prior_reviewer"] = *priorReviewer
	}
	if priorReason != nil {
		payload["prior_reason"] = *priorReason
	}
	return payload
}

// Before absence_status existed, every row represented an excused absence.
// The migration backfills persisted rows; this fallback keeps zero-value
// test doubles compatible too.
func normalizedAbsenceRequestStatus(absenceStatus string) string {
	if absenceStatus == "" {
		return careplan.StudentStatusDayExcused
	}
	return absenceStatus
}

// sameDateSet reports whether two date slices contain exactly the same days.
// Both are stored deduped and sorted, so a positional comparison suffices.
func sameDateSet(a, b []careplan.Date) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// datesIntersect reports whether the two date slices share at least one day.
func datesIntersect(a, b []careplan.Date) bool {
	set := make(map[careplan.Date]struct{}, len(a))
	for _, d := range a {
		set[d] = struct{}{}
	}
	for _, d := range b {
		if _, ok := set[d]; ok {
			return true
		}
	}
	return false
}

// dedupeSortedDates removes duplicate dates and returns them ascending, so
// the stored payload is canonical regardless of client ordering.
func dedupeSortedDates(dates []careplan.Date) []careplan.Date {
	seen := make(map[careplan.Date]struct{}, len(dates))
	out := make([]careplan.Date, 0, len(dates))
	for _, d := range dates {
		if _, ok := seen[d]; ok {
			continue
		}
		seen[d] = struct{}{}
		out = append(out, d)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Before(out[j]) })
	return out
}

func containsDate(dates []careplan.Date, needle careplan.Date) bool {
	for _, d := range dates {
		if d == needle {
			return true
		}
	}
	return false
}
