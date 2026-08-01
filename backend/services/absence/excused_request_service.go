// Excused-absence approval requests (#1845): when a school turns the approval
// gate (operations.parent_excused_requires_approval) on, a guardian's
// "entschuldigte Abmeldung" becomes a PENDING request here instead of being
// written straight to the child's status days. Staff review and decide it on
// the central "Änderungsanfragen" page. On approval the request is applied as
// excused active.student_status_days rows exactly like the direct parent path;
// a rejected/withdrawn request never touches the status days, so the child
// stays "expected" until a decision is made. The chat receives non-interactive
// notification pills (created / decided / withdrawn) via parentmessaging.Emitter.
package absence

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/realtime"
	notificationsService "github.com/moto-nrw/project-phoenix/services/notifications"
	"github.com/moto-nrw/project-phoenix/services/parentmessaging"
	userContextService "github.com/moto-nrw/project-phoenix/services/usercontext"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// excusedRequestMaxReasonLen bounds the staff reject reason (in runes).
const excusedRequestMaxReasonLen = 2000

// German pill texts. The staff portal renders these directly; the parents
// portal localizes from the structured event fields instead.
const (
	excusedRequestCreatedBody   = "Anfrage: Entschuldigte Abmeldung"
	excusedRequestConfirmedBody = "Abmeldung bestätigt"
	excusedRequestWithdrawnBody = "Abmeldung zurückgezogen"
)

var (
	// ErrExcusedRequestForbidden means the caller may not decide requests for
	// this child (no staff identity, or no write access to the student).
	ErrExcusedRequestForbidden = errors.New("active: excused request forbidden")
	// ErrExcusedRequestGuardianAccessRevoked means the submitting guardian is no
	// longer a linked guardian of the child with parent_portal.access, so
	// approving (which writes parent-sourced excused status days and posts a
	// parent-visible "bestätigt" pill for a recipient who can no longer read it)
	// is refused. Staff wind such a request down by REJECTING it — the reject
	// path is deliberately not gated on the guardian link, mirroring the
	// care-schedule request flow.
	ErrExcusedRequestGuardianAccessRevoked = errors.New("active: excused request guardian access revoked")
	// ErrExcusedRequestOverlap means the guardian already has a PENDING request
	// whose dates intersect (but are not identical to) the new submission. An
	// identical resubmit is treated as an idempotent retry (the existing row is
	// returned, no error); a partial overlap is refused so two pending requests
	// can't cover the same day with contradictory outcomes. Disjoint date sets are
	// allowed. Maps to a 409 on the parent write path.
	ErrExcusedRequestOverlap = errors.New("active: excused request overlaps an existing pending request")
	// ErrExcusedRequestStatusConflict means the child's absence for one of the
	// requested dates was created or changed AFTER this request was filed — a
	// newer sick / class-trip / excused record staff (or another flow) set in the
	// meantime. Approving would silently overwrite that newer decision, so the
	// approval is refused; staff resolve it by rejecting the stale request (or
	// clearing the newer status first). Maps to a 409 on the staff decide path.
	ErrExcusedRequestStatusConflict = errors.New("active: excused request superseded by a newer status")
	// ErrExcusedRequestNoDates means the request carried no dates.
	ErrExcusedRequestNoDates = errors.New("active: excused request requires at least one date")
	// ErrExcusedRequestEmptyNote means the mandatory note was blank.
	ErrExcusedRequestEmptyNote = errors.New("active: excused request note must not be empty")
	// ErrExcusedRequestNoteTooLong means the note exceeded the bound.
	ErrExcusedRequestNoteTooLong = errors.New("active: excused request note too long")
	// ErrExcusedRequestRejectReasonRequired means staff rejected without a reason.
	ErrExcusedRequestRejectReasonRequired = errors.New("active: reject reason is required")
	// ErrExcusedRequestRejectReasonTooLong means the reason exceeded the bound.
	ErrExcusedRequestRejectReasonTooLong = errors.New("active: reject reason too long")
)

// ExcusedRequestReviewItem is one request enriched with the child's name for
// the staff review queue.
type ExcusedRequestReviewItem struct {
	Request   *activeModels.ExcusedAbsenceRequest
	FirstName string
	LastName  string
}

