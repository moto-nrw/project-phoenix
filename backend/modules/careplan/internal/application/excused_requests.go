package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
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
	Scope       careplan.ReviewScopeResolver
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
	scope       careplan.ReviewScopeResolver
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
	return w.emitRequestPill(ctx, req, ports.RequestEvent{
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