// ExcusedRequestDecideInput carries a staff decision on one pending request.
type ExcusedRequestDecideInput struct {
	RequestID int64
	Approve   bool
	Reason    string
	// ReviewedBy is the acting staff ACCOUNT id (auth.accounts).
	ReviewedBy int64
}

// ExcusedAbsenceRequestService owns the excused-absence request lifecycle. All
// methods run inside an ambient tenant transaction: the staff paths under the
// request's TenantTxMiddleware, the parent paths inside the tenant.WithTenantTx
// the parent service opens after resolving the child.
type ExcusedAbsenceRequestService interface {
	// CreateRequest stores a guardian's pending request, then (after commit)
	// posts the "Anfrage erstellt" pill. The caller has already authorized the
	// guardian for this child; note is required and length-bounded here.
	CreateRequest(ctx context.Context, studentID, guardianAccountID int64, dates []timezone.Date, note string) (*activeModels.ExcusedAbsenceRequest, error)
	// WithdrawRequest moves the submitter's own pending request to withdrawn and
	// (after commit) posts the withdrawal pill.
	WithdrawRequest(ctx context.Context, requestID, studentID, guardianAccountID int64) (*activeModels.ExcusedAbsenceRequest, error)
	// ListForStudent returns the child's pending requests plus any decided since
	// `recentSince`, newest-first — the parent read view.
	ListForStudent(ctx context.Context, studentID int64, recentSince time.Time) ([]*activeModels.ExcusedAbsenceRequest, error)
	// ListPending returns every pending request for the current tenant,
	// newest-first, enriched with child names — the staff review queue.
	ListPending(ctx context.Context) ([]*ExcusedRequestReviewItem, error)
	// PendingByStudentForDate returns, per student, the newest pending request
	// whose dates cover the given calendar day — for the inline planning badge.
	// A student with no pending request for the day is absent from the map.
	PendingByStudentForDate(ctx context.Context, date timezone.Date) (map[int64]*activeModels.ExcusedAbsenceRequest, error)
	// Decide approves (writes the excused status days, then stamps) or rejects
	// (reason required) one pending request and returns the refreshed row.
	Decide(ctx context.Context, input ExcusedRequestDecideInput) (*ExcusedRequestReviewItem, error)
}

// AbsenceNotifierSetter injects the notification producer after construction.
// The notification stack is built after this service in the factory.
type AbsenceNotifierSetter interface {
	SetAbsenceNotifier(notifier notificationsService.AbsenceNotifier)
}

type excusedAbsenceRequestService struct {
	requestRepo   activeModels.ExcusedAbsenceRequestRepository
	statusDayRepo activeModels.StudentStatusDayRepository
	studentRepo   usersModels.StudentRepository
	personRepo    usersModels.PersonRepository
	userContext   userContextService.UserContextService
	emitter       *parentmessaging.Emitter
	broadcaster   realtime.Broadcaster
	absenceNotify notificationsService.AbsenceNotifier
	logger        *slog.Logger
}

// NewExcusedAbsenceRequestService wires the excused-request service.
func NewExcusedAbsenceRequestService(
	requestRepo activeModels.ExcusedAbsenceRequestRepository,
	statusDayRepo activeModels.StudentStatusDayRepository,
	studentRepo usersModels.StudentRepository,
	personRepo usersModels.PersonRepository,
	userContext userContextService.UserContextService,
	emitter *parentmessaging.Emitter,
	broadcaster realtime.Broadcaster,
	logger *slog.Logger,
) ExcusedAbsenceRequestService {
	if logger == nil {
		logger = slog.Default()
	}
	return &excusedAbsenceRequestService{
		requestRepo:   requestRepo,
		statusDayRepo: statusDayRepo,
		studentRepo:   studentRepo,
		personRepo:    personRepo,
		userContext:   userContext,
		emitter:       emitter,
		broadcaster:   broadcaster,
		logger:        logger,
	}
}

// SetAbsenceNotifier implements AbsenceNotifierSetter.
func (s *excusedAbsenceRequestService) SetAbsenceNotifier(notifier notificationsService.AbsenceNotifier) {
	s.absenceNotify = notifier
}

func (s *excusedAbsenceRequestService) CreateRequest(ctx context.Context, studentID, guardianAccountID int64, dates []timezone.Date, note string) (*activeModels.ExcusedAbsenceRequest, error) {
	if len(dates) == 0 {
		return nil, ErrExcusedRequestNoDates
	}
	trimmed := strings.TrimSpace(note)
	if trimmed == "" {
		return nil, ErrExcusedRequestEmptyNote
	}
	if utf8.RuneCountInString(trimmed) > excusedRequestMaxReasonLen {
		return nil, ErrExcusedRequestNoteTooLong
	}

	sorted := dedupeSortedDates(dates)

	// Serialize concurrent submissions for the same child so the idempotent-retry
	// and overlap checks below (read pending, then insert) can't race two rows
	// through. The advisory lock releases at the end of the ambient transaction.
	if err := s.requestRepo.LockStudentRequests(ctx, studentID); err != nil {
		return nil, err
	}
	existing, err := s.requestRepo.ListPendingForStudent(ctx, studentID)
	if err != nil {
		return nil, fmt.Errorf("active: check existing pending excused requests: %w", err)
	}
	for _, e := range existing {
		if sameDateSet(e.Dates, sorted) {
			// Idempotent retry: a lost response, a double-tap, or a concurrent
			// identical submit. The pending request already exists — return it
			// without creating a duplicate row or re-firing the pill/broadcast.
			return e, nil
		}
		if datesIntersect(e.Dates, sorted) {
			// A partial overlap: a different (non-identical) pending request already
			// covers one of these days. Refuse rather than let two pending requests
			// decide the same date. Disjoint submissions fall through and are allowed.
			return nil, ErrExcusedRequestOverlap
		}
	}

	req := &activeModels.ExcusedAbsenceRequest{
		StudentID:   studentID,
		SubmittedBy: guardianAccountID,
		Dates:       sorted,
		Note:        trimmed,
		Status:      activeModels.ExcusedRequestStatusPending,
	}
	if err := s.requestRepo.Create(ctx, req); err != nil {
		return nil, fmt.Errorf("active: create excused request: %w", err)
	}
	// A new pending request adds the "Freigabe ausstehend" badge on the child;
	// wake staff tabs so planning/search views pick it up without a manual
	// refetch.
	s.broadcastRequestTransition(ctx, req.TenantID, req.StudentID)
	s.emitRequestPillAfterCommit(ctx, req, parentmessaging.ChildEvent{
		EventType:      usersModels.ParentMessageEventRequestCreated,
		ActorKind:      usersModels.ParentMessageSenderGuardian,
		ActorAccountID: guardianAccountID,
		Body:           excusedRequestCreatedBody,
		RequestType:    usersModels.ParentMessageRequestExcusedAbsence,
		RequestStatus:  usersModels.ParentMessageRequestStatusOpen,
	})
	return req, nil
}

func (s *excusedAbsenceRequestService) WithdrawRequest(ctx context.Context, requestID, studentID, guardianAccountID int64) (*activeModels.ExcusedAbsenceRequest, error) {
	// Lock the row regardless of status so ownership is checked BEFORE the
	// pending-status distinction — a foreign child's decided request id must
	// surface as not-found, not not-pending.
	req, err := s.requestRepo.FindByIDForUpdate(ctx, requestID)
	if err != nil {
		return nil, err
	}
	if req.SubmittedBy != guardianAccountID || req.StudentID != studentID {
		return nil, activeModels.ErrExcusedRequestNotFound
	}
	if req.Status != activeModels.ExcusedRequestStatusPending {
		return nil, activeModels.ErrExcusedRequestNotPending
	}
	if err := s.requestRepo.Decide(ctx, req.ID, activeModels.ExcusedRequestStatusWithdrawn, nil, nil, false); err != nil {
		return nil, err
	}
	req.Status = activeModels.ExcusedRequestStatusWithdrawn
	// Withdrawal clears the child's pending badge; wake staff tabs so the
	// planning/search views drop it without a manual refetch.
	s.broadcastRequestTransition(ctx, req.TenantID, req.StudentID)
	s.emitRequestPillAfterCommit(ctx, req, parentmessaging.ChildEvent{
		EventType:      usersModels.ParentMessageEventRequestStatus,
		ActorKind:      usersModels.ParentMessageSenderGuardian,
		ActorAccountID: guardianAccountID,
		Body:           excusedRequestWithdrawnBody,
		RequestType:    usersModels.ParentMessageRequestExcusedAbsence,
		RequestStatus:  usersModels.ParentMessageRequestStatusWithdrawn,
	})
	return req, nil
}

func (s *excusedAbsenceRequestService) ListForStudent(ctx context.Context, studentID int64, recentSince time.Time) ([]*activeModels.ExcusedAbsenceRequest, error) {
	// Pending first (any age) then recently-decided, deduped by id. The parent
	// view shows pending requests indefinitely and rejected ones for a short
	// window so a parent learns their request was declined.
	pending, err := s.requestRepo.ListPendingForStudent(ctx, studentID)
	if err != nil {
		return nil, err
	}
	recent, err := s.requestRepo.ListRecentForStudent(ctx, studentID, recentSince)
	if err != nil {
		return nil, err
	}
	seen := make(map[int64]struct{}, len(pending)+len(recent))
	out := make([]*activeModels.ExcusedAbsenceRequest, 0, len(pending)+len(recent))
	for _, r := range pending {
		if _, ok := seen[r.ID]; ok {
			continue
		}
		seen[r.ID] = struct{}{}
		out = append(out, r)
	}
	for _, r := range recent {
		if _, ok := seen[r.ID]; ok {
			continue
		}
		// A withdrawn request was the parent's own cancellation — drop it. Keep
		// APPROVED requests, though: they normally surface as excused status days,
		// but the parent's status-day view fetches only a bounded window
		// (today..+2 months), so an approval for a past date (a delayed decision)
		// or one more than two months ahead would otherwise vanish entirely — the
		// pending row disappears with nothing replacing it. Returning the approved
		// request lets the parent UI surface those out-of-window confirmations; it
		// de-dupes in-window dates against the status days it already shows (#1845
		// review).
		if r.Status == activeModels.ExcusedRequestStatusWithdrawn {
			continue
		}
		seen[r.ID] = struct{}{}
		out = append(out, r)
	}
	return out, nil
}

func (s *excusedAbsenceRequestService) ListPending(ctx context.Context) ([]*ExcusedRequestReviewItem, error) {
	rows, err := s.requestRepo.ListPendingForTenant(ctx)
	if err != nil {
		return nil, fmt.Errorf("active: list pending excused requests: %w", err)
	}
	if len(rows) == 0 {
		return []*ExcusedRequestReviewItem{}, nil
	}

	studentIDs := make([]int64, 0, len(rows))
	seen := make(map[int64]struct{}, len(rows))
	for _, r := range rows {
		if _, ok := seen[r.StudentID]; !ok {
			seen[r.StudentID] = struct{}{}
			studentIDs = append(studentIDs, r.StudentID)
		}
	}
	students, err := s.studentRepo.FindByIDs(ctx, studentIDs)
	if err != nil {
		return nil, fmt.Errorf("active: load students for excused requests: %w", err)
	}
	personIDs := make([]int64, 0, len(students))
	for _, st := range students {
		personIDs = append(personIDs, st.PersonID)
	}
	persons, err := s.personRepo.FindByIDs(ctx, personIDs)
	if err != nil {
		return nil, fmt.Errorf("active: load persons for excused requests: %w", err)
	}

	// Scope the queue to children the caller may WRITE (admin, or the child's
	// group supervisor) — the same gate as Decide, so a staffer only ever sees
	// requests they can act on. This also scopes the sidebar badge.
	writable := authorize.WritableStudentFilter(ctx, jwt.PermissionsFromCtx(ctx), s.userContext)

	items := make([]*ExcusedRequestReviewItem, 0, len(rows))
	for _, r := range rows {
		if !writable(students[r.StudentID]) {
			continue
		}
		// A graduated child is soft-deleted: their pending requests survive the
		// graduation (the hard delete it replaced cascaded them away), so without
		// this they would sit in the staff queue and the sidebar badge and could
		// still be approved onto an alumnus. A revert brings them back (#405 review).
		if students[r.StudentID].IsAlumnus() {
			continue
		}
		item := &ExcusedRequestReviewItem{Request: r}
		if st, ok := students[r.StudentID]; ok {
			if p, ok := persons[st.PersonID]; ok {
				item.FirstName = p.FirstName
				item.LastName = p.LastName
			}
		}
		items = append(items, item)
	}
	return items, nil
}

func (s *excusedAbsenceRequestService) PendingByStudentForDate(ctx context.Context, date timezone.Date) (map[int64]*activeModels.ExcusedAbsenceRequest, error) {
	rows, err := s.requestRepo.ListPendingForTenant(ctx)
	if err != nil {
		return nil, err
	}
	// rows are newest-first; keep the first (newest) pending request per student
	// that covers the date.
	candidates := make(map[int64]*activeModels.ExcusedAbsenceRequest, len(rows))
	studentIDs := make([]int64, 0, len(rows))
	for _, r := range rows {
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

	// Scope to children the caller may WRITE — the same gate as the review queue
	// and Decide — so a read-only supervisor never sees a pending-approval badge
	// for a child they cannot act on.
	students, err := s.studentRepo.FindByIDs(ctx, studentIDs)
	if err != nil {
		return nil, fmt.Errorf("active: load students for pending excused badges: %w", err)
	}
	writable := authorize.WritableStudentFilter(ctx, jwt.PermissionsFromCtx(ctx), s.userContext)
	out := make(map[int64]*activeModels.ExcusedAbsenceRequest, len(candidates))
	for studentID, req := range candidates {
		// Alumni are excluded for the same reason as in the review queue: a
		// graduated child must not raise a pending-approval marker anywhere on the
		// staff surface (#405 review).
		if writable(students[studentID]) && !students[studentID].IsAlumnus() {
			out[studentID] = req
		}
	}
	return out, nil
}

func (s *excusedAbsenceRequestService) Decide(ctx context.Context, input ExcusedRequestDecideInput) (*ExcusedRequestReviewItem, error) {
	if input.RequestID <= 0 {
		return nil, activeModels.ErrExcusedRequestNotFound
	}
	reason := strings.TrimSpace(input.Reason)
	if !input.Approve {
		if reason == "" {
			return nil, ErrExcusedRequestRejectReasonRequired
		}
		if utf8.RuneCountInString(reason) > excusedRequestMaxReasonLen {
			return nil, ErrExcusedRequestRejectReasonTooLong
		}
	}

	// Lock the row and re-verify it is still pending so two staff deciding the
	// same request (or a decide racing the guardian's withdrawal) serialize.
	req, err := s.requestRepo.FindPendingByIDForUpdate(ctx, input.RequestID)
	if err != nil {
		return nil, err
	}

	// Per-child write authorization: the caller may decide only if they could
	// edit the child directly — admin, or the child's group supervisor.
	//
	// Taken FOR UPDATE so the alumnus gate below decides on a state a concurrent
	// grade transition cannot change underneath it: the transition apply locks
	// exactly this row before flipping it to alumnus, so an unlocked read could
	// see "active", let the approve through, and have the graduation commit
	// before the status-day writes land — excused rows written for, or cleared
	// from, a child who is an alumnus by then. Under the lock the two serialize.
	// This is the only student row this transaction locks, so the ascending-id
	// order every student-row locker follows is preserved (#405 review).
	student, err := s.studentRepo.FindByIDForUpdate(ctx, req.StudentID)
	if err != nil {
		return nil, fmt.Errorf("active: load student for excused request decision: %w", err)
	}
	// The child graduated after filing this request. The lookup is unfiltered, so
	// without this gate an approve would still write excused status days for an
	// alumnus. Same 404 the rest of the child surface returns for graduates
	// (#405 review).
	if student.IsAlumnus() {
		return nil, activeModels.ErrExcusedRequestNotFound
	}
	if ok, _ := authorize.CanUpdateStudent(ctx, jwt.PermissionsFromCtx(ctx), student, s.userContext); !ok {
		return nil, ErrExcusedRequestForbidden
	}

	if input.Approve {
		// Refuse to APPLY when the submitting guardian has lost access to the
		// child (unlinked or parent_portal.access revoked) since the request was
		// filed. Approving writes parent-sourced excused status days and posts a
		// parent-visible "bestätigt" pill for a recipient the parent APIs now
		// hide; staff wind such a stale request down by REJECTING it (reject is
		// deliberately not gated). Mirrors the care-schedule request guard.
		if s.emitter != nil {
			hasAccess, err := s.emitter.GuardianHasChildAccess(ctx, req.StudentID, req.SubmittedBy)
			if err != nil {
				return nil, fmt.Errorf("active: excused request guardian link check: %w", err)
			}
			if !hasAccess {
				return nil, ErrExcusedRequestGuardianAccessRevoked
			}
		}
		// Refuse to APPLY when the child's absence for a requested date was created
		// or changed AFTER this request was filed (a newer sick / class-trip /
		// excused record from staff or another flow). applyExcusedRequest clears
		// every other status for those dates and upserts excused, so a blind approve
		// would silently overwrite that newer decision. Detect it and return a
		// conflict; staff resolve it by rejecting the stale request.
		if err := s.ensureNoNewerStatus(ctx, req); err != nil {
			return nil, err
		}
		// Apply, status update and after-commit hooks all run in the ambient
		// tenant transaction. A mid-apply failure propagates as a plain error
		// (→ 500) so the WHOLE transaction rolls back rather than committing a
		// half-applied absence.
		if err := s.applyExcusedRequest(ctx, req); err != nil {
			return nil, err
		}
	}

	newStatus := activeModels.ExcusedRequestStatusApproved
	pillStatus := usersModels.ParentMessageRequestStatusDone
	pillBody := excusedRequestConfirmedBody
	var reasonPtr *string
	if !input.Approve {
		newStatus = activeModels.ExcusedRequestStatusRejected
		pillStatus = usersModels.ParentMessageRequestStatusRejected
		pillBody = "Abmeldung abgelehnt: " + reason
		reasonPtr = &reason
	}
	reviewedBy := input.ReviewedBy
	if err := s.requestRepo.Decide(ctx, req.ID, newStatus, reasonPtr, &reviewedBy, input.Approve); err != nil {
		return nil, err
	}

	if input.Approve {
		if s.absenceNotify != nil {
			tenantID := req.TenantID
			if tenantID <= 0 {
				tenantID = tenant.FromContext(ctx)
			}
			report := notificationsService.AbsenceReport{
				TenantID:           tenantID,
				StudentIDs:         []int64{req.StudentID},
				Status:             activeModels.StudentStatusDayExcused,
				Dates:              req.Dates,
				FromParent:         true,
				ActorAccountID:     input.ReviewedBy,
				ExcludedAccountIDs: []int64{req.SubmittedBy},
			}
			tenant.RegisterAfterCommit(ctx, func() {
				s.absenceNotify.NotifyAbsenceReported(context.Background(), report)
			})
		}
		tenant.RegisterAfterCommit(ctx, func() {
			s.logger.Info("excused request approved",
				slog.Int64("request_id", req.ID),
				slog.Int64("student_id", req.StudentID),
				slog.Int64("tenant_id", req.TenantID),
				slog.Int64("reviewed_by", input.ReviewedBy),
				slog.Int("days", len(req.Dates)),
			)
		})
	} else {
		tenant.RegisterAfterCommit(ctx, func() {
			s.logger.Info("excused request rejected",
				slog.Int64("request_id", req.ID),
				slog.Int64("student_id", req.StudentID),
				slog.Int64("tenant_id", req.TenantID),
				slog.Int64("reviewed_by", input.ReviewedBy),
			)
		})
	}
	// Either decision clears the child's pending badge (approval also surfaces
	// the confirmed absence). Wake staff tabs for both outcomes.
	s.broadcastRequestTransition(ctx, req.TenantID, req.StudentID)
	s.emitRequestPillAfterCommit(ctx, req, parentmessaging.ChildEvent{
		EventType:      usersModels.ParentMessageEventRequestStatus,
		ActorKind:      usersModels.ParentMessageSenderStaff,
		ActorAccountID: input.ReviewedBy,
		Body:           pillBody,
		RequestType:    usersModels.ParentMessageRequestExcusedAbsence,
		RequestStatus:  pillStatus,
		DecisionReason: reason,
	})

	row, err := s.requestRepo.FindByID(ctx, req.ID)
	if err != nil {
		return nil, fmt.Errorf("active: reload decided excused request: %w", err)
	}
	item := &ExcusedRequestReviewItem{Request: row}
	if student != nil {
		if person, perr := s.personRepo.FindByID(ctx, student.PersonID); perr == nil && person != nil {
			item.FirstName = person.FirstName
			item.LastName = person.LastName
		}
	}
	return item, nil
}

// ensureNoNewerStatus refuses the approval when any ACTIVE status day on a
// requested date was reported after the request was created — i.e. a newer
// absence decision (sick, class trip, or an updated excused note) that approving
// would clobber. It compares each active row's reported_at against the request's
// created_at: a status set BEFORE the request is the older state the parent's
// request legitimately supersedes (no conflict); a status set AFTER is a fresher
// decision that must win. Runs inside the ambient tenant transaction, right
// before applyExcusedRequest, so the window between check and write is minimal.
func (s *excusedAbsenceRequestService) ensureNoNewerStatus(ctx context.Context, req *activeModels.ExcusedAbsenceRequest) error {
	if len(req.Dates) == 0 {
		return nil
	}
	// req.Dates is stored sorted ascending (dedupeSortedDates), so first/last bound
	// the range; the query spans min..max and we filter back to the exact dates.
	minDate := req.Dates[0]
	maxDate := req.Dates[len(req.Dates)-1]
	rows, err := s.statusDayRepo.FindActiveByStudentAndDateRange(ctx, req.StudentID, minDate, maxDate)
	if err != nil {
		return err
	}
	dateSet := make(map[timezone.Date]struct{}, len(req.Dates))
	for _, d := range req.Dates {
		dateSet[d] = struct{}{}
	}
	for _, row := range rows {
		if _, ok := dateSet[row.Date]; !ok {
			continue
		}
		if row.ReportedAt.After(req.CreatedAt) {
			return ErrExcusedRequestStatusConflict
		}
	}
	return nil
}

// sameDateSet reports whether two date slices contain exactly the same days.
// Both are stored deduped+sorted, so a length + positional comparison suffices.
func sameDateSet(a, b []timezone.Date) bool {
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
func datesIntersect(a, b []timezone.Date) bool {
	set := make(map[timezone.Date]struct{}, len(a))
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

// applyExcusedRequest writes the approved request's dates as excused status
// days, mirroring the direct parent path: clear any opposite status for those
// dates, upsert one excused row per date carrying the request note, and — when
// today is included — clear a stale live sick flag (excused sets no live flag,
// per issue #1735). Runs in the ambient tenant transaction.
func (s *excusedAbsenceRequestService) applyExcusedRequest(ctx context.Context, req *activeModels.ExcusedAbsenceRequest) error {
	if len(req.Dates) == 0 {
		return ErrExcusedRequestNoDates
	}
	now := time.Now()
	note := req.Note
	notePtr := &note

	for _, other := range activeModels.StudentStatusDayStatusesExcept(activeModels.StudentStatusDayExcused) {
		if err := s.statusDayRepo.MarkClearedForDates(ctx, req.StudentID, other, req.Dates, now, activeModels.StudentStatusSourceParent); err != nil {
			return err
		}
	}
	for _, d := range req.Dates {
		if err := s.statusDayRepo.UpsertReported(ctx, &activeModels.StudentStatusDay{
			StudentID:  req.StudentID,
			Date:       d,
			Status:     activeModels.StudentStatusDayExcused,
			ReportedAt: now,
			Source:     activeModels.StudentStatusSourceParent,
			Note:       notePtr,
		}); err != nil {
			return err
		}
	}

	if containsDate(req.Dates, timezone.TodayDate()) {
		fresh, err := s.studentRepo.FindByIDForUpdate(ctx, req.StudentID)
		if err != nil {
			return err
		}
		falseVal := false
		fresh.Sick = &falseVal
		fresh.SickSince = nil
		if err := s.studentRepo.Update(ctx, fresh); err != nil {
			return err
		}
	}
	return nil
}

// emitRequestPillAfterCommit schedules the notification pill for after the
// ambient transaction commits. The emitter is best-effort and opens its own
// tenant transaction; ref fields always point at the request row.
func (s *excusedAbsenceRequestService) emitRequestPillAfterCommit(ctx context.Context, req *activeModels.ExcusedAbsenceRequest, ev parentmessaging.ChildEvent) {
	if s.emitter == nil {
		return
	}
	tenantID := req.TenantID
	if tenantID <= 0 {
		tenantID = tenant.FromContext(ctx)
	}
	refID := req.ID
	ev.RefTable = "active.excused_absence_requests"
	ev.RefID = &refID
	studentID := req.StudentID
	guardianAccountID := req.SubmittedBy
	tenant.RegisterAfterCommit(ctx, func() {
		s.emitter.EmitChildEvent(tenantID, studentID, guardianAccountID, ev)
	})
}

// broadcastRequestTransition wakes BOTH audiences after any excused-request
// transition that changes the child's "Freigabe ausstehend" badge — a new pending
// request (badge appears), an approval (badge clears, confirmed absence shows), or
// a rejection/withdrawal (badge clears with no absence):
//
//   - Staff: a tenant-wide student_updated so an open dashboard/planning/search
//     view drops or adds the badge instead of showing stale pending state.
//   - Guardians: a message-INDEPENDENT invalidation to EVERY guardian of the child
//     (not only the one who acted, and even when parent messaging is off) so an
//     already-open parents-app tab refetches the child's care state in real time.
//     This is the event-based recovery the review asked for; the parents view's
//     focus refetch remains as a fallback for a tab whose SSE stream had dropped.
//
// Fire-and-forget: both wakes run AFTER commit so a woken client never reads the
// pre-commit snapshot.
func (s *excusedAbsenceRequestService) broadcastRequestTransition(ctx context.Context, tenantID, studentID int64) {
	if tenantID <= 0 {
		tenantID = tenant.FromContext(ctx)
	}
	tenant.RegisterAfterCommit(ctx, func() {
		if s.broadcaster != nil {
			source := "excused_request"
			// student_updated drives the child-card "Freigabe ausstehend" badge and
			// student list/detail caches.
			studentEvent := realtime.NewEvent(realtime.EventStudentUpdated, "", realtime.EventData{Source: &source})
			if err := s.broadcaster.BroadcastToTenant(tenantID, studentEvent); err != nil {
				s.logger.Warn("active: broadcast student_updated after excused request decision failed",
					slog.Int64("tenant_id", tenantID),
					slog.Int64("student_id", studentID),
					slog.String("error", err.Error()),
				)
			}
			// change_requests_changed drives the staff review queue + pending-count
			// badge in real time, independent of the parent-messaging pill (which a
			// messaging-off school never emits) — the review's staff-side counterpart
			// to the guardian fan-out below.
			queueEvent := realtime.NewEvent(realtime.EventChangeRequestsChanged, "", realtime.EventData{Source: &source})
			if err := s.broadcaster.BroadcastToTenant(tenantID, queueEvent); err != nil {
				s.logger.Warn("active: broadcast change_requests_changed after excused request transition failed",
					slog.Int64("tenant_id", tenantID),
					slog.Int64("student_id", studentID),
					slog.String("error", err.Error()),
				)
			}
		}
		if s.emitter != nil {
			s.emitter.BroadcastChildUpdateToGuardians(tenantID, studentID)
		}
	})
}

// dedupeSortedDates removes duplicate dates and returns them ascending, so the
// stored payload is canonical regardless of client ordering.
func dedupeSortedDates(dates []timezone.Date) []timezone.Date {
	seen := make(map[timezone.Date]struct{}, len(dates))
	out := make([]timezone.Date, 0, len(dates))
	for _, d := range dates {
		if _, ok := seen[d]; ok {
			continue
		}
		seen[d] = struct{}{}
		out = append(out, d)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Before(out[j])
	})
	return out
}

// containsDate reports whether needle is in dates.
func containsDate(dates []timezone.Date, needle timezone.Date) bool {
	for _, d := range dates {
		if d == needle {
			return true
		}
	}
	return false
}
