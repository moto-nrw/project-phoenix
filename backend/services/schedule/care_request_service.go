// Care-schedule change requests (#1803): a guardian requests a permanent
// change to their child's weekly care plan (departure mode + arrival/pickup
// times per weekday); staff review and decide it on the central
// "Änderungsanfragen" page. The request lifecycle lives in
// schedule.care_schedule_change_requests — the chat only receives
// non-interactive notification pills (created / decided / withdrawn) via
// parentmessaging.Emitter.
//
// The payload machinery (canonicalize / validate / apply / diff) moved here
// from services/messaging/requests.go when the requests were decoupled from
// the chat; the merge-not-replace apply semantics and the shared
// create/apply parser are preserved unchanged.
package schedule

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/realtime"
	"github.com/moto-nrw/project-phoenix/services/parentmessaging"
	userContextService "github.com/moto-nrw/project-phoenix/services/usercontext"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// careRequestMaxReasonLen bounds the staff reject reason (in runes) so a
// direct API client cannot persist an unbounded payload. Matches the
// message-body limit the chat used for the same field.
const careRequestMaxReasonLen = 2000

// careRequestPendingUniqueIndex is the partial unique index enforcing one
// open request per student; its violation maps to ErrCareRequestAlreadyPending.
const careRequestPendingUniqueIndex = "uniq_care_schedule_change_requests_pending"

// German pill texts. The staff portal renders these directly; the parents
// portal localizes from the structured event fields instead.
const (
	careRequestCreatedBody   = "Anfrage: Dauerhafte Betreuungszeiten ändern"
	careRequestConfirmedBody = "Anfrage bestätigt, Betreuungszeiten übernommen"
	careRequestWithdrawnBody = "Anfrage zurückgezogen"
)

var (
	// ErrCareRequestForbidden means the caller may not decide requests for this
	// child (no staff identity, or no write access to the student).
	ErrCareRequestForbidden = errors.New("schedule: care request forbidden")
	// ErrCareRequestAlreadyPending means the student already has an open
	// request (partial unique index) — the guardian withdraws and re-submits
	// instead of stacking requests.
	ErrCareRequestAlreadyPending = errors.New("schedule: care request already pending for this student")
	// ErrInvalidCareRequestPayload means the payload failed shape/policy
	// validation (bad weekday, mode, or time).
	ErrInvalidCareRequestPayload = errors.New("schedule: invalid care request payload")
	// ErrCareRequestGuardianAccessRevoked means the submitting guardian is no
	// longer a linked guardian of the child with parent_portal.access, so
	// approving (which OVERWRITES the child's permanent weekly plan and posts a
	// parent-visible "bestätigt" pill for a recipient who can no longer read it)
	// is refused. Staff wind such a request down by REJECTING it — the reject
	// path is deliberately not gated on the guardian link, mirroring the chat's
	// old ConfirmRequest/requireLinkedGuardian split this flow replaced.
	ErrCareRequestGuardianAccessRevoked = errors.New("schedule: care request guardian access revoked")
	// ErrCareRequestMessagingDisabled means the school turned parent-OGS
	// messaging (operations.parent_notes_enabled) OFF after the request was
	// filed. Approving APPLIES the permanent weekly-plan change AND the
	// guardian's "bestätigt" pill is the only notification channel — the emitter
	// drops that pill while messaging is off, so approving would silently change
	// the child's plan with no parent notice. Approval is therefore refused;
	// staff wind the request down by REJECTING it (reject is deliberately not
	// gated). Mirrors the chat's requireEnabled gate this flow replaced.
	ErrCareRequestMessagingDisabled = errors.New("schedule: care request messaging disabled")
	// ErrCareRequestRejectReasonRequired means staff rejected without a reason.
	ErrCareRequestRejectReasonRequired = errors.New("schedule: reject reason is required")
	// ErrCareRequestRejectReasonTooLong means the reason exceeded the bound.
	ErrCareRequestRejectReasonTooLong = errors.New("schedule: reject reason too long")
)

// Diff care-kind discriminators (see RequestDiffEntry.CareKind). Stable wire
// tokens — the localized parents portal maps them to its own labels.
const (
	DiffCareKindArrival       = "arrival"
	DiffCareKindPickup        = "pickup"
	DiffCareKindDepartureMode = "departure_mode"
)

// RequestDiffEntry is one field-level "current → requested" comparison row in
// a change request's diff (staff- and parent-visible).
type RequestDiffEntry struct {
	Label string
	Old   string
	New   string
	// Structured discriminators that let a localized client (the parents portal)
	// render the label in its own language instead of the German Label, which is
	// authoritative only for the German-only staff portal. Weekday (1-5) +
	// CareKind identify a care-schedule row.
	Weekday  int
	CareKind string
	// OldModes / NewMode carry the raw departure-mode keys (alone|bus|pickup|
	// accompanied) behind Old/New for DiffCareKindDepartureMode rows, so the
	// localized parents portal renders the mode names in the guardian's
	// language. OldModes is the day's allowed set (an "alone" entry stands in
	// for an empty set); NewMode is the single requested mode. Empty for
	// arrival/pickup rows (times are language-neutral).
	OldModes []string
	NewMode  string
}

// CareRequestReviewItem is one request enriched with the child's name and the
// live "current → requested" diff for the staff review queue.
type CareRequestReviewItem struct {
	Request   *scheduleModels.CareScheduleChangeRequest
	FirstName string
	LastName  string
	Diff      []RequestDiffEntry
}

// CareRequestDecideInput carries a staff decision on one pending request.
type CareRequestDecideInput struct {
	RequestID int64
	Approve   bool
	Reason    string
	// ReviewedBy is the acting staff ACCOUNT id (auth.accounts), stamped as
	// reviewed_by and used as the pill's actor.
	ReviewedBy int64
}

// CareScheduleRequestService owns the care-schedule change-request lifecycle.
// All methods run inside an ambient tenant transaction: the staff paths under
// the request's TenantTxMiddleware, the parent paths inside the
// tenant.WithTenantTx the parent service opens after resolving the child.
type CareScheduleRequestService interface {
	// CreateRequest canonicalizes and stores a guardian's request, then (after
	// commit) posts the "Anfrage erstellt" pill. The caller has already
	// authorized the guardian for this child.
	CreateRequest(ctx context.Context, studentID, guardianAccountID int64, payload map[string]any) (*scheduleModels.CareScheduleChangeRequest, error)
	// WithdrawRequest moves the submitter's own pending request to withdrawn
	// and (after commit) posts the withdrawal pill. studentID pins the request
	// to the child the caller was authorized for.
	WithdrawRequest(ctx context.Context, requestID, studentID, guardianAccountID int64) (*scheduleModels.CareScheduleChangeRequest, error)
	// GetPendingForStudent returns the child's open request (nil when none)
	// with its live "current → requested" diff, for the parent read view.
	GetPendingForStudent(ctx context.Context, studentID int64) (*scheduleModels.CareScheduleChangeRequest, []RequestDiffEntry, error)
	// ListPending returns every pending request for the current tenant,
	// newest-first, enriched with child names and live diffs — the staff
	// review queue on the Änderungsanfragen page.
	ListPending(ctx context.Context) ([]*CareRequestReviewItem, error)
	// Decide approves (applies the weekly plan, then stamps) or rejects
	// (reason required) one pending request and returns the refreshed row.
	// After commit it posts the decision pill and fires the schedule cache
	// invalidations.
	Decide(ctx context.Context, input CareRequestDecideInput) (*CareRequestReviewItem, error)
}

type careScheduleRequestService struct {
	requestRepo scheduleModels.CareScheduleChangeRequestRepository
	studentRepo usersModels.StudentRepository
	personRepo  usersModels.PersonRepository
	arrival     ArrivalScheduleService
	pickup      PickupScheduleService
	userContext userContextService.UserContextService
	emitter     *parentmessaging.Emitter
	broadcaster realtime.Broadcaster
	logger      *slog.Logger
}

// NewCareScheduleRequestService wires the care-request service.
func NewCareScheduleRequestService(
	requestRepo scheduleModels.CareScheduleChangeRequestRepository,
	studentRepo usersModels.StudentRepository,
	personRepo usersModels.PersonRepository,
	arrival ArrivalScheduleService,
	pickup PickupScheduleService,
	userContext userContextService.UserContextService,
	emitter *parentmessaging.Emitter,
	broadcaster realtime.Broadcaster,
	logger *slog.Logger,
) CareScheduleRequestService {
	if logger == nil {
		logger = slog.Default()
	}
	return &careScheduleRequestService{
		requestRepo: requestRepo,
		studentRepo: studentRepo,
		personRepo:  personRepo,
		arrival:     arrival,
		pickup:      pickup,
		userContext: userContext,
		emitter:     emitter,
		broadcaster: broadcaster,
		logger:      logger,
	}
}

func (s *careScheduleRequestService) CreateRequest(ctx context.Context, studentID, guardianAccountID int64, payload map[string]any) (*scheduleModels.CareScheduleChangeRequest, error) {
	canonical, err := canonicalizeCareSchedulePayload(payload)
	if err != nil {
		return nil, err
	}
	req := &scheduleModels.CareScheduleChangeRequest{
		StudentID:   studentID,
		SubmittedBy: guardianAccountID,
		Payload:     canonical,
		Status:      scheduleModels.CareRequestStatusPending,
	}
	if err := s.requestRepo.Create(ctx, req); err != nil {
		if isCareRequestPendingUniqueViolation(err) {
			return nil, ErrCareRequestAlreadyPending
		}
		return nil, fmt.Errorf("schedule: create care request: %w", err)
	}
	s.emitRequestPillAfterCommit(ctx, req, parentmessaging.ChildEvent{
		EventType:      "request_created",
		ActorKind:      usersModels.ParentMessageSenderGuardian,
		ActorAccountID: guardianAccountID,
		Body:           careRequestCreatedBody,
		RequestType:    usersModels.ParentMessageRequestCareSchedule,
		RequestStatus:  usersModels.ParentMessageRequestStatusOpen,
	})
	s.wakeGuardiansAfterCommit(ctx, req)
	return req, nil
}

func (s *careScheduleRequestService) WithdrawRequest(ctx context.Context, requestID, studentID, guardianAccountID int64) (*scheduleModels.CareScheduleChangeRequest, error) {
	// Lock the row regardless of status so ownership is checked BEFORE the
	// pending-status distinction — FindPendingByIDForUpdate would collapse a
	// terminal (already decided/withdrawn) row into ErrCareRequestNotPending
	// before the ownership check below runs, letting a stranger probe a foreign
	// child's decided request id via a 409 instead of the intended 404.
	req, err := s.requestRepo.FindByIDForUpdate(ctx, requestID)
	if err != nil {
		return nil, err
	}
	// Only the submitting guardian withdraws their own request, and only under
	// the child the caller was authorized for. Report a foreign request as
	// not-found rather than forbidden so the id space is not probeable.
	if req.SubmittedBy != guardianAccountID || req.StudentID != studentID {
		return nil, scheduleModels.ErrCareRequestNotFound
	}
	// The caller owns a real request; only a still-pending one can be withdrawn.
	// A caller's own already-terminal request surfaces as not-pending (409),
	// distinct from the not-found (404) a foreign id gets above.
	if req.Status != scheduleModels.CareRequestStatusPending {
		return nil, scheduleModels.ErrCareRequestNotPending
	}
	if err := s.requestRepo.Decide(ctx, req.ID, scheduleModels.CareRequestStatusWithdrawn, nil, nil, false); err != nil {
		return nil, err
	}
	req.Status = scheduleModels.CareRequestStatusWithdrawn
	s.emitRequestPillAfterCommit(ctx, req, parentmessaging.ChildEvent{
		EventType:      "request_status",
		ActorKind:      usersModels.ParentMessageSenderGuardian,
		ActorAccountID: guardianAccountID,
		Body:           careRequestWithdrawnBody,
		RequestType:    usersModels.ParentMessageRequestCareSchedule,
		RequestStatus:  usersModels.ParentMessageRequestStatusWithdrawn,
	})
	s.wakeGuardiansAfterCommit(ctx, req)
	return req, nil
}

func (s *careScheduleRequestService) GetPendingForStudent(ctx context.Context, studentID int64) (*scheduleModels.CareScheduleChangeRequest, []RequestDiffEntry, error) {
	req, err := s.requestRepo.GetPendingForStudent(ctx, studentID)
	if err != nil {
		return nil, nil, err
	}
	if req == nil {
		return nil, nil, nil
	}
	src := &careDiffSource{s: s, studentID: studentID}
	diff, err := s.careScheduleDiffFrom(ctx, src, req.Payload)
	if err != nil {
		// The diff is presentation — a failed load must not hide the pending
		// request itself from the parent read view.
		s.logger.Warn("schedule: build care request diff failed",
			slog.Int64("request_id", req.ID),
			slog.String("error", err.Error()),
		)
		return req, nil, nil
	}
	return req, diff, nil
}

func (s *careScheduleRequestService) ListPending(ctx context.Context) ([]*CareRequestReviewItem, error) {
	rows, err := s.requestRepo.ListPendingForTenant(ctx)
	if err != nil {
		return nil, fmt.Errorf("schedule: list pending care requests: %w", err)
	}
	if len(rows) == 0 {
		return []*CareRequestReviewItem{}, nil
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
		return nil, fmt.Errorf("schedule: load students for care requests: %w", err)
	}
	personIDs := make([]int64, 0, len(students))
	for _, st := range students {
		personIDs = append(personIDs, st.PersonID)
	}
	persons, err := s.personRepo.FindByIDs(ctx, personIDs)
	if err != nil {
		return nil, fmt.Errorf("schedule: load persons for care requests: %w", err)
	}

	// Scope the queue to children the caller may WRITE (admin, or the child's
	// group supervisor) — the same gate as Decide, so a staffer only ever sees
	// requests they can actually act on. This also scopes the sidebar badge, which
	// sums these ListPending results.
	writable := authorize.WritableStudentFilter(ctx, jwt.PermissionsFromCtx(ctx), s.userContext)

	items := make([]*CareRequestReviewItem, 0, len(rows))
	// One diff source per student so several requests never re-read the same
	// child (mirrors the per-call memoization the chat diff builder had).
	sources := map[int64]*careDiffSource{}
	for _, r := range rows {
		if !writable(students[r.StudentID]) {
			continue
		}
		// A graduated child is soft-deleted: the parent portal already hides them,
		// but their pending requests survive graduation (a hard delete used to
		// cascade them away). Keeping them here would leave the request in the
		// staff queue and the sidebar badge, and let staff approve a change onto an
		// alumnus. They reappear if the transition is reverted (#405 review).
		if students[r.StudentID].IsAlumnus() {
			continue
		}
		item := &CareRequestReviewItem{Request: r}
		if st, ok := students[r.StudentID]; ok {
			if p, ok := persons[st.PersonID]; ok {
				item.FirstName = p.FirstName
				item.LastName = p.LastName
			}
		}
		src, ok := sources[r.StudentID]
		if !ok {
			src = &careDiffSource{s: s, studentID: r.StudentID}
			sources[r.StudentID] = src
		}
		diff, err := s.careScheduleDiffFrom(ctx, src, r.Payload)
		if err != nil {
			// A diff that can't be built is logged and skipped rather than
			// failing the whole queue load.
			s.logger.Warn("schedule: build care request diff failed",
				slog.Int64("request_id", r.ID),
				slog.String("error", err.Error()),
			)
		} else {
			item.Diff = diff
		}
		items = append(items, item)
	}
	return items, nil
}

func (s *careScheduleRequestService) Decide(ctx context.Context, input CareRequestDecideInput) (*CareRequestReviewItem, error) {
	if input.RequestID <= 0 {
		return nil, scheduleModels.ErrCareRequestNotFound
	}
	reason := strings.TrimSpace(input.Reason)
	if !input.Approve {
		if reason == "" {
			return nil, ErrCareRequestRejectReasonRequired
		}
		// Count runes so German umlauts are not penalized.
		if utf8.RuneCountInString(reason) > careRequestMaxReasonLen {
			return nil, ErrCareRequestRejectReasonTooLong
		}
	}

	// Lock the row and re-verify it is still pending. Two staff deciding the
	// same request (or a decide racing the guardian's withdrawal) serialize
	// here: the second sees the terminal status and gets NotPending instead
	// of applying/flipping it twice.
	req, err := s.requestRepo.FindPendingByIDForUpdate(ctx, input.RequestID)
	if err != nil {
		return nil, err
	}

	// Per-child write authorization: the caller may decide this request only if
	// they could edit the child directly — admin, or the child's group supervisor
	// (auth/authorize.CanUpdateStudent, the exact gate the direct student edit
	// uses). The route now gates on users:update (not users:manage), so the
	// deciding surface matches "who may change this child's data" instead of
	// blanket admin, and a supervising staffer gets a signal + action for their
	// own group's requests. Reject is gated identically to approve: a staffer who
	// cannot edit the child has no business winding its request down either.
	student, err := s.studentRepo.FindByID(ctx, req.StudentID)
	if err != nil {
		return nil, fmt.Errorf("schedule: load student for care request decision: %w", err)
	}
	// The child graduated after filing this request. FindByID is unfiltered, so
	// without this gate an approve would still rewrite an alumnus' care plan (and
	// a reject would post a pill to a portal that no longer shows the child). Same
	// 404 the whole child surface returns for graduates — as if the request had
	// been cascaded away by the hard delete graduation replaced (#405 review).
	if student.IsAlumnus() {
		return nil, scheduleModels.ErrCareRequestNotFound
	}
	if ok, _ := authorize.CanUpdateStudent(ctx, jwt.PermissionsFromCtx(ctx), student, s.userContext); !ok {
		return nil, ErrCareRequestForbidden
	}

	// Whether the apply actually changed the child's "läuft mit" links — the
	// only thing that may announce a companion change.
	companionsChanged := false
	if input.Approve {
		// Two apply-only gates, both mirroring the chat's ConfirmRequest path this
		// flow replaced. Both run in the ambient tenant tx, on the
		// locked-still-pending row; reject skips both so staff can always wind a
		// stuck request down.
		if s.emitter != nil {
			// (1) Refuse to APPLY when messaging is OFF for the school. Approving
			// posts the guardian's "bestätigt" pill as its only notification, and
			// the emitter drops that pill while operations.parent_notes_enabled is
			// off — so approving would change the child's permanent plan with no
			// parent notice. The chat's ConfirmRequest gated exactly this via
			// requireEnabled. Fail-open on a transient settings blip (see the
			// emitter helper).
			tenantID := req.TenantID
			if tenantID <= 0 {
				tenantID = tenant.FromContext(ctx)
			}
			if !s.emitter.MessagingEnabledForTenant(ctx, tenantID) {
				return nil, ErrCareRequestMessagingDisabled
			}
			// (2) Refuse to APPLY when the submitting guardian has lost access to
			// the child (unlinked or parent_portal.access revoked) since the
			// request was filed. Approving overwrites the child's permanent weekly
			// plan and posts a parent-visible pill for a recipient the parent APIs
			// now hide; the chat's ConfirmRequest gated this via
			// requireLinkedGuardian.
			hasAccess, err := s.emitter.GuardianHasChildAccess(ctx, req.StudentID, req.SubmittedBy)
			if err != nil {
				return nil, fmt.Errorf("schedule: care request guardian link check: %w", err)
			}
			if !hasAccess {
				return nil, ErrCareRequestGuardianAccessRevoked
			}
		}
		// The apply, the status update and the after-commit hooks all run in the
		// ambient tenant transaction. A mid-apply failure must propagate as a
		// plain error (→ 500) so the WHOLE transaction rolls back — masking it
		// as a 409 would commit a half-applied weekly plan.
		linksChanged, err := s.applyCareScheduleRequest(ctx, req)
		if err != nil {
			return nil, err
		}
		companionsChanged = linksChanged
	}

	newStatus := scheduleModels.CareRequestStatusApproved
	pillStatus := usersModels.ParentMessageRequestStatusDone
	pillBody := careRequestConfirmedBody
	var reasonPtr *string
	if !input.Approve {
		newStatus = scheduleModels.CareRequestStatusRejected
		pillStatus = usersModels.ParentMessageRequestStatusRejected
		pillBody = "Anfrage abgelehnt: " + reason
		reasonPtr = &reason
	}
	reviewedBy := input.ReviewedBy
	if err := s.requestRepo.Decide(ctx, req.ID, newStatus, reasonPtr, &reviewedBy, input.Approve); err != nil {
		return nil, err
	}

	// Post-commit side effects: audit line, cache invalidation, decision pill.
	// All tied to the durable outcome — a rollback fires none of them.
	if input.Approve {
		tenant.RegisterAfterCommit(ctx, func() {
			s.recordApplyAudit(req, input.ReviewedBy)
		})
		s.broadcastCareScheduleChanges(ctx, req.TenantID, req.StudentID, companionsChanged)
	} else {
		tenant.RegisterAfterCommit(ctx, func() {
			s.logger.Info("care request rejected",
				slog.Int64("request_id", req.ID),
				slog.Int64("student_id", req.StudentID),
				slog.Int64("tenant_id", req.TenantID),
				slog.Int64("reviewed_by", input.ReviewedBy),
			)
		})
	}
	s.emitRequestPillAfterCommit(ctx, req, parentmessaging.ChildEvent{
		EventType:      "request_status",
		ActorKind:      usersModels.ParentMessageSenderStaff,
		ActorAccountID: input.ReviewedBy,
		Body:           pillBody,
		RequestType:    usersModels.ParentMessageRequestCareSchedule,
		RequestStatus:  pillStatus,
		DecisionReason: reason,
	})
	s.wakeGuardiansAfterCommit(ctx, req)

	row, err := s.requestRepo.FindByID(ctx, req.ID)
	if err != nil {
		return nil, fmt.Errorf("schedule: reload decided care request: %w", err)
	}
	item := &CareRequestReviewItem{Request: row}
	student, err = s.studentRepo.FindByID(ctx, row.StudentID)
	if err == nil && student != nil {
		if person, perr := s.personRepo.FindByID(ctx, student.PersonID); perr == nil && person != nil {
			item.FirstName = person.FirstName
			item.LastName = person.LastName
		}
	}
	return item, nil
}

// emitRequestPillAfterCommit schedules the notification pill for after the
// ambient transaction commits. The emitter is best-effort and opens its own
// tenant transaction; ref fields always point at the request row.
func (s *careScheduleRequestService) emitRequestPillAfterCommit(ctx context.Context, req *scheduleModels.CareScheduleChangeRequest, ev parentmessaging.ChildEvent) {
	if s.emitter == nil {
		return
	}
	tenantID := req.TenantID
	if tenantID <= 0 {
		tenantID = tenant.FromContext(ctx)
	}
	refID := req.ID
	ev.RefTable = "schedule.care_schedule_change_requests"
	ev.RefID = &refID
	studentID := req.StudentID
	guardianAccountID := req.SubmittedBy
	tenant.RegisterAfterCommit(ctx, func() {
		s.emitter.EmitChildEvent(tenantID, studentID, guardianAccountID, ev)
	})
}

// wakeGuardiansAfterCommit fans a message-INDEPENDENT parent_child_updated out to
// EVERY guardian of the child after the ambient transaction commits, so a
// co-guardian with the child open refetches the request/pickup state a lifecycle
// transition changed. emitRequestPillAfterCommit only touches the SUBMITTING
// guardian's own thread (and staff-side broadcasts never reach the parents SSE
// stream), so without this a second guardian keeps a stale "Anfrage offen" badge
// after a submit/withdraw, or a stale pickup time after an approve, until they
// refocus or reload (#1725). Best-effort and after-commit, mirroring
// emitRequestPillAfterCommit; a nil emitter is a no-op.
func (s *careScheduleRequestService) wakeGuardiansAfterCommit(ctx context.Context, req *scheduleModels.CareScheduleChangeRequest) {
	if s.emitter == nil {
		return
	}
	tenantID := req.TenantID
	if tenantID <= 0 {
		tenantID = tenant.FromContext(ctx)
	}
	studentID := req.StudentID
	tenant.RegisterAfterCommit(ctx, func() {
		s.emitter.BroadcastChildUpdateToGuardians(tenantID, studentID)
	})
}

// --- apply (moved from services/messaging/requests.go) ---

// applyCareScheduleRequest sets the child's permanent weekly plan from the
// parent's request: per weekday a departure mode and/or arrival/pickup time.
// It MERGES onto the current plan — only the weekdays/aspects the parent
// filled in are changed, the rest are preserved — so a request that touches
// Friday never wipes Monday. The acting staff member (the confirmer) is
// stamped as CreatedBy on any schedule row it writes.
//
// Deliberately NO staleness guard (unlike the master-data review): the apply
// merges onto live data and the reviewer decides on a live diff, so an
// intervening direct edit is superseded rather than blocking the decision.
//
// It reports whether the apply actually CHANGED the child's "läuft mit" links,
// read from the write itself via users.CompanionChangeRecorder. Carrying
// departure modes is not the same question: a request that merges modes the
// child already has (or that only moves arrival/pickup times) leaves every link
// in place, and announcing a companion change for it makes open companion
// editors across the school discard or block their draft for nothing.
func (s *careScheduleRequestService) applyCareScheduleRequest(ctx context.Context, req *scheduleModels.CareScheduleChangeRequest) (bool, error) {
	if s.studentRepo == nil || s.arrival == nil || s.pickup == nil || s.userContext == nil {
		return false, errors.New("schedule: care request apply dependencies not configured")
	}
	// Re-validate and bucket the payload through the SAME parser the create
	// path uses, so the two paths cannot drift on weekday range, mode enum, or
	// time format.
	changes, err := buildCareScheduleChanges(req.Payload)
	if err != nil {
		return false, err
	}
	if changes.isEmpty() {
		return false, errors.New("schedule: no weekdays to apply")
	}

	// Every write below runs in this scope, so the repository can report whether
	// the departure-plan merge actually trimmed a "läuft mit" link.
	ctx, companionChanges := usersModels.ContextWithCompanionChangeRecorder(ctx)

	staff, err := s.userContext.GetCurrentStaff(ctx)
	if err != nil {
		// An account with no staff identity — e.g. a wildcard-admin account that
		// cleared the write check via admin:* but has no users.staff row —
		// cannot be stamped as the confirming staff member. That is an
		// authorization outcome (staff profile required), not a transient
		// failure, so surface it as 403 instead of a 500 that rolls the whole
		// request back.
		if errors.Is(err, userContextService.ErrUserNotLinkedToStaff) ||
			errors.Is(err, userContextService.ErrUserNotLinkedToPerson) {
			return false, ErrCareRequestForbidden
		}
		return false, fmt.Errorf("schedule: resolve acting staff: %w", err)
	}
	if staff == nil {
		return false, ErrCareRequestForbidden
	}
	staffID := staff.ID
	studentID := req.StudentID

	// Lock the student row up front (held until the tx commits) so two staff
	// approving requests for the same child serialize. The departure-mode
	// merge below is a read-modify-write on this row (AllowedDepartureModes),
	// so without the lock the second approve would read the same pre-change
	// snapshot and drop the first approve's mode change.
	student, err := s.studentRepo.FindByIDForUpdate(ctx, studentID)
	if err != nil {
		return false, err
	}

	if err := s.applyDepartureModeChanges(ctx, student, changes.modes); err != nil {
		return false, err
	}
	if err := s.applyArrivalChanges(ctx, studentID, staffID, changes.arrivals); err != nil {
		return false, err
	}
	if err := s.applyPickupChanges(ctx, studentID, staffID, changes.pickups); err != nil {
		return false, err
	}
	return companionChanges.Changed(), nil
}

// applyDepartureModeChanges merges the requested per-weekday departure modes
// onto the (already row-locked) student's AllowedDepartureModes, changing ONLY
// the touched weekdays. The parent request carries a single mode per day, so a
// touched day becomes exactly that one allowed mode; untouched days keep every
// allowed mode they had (multi-mode days like bus+pickup survive). The derived
// exclusive/legacy fields are cleared so the repository re-derives them from
// the merged allowed modes (the single source of truth).
func (s *careScheduleRequestService) applyDepartureModeChanges(ctx context.Context, student *usersModels.Student, changes map[string]usersModels.DepartureMode) error {
	if len(changes) == 0 {
		return nil
	}
	merged := usersModels.AllowedDepartureModes{}
	for day, modes := range student.AllowedDepartureModes {
		merged[day] = append([]usersModels.DepartureMode(nil), modes...)
	}
	for day, mode := range changes {
		merged[day] = []usersModels.DepartureMode{mode}
	}
	student.AllowedDepartureModes = merged
	student.DepartureDays = nil
	student.BusDays = nil
	student.PickupDays = nil
	// No Validate() here: the repository Update validates the merged plan anyway,
	// and only IT can see the second source of truth for the
	// accompanied-requires-a-"mit wem" rule — the structured companion links in
	// users.student_companions, which are not part of the model and are not
	// hydrated by FindByIDForUpdate. Validating here would refuse every approval
	// for a child whose accompanied day is answered by a link instead of the
	// free-text note, even when this request touches a different weekday (#1694).
	return s.studentRepo.Update(ctx, student)
}

// applyArrivalChanges upserts ONLY the weekdays this request changes, each via
// the per-(tenant, student, weekday) single-row upsert, so a concurrent direct
// edit to an untouched weekday is never clobbered by a snapshot reinsert.
func (s *careScheduleRequestService) applyArrivalChanges(ctx context.Context, studentID, staffID int64, changes map[int]string) error {
	for weekday, hhmm := range changes {
		// Do NOT discard the parse error: buildCareScheduleChanges validates the
		// same string on the create path, but a future caller reaching apply
		// without that pre-validation must still fail loudly rather than
		// silently overwrite the real arrival.
		t, err := parseCareWallClock(hhmm)
		if err != nil {
			return fmt.Errorf("apply arrival weekday %d: %w", weekday, err)
		}
		sched := &scheduleModels.StudentArrivalSchedule{
			StudentID:       studentID,
			Weekday:         weekday,
			ExpectedArrival: t,
			CreatedBy:       staffID,
		}
		if err := s.arrival.UpsertStudentArrivalSchedule(ctx, sched); err != nil {
			return fmt.Errorf("apply arrival weekday %d: %w", weekday, err)
		}
	}
	return nil
}

// applyPickupChanges mirrors applyArrivalChanges for pickup times.
func (s *careScheduleRequestService) applyPickupChanges(ctx context.Context, studentID, staffID int64, changes map[int]string) error {
	for weekday, hhmm := range changes {
		t, err := parseCareWallClock(hhmm)
		if err != nil {
			return fmt.Errorf("apply pickup weekday %d: %w", weekday, err)
		}
		sched := &scheduleModels.StudentPickupSchedule{
			StudentID:  studentID,
			Weekday:    weekday,
			PickupTime: t,
			CreatedBy:  staffID,
		}
		if err := s.pickup.UpsertStudentPickupSchedule(ctx, sched); err != nil {
			return fmt.Errorf("apply pickup weekday %d: %w", weekday, err)
		}
	}
	return nil
}

// broadcastCareScheduleChanges invalidates the staff-side caches an approved
// care-schedule change touches: the student master record (departure modes →
// student_updated, tenant-scoped) and the arrival/pickup schedules
// (arrival_schedule_changed, global — the event the student list/detail and
// "not arriving today" badges invalidate on). Registered after-commit so a
// woken client refetches the persisted plan, never the pre-commit snapshot.
// Fire-and-forget: a broadcast error only costs other tabs an auto-refresh.
func (s *careScheduleRequestService) broadcastCareScheduleChanges(ctx context.Context, tenantID, studentID int64, companionsChanged bool) {
	if s.broadcaster == nil {
		return
	}
	tenant.RegisterAfterCommit(ctx, func() {
		source := "parent_request"
		studentEvent := realtime.NewEvent(realtime.EventStudentUpdated, "", realtime.EventData{Source: &source})
		if err := s.broadcaster.BroadcastToTenant(tenantID, studentEvent); err != nil {
			s.logger.Warn("schedule: broadcast student_updated after care request approve failed",
				slog.Int64("tenant_id", tenantID),
				slog.Int64("student_id", studentID),
				slog.String("error", err.Error()),
			)
		}
		// Only when the write actually trimmed a "läuft mit" link — links that are
		// rows on ANOTHER child's card too. student_updated is not that signal:
		// companion views must not react to every student write, or an unrelated
		// edit discards someone's in-progress draft. A request that only moves
		// arrival or pickup times, or that re-merges the departure modes the child
		// already had, changes no link and stays silent here for the same reason.
		if companionsChanged {
			companionEvent := realtime.NewEvent(realtime.EventStudentCompanionsChanged, "", realtime.EventData{Source: &source})
			if err := s.broadcaster.BroadcastToTenant(tenantID, companionEvent); err != nil {
				s.logger.Warn("schedule: broadcast student_companions_changed after care request approve failed",
					slog.Int64("tenant_id", tenantID),
					slog.Int64("student_id", studentID),
					slog.String("error", err.Error()),
				)
			}
		}
		arrivalEvent := realtime.NewEvent(realtime.EventArrivalScheduleChanged, "", realtime.EventData{Source: &source})
		if err := s.broadcaster.BroadcastToAll(arrivalEvent); err != nil {
			s.logger.Warn("schedule: broadcast arrival_schedule_changed after care request approve failed",
				slog.Int64("tenant_id", tenantID),
				slog.Int64("student_id", studentID),
				slog.String("error", err.Error()),
			)
		}
	})
}

// recordApplyAudit writes the audit trail for an applied request. A durable
// per-child change-audit table does not exist yet (#1455 owns that); until
// then this is a structured, GDPR-safe slog record: actor, child, tenant and
// WHICH weekdays changed — never the values.
func (s *careScheduleRequestService) recordApplyAudit(req *scheduleModels.CareScheduleChangeRequest, accountID int64) {
	s.logger.Info("care request applied",
		slog.Int64("request_id", req.ID),
		slog.Int64("student_id", req.StudentID),
		slog.Int64("tenant_id", req.TenantID),
		slog.Int64("account_id", accountID),
		slog.String("changed", careScheduleAuditSummary(req.Payload)),
	)
}

// careScheduleAuditSummary lists the changed weekday numbers — keys only, no
// values — for the audit log.
func careScheduleAuditSummary(payload map[string]any) string {
	p, err := decodeCarePayload[careSchedulePayload](payload)
	if err != nil {
		return ""
	}
	days := make([]string, 0, len(p.Weekdays))
	for _, wd := range p.Weekdays {
		days = append(days, strconv.Itoa(wd.Weekday))
	}
	return "weekdays:" + strings.Join(days, ",")
}

// --- payload machinery (moved from services/messaging/requests.go) ---

// careWeekdayPayload is one weekday (1=Mon..5=Fri) of the parent's desired
// weekly plan. Every field is optional: an empty Mode/Arrival/Pickup means
// "leave that aspect of this weekday unchanged", so the apply merges the
// requested changes onto the child's current plan instead of replacing it.
type careWeekdayPayload struct {
	Weekday int    `json:"weekday"`
	Mode    string `json:"mode,omitempty"`    // alone|bus|pickup, "" = unchanged
	Arrival string `json:"arrival,omitempty"` // HH:MM, "" = unchanged
	Pickup  string `json:"pickup,omitempty"`  // HH:MM, "" = unchanged
}

type careSchedulePayload struct {
	Weekdays []careWeekdayPayload `json:"weekdays"`
}

// careScheduleChanges is the validated, bucketed form of a care-schedule
// payload: per-weekday departure modes (keyed by abbreviation), arrival times
// and pickup times (keyed by weekday number, both "HH:MM"). Produced by
// buildCareScheduleChanges and consumed by both the create-time validation and
// the approve-time apply.
type careScheduleChanges struct {
	modes    map[string]usersModels.DepartureMode
	arrivals map[int]string
	pickups  map[int]string
}

func (c careScheduleChanges) isEmpty() bool {
	return len(c.modes) == 0 && len(c.arrivals) == 0 && len(c.pickups) == 0
}

// toCanonicalPayload rebuilds the persistable payload from the bucketed
// changes: exactly one entry per weekday (1=Mon..5=Fri) that has a change, in
// fixed weekday order, carrying only the merged mode/arrival/pickup. This is
// what collapses a direct API client's duplicate or out-of-order weekday
// entries into the same form the apply path already derives.
func (c careScheduleChanges) toCanonicalPayload() careSchedulePayload {
	weekdays := make([]careWeekdayPayload, 0, len(usersModels.PickupDayOrder))
	for i, abbrev := range usersModels.PickupDayOrder {
		wd := i + 1
		entry := careWeekdayPayload{Weekday: wd}
		present := false
		if mode, ok := c.modes[abbrev]; ok {
			entry.Mode = string(mode)
			present = true
		}
		if arrival, ok := c.arrivals[wd]; ok {
			entry.Arrival = arrival
			present = true
		}
		if pickup, ok := c.pickups[wd]; ok {
			entry.Pickup = pickup
			present = true
		}
		if present {
			weekdays = append(weekdays, entry)
		}
	}
	return careSchedulePayload{Weekdays: weekdays}
}

func decodeCarePayload[T any](payload map[string]any) (T, error) {
	var out T
	raw, err := json.Marshal(payload)
	if err != nil {
		return out, err
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return out, err
	}
	return out, nil
}

// careStructToMap round-trips a typed payload struct through JSON into a
// generic map for JSONB storage, dropping any keys not declared on the struct.
func careStructToMap(v any) (map[string]any, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// parseCareWallClock turns an "HH:MM" string into the normalized wall-clock
// time.Time the schedule TIME columns store. Routing through timezone.WallClock
// is the mandated normalization for TIME columns (CLAUDE.md rule 11). Preserved
// rows carried through a merge must be re-normalized the same way before
// re-insert, since TIME columns scan back with a driver-chosen year that
// Postgres rejects.
func parseCareWallClock(hhmm string) (time.Time, error) {
	t, err := time.Parse("15:04", hhmm)
	if err != nil {
		return time.Time{}, err
	}
	return timezone.WallClock(t), nil
}

// buildCareScheduleChanges validates a care-schedule payload (weekday range,
// departure-mode enum, HH:MM format) and buckets it into the per-aspect change
// maps. It is the SINGLE parser shared by the create-time validation and the
// approve-time apply, so the two paths cannot drift. Validation failures wrap
// ErrInvalidCareRequestPayload; the apply path treats any non-nil error as a
// 500.
func buildCareScheduleChanges(payload map[string]any) (careScheduleChanges, error) {
	p, err := decodeCarePayload[careSchedulePayload](payload)
	if err != nil {
		return careScheduleChanges{}, ErrInvalidCareRequestPayload
	}
	out := careScheduleChanges{
		modes:    map[string]usersModels.DepartureMode{},
		arrivals: map[int]string{},
		pickups:  map[int]string{},
	}
	for _, wd := range p.Weekdays {
		if wd.Weekday < 1 || wd.Weekday > 5 {
			return careScheduleChanges{}, fmt.Errorf("%w: weekday %d", ErrInvalidCareRequestPayload, wd.Weekday)
		}
		abbrev := usersModels.PickupDayOrder[wd.Weekday-1]
		if wd.Mode != "" {
			mode := usersModels.DepartureMode(wd.Mode)
			switch mode {
			case usersModels.DepartureAlone, usersModels.DepartureBus, usersModels.DeparturePickup:
				out.modes[abbrev] = mode
			default:
				return careScheduleChanges{}, fmt.Errorf("%w: mode %q", ErrInvalidCareRequestPayload, wd.Mode)
			}
		}
		if wd.Arrival != "" {
			// Reject 00:00 as well as malformed times. parseCareWallClock
			// normalizes midnight to the zero time.Time, which the schedule
			// validators treat as "no time set" — so a stored 00:00 request
			// would pass create-time validation but fail on approve with a 500
			// staff cannot clear. Reject it here, on the single validator
			// shared by the create and apply paths.
			if t, err := parseCareWallClock(wd.Arrival); err != nil || t.IsZero() {
				return careScheduleChanges{}, fmt.Errorf("%w: arrival %q", ErrInvalidCareRequestPayload, wd.Arrival)
			}
			out.arrivals[wd.Weekday] = wd.Arrival
		}
		if wd.Pickup != "" {
			if t, err := parseCareWallClock(wd.Pickup); err != nil || t.IsZero() {
				return careScheduleChanges{}, fmt.Errorf("%w: pickup %q", ErrInvalidCareRequestPayload, wd.Pickup)
			}
			out.pickups[wd.Weekday] = wd.Pickup
		}
	}
	return out, nil
}

// canonicalizeCareSchedulePayload validates and returns the sanitized payload
// to persist. Buckets via the shared parser (which rejects bad weekdays/modes/
// times), then rebuilds one entry per changed weekday so unknown keys and
// duplicate weekday rows from a direct API client never reach storage.
func canonicalizeCareSchedulePayload(payload map[string]any) (map[string]any, error) {
	changes, err := buildCareScheduleChanges(payload)
	if err != nil {
		return nil, err
	}
	if changes.isEmpty() {
		return nil, fmt.Errorf("%w: no changes", ErrInvalidCareRequestPayload)
	}
	return careStructToMap(changes.toCanonicalPayload())
}

// --- diff (moved from services/messaging/requests.go) ---

// careDiffSource lazily loads and memoizes the student records a request diff
// compares against. Each getter loads its record at most once. Not safe to
// reuse across queue loads — the values must be live per load, because staff
// approve acts on them.
type careDiffSource struct {
	s         *careScheduleRequestService
	studentID int64

	student     *usersModels.Student
	studentErr  error
	studentDone bool

	arrivalMap  map[int]string
	arrivalDone bool

	pickupMap  map[int]string
	pickupDone bool
}

func (d *careDiffSource) getStudent(ctx context.Context) (*usersModels.Student, error) {
	if !d.studentDone {
		d.studentDone = true
		student, err := d.s.studentRepo.FindByID(ctx, d.studentID)
		if err != nil || student == nil {
			d.studentErr = fmt.Errorf("schedule: load student for diff: %w", err)
		} else {
			d.student = student
		}
	}
	return d.student, d.studentErr
}

// getArrival / getPickup are best-effort: a load failure yields an empty map
// so the diff shows "—" as the current value rather than failing the whole
// diff.
func (d *careDiffSource) getArrival(ctx context.Context) map[int]string {
	if !d.arrivalDone {
		d.arrivalDone = true
		d.arrivalMap = map[int]string{}
		if cur, err := d.s.arrival.GetStudentArrivalSchedules(ctx, d.studentID); err == nil {
			for _, a := range cur {
				d.arrivalMap[a.Weekday] = a.ExpectedArrival.Format("15:04")
			}
		}
	}
	return d.arrivalMap
}

func (d *careDiffSource) getPickup(ctx context.Context) map[int]string {
	if !d.pickupDone {
		d.pickupDone = true
		d.pickupMap = map[int]string{}
		if cur, err := d.s.pickup.GetStudentPickupSchedules(ctx, d.studentID); err == nil {
			for _, pc := range cur {
				d.pickupMap[pc.Weekday] = pc.PickupTime.Format("15:04")
			}
		}
	}
	return d.pickupMap
}

func (s *careScheduleRequestService) careScheduleDiffFrom(ctx context.Context, src *careDiffSource, payload map[string]any) ([]RequestDiffEntry, error) {
	p, err := decodeCarePayload[careSchedulePayload](payload)
	if err != nil {
		return nil, err
	}
	student, err := src.getStudent(ctx)
	if err != nil {
		return nil, err
	}

	arrivalMap := src.getArrival(ctx)
	pickupMap := src.getPickup(ctx)

	weekdays := append([]careWeekdayPayload(nil), p.Weekdays...)
	sort.Slice(weekdays, func(i, j int) bool { return weekdays[i].Weekday < weekdays[j].Weekday })

	var entries []RequestDiffEntry
	for _, wd := range weekdays {
		if wd.Weekday < 1 || wd.Weekday > 5 {
			continue
		}
		name := scheduleModels.WeekdayNames[wd.Weekday]
		abbrev := usersModels.PickupDayOrder[wd.Weekday-1]
		if wd.Mode != "" {
			entries = append(entries, RequestDiffEntry{
				Label: name + " · Abholart",
				// "Current" must read AllowedDepartureModes — the non-exclusive
				// field the apply path actually mutates — not the lossy
				// single-mode DepartureDays projection. Otherwise a day allowing
				// {bus, pickup} renders as one mode, a parent re-requesting that
				// mode looks like a no-op, and approving silently drops the
				// other allowed mode.
				Old:      germanAllowedDepartureModes(student.AllowedDepartureModes[abbrev]),
				New:      usersModels.DepartureMode(wd.Mode).GermanLabel(),
				Weekday:  wd.Weekday,
				CareKind: DiffCareKindDepartureMode,
				OldModes: departureModeKeys(student.AllowedDepartureModes[abbrev]),
				NewMode:  wd.Mode,
			})
		}
		if wd.Arrival != "" {
			entries = append(entries, RequestDiffEntry{
				Label:    name + " · Bringzeit",
				Old:      careDashIfEmpty(arrivalMap[wd.Weekday]),
				New:      wd.Arrival,
				Weekday:  wd.Weekday,
				CareKind: DiffCareKindArrival,
			})
		}
		if wd.Pickup != "" {
			entries = append(entries, RequestDiffEntry{
				Label:    name + " · Abholzeit",
				Old:      careDashIfEmpty(pickupMap[wd.Weekday]),
				New:      wd.Pickup,
				Weekday:  wd.Weekday,
				CareKind: DiffCareKindPickup,
			})
		}
	}
	return entries, nil
}

// germanAllowedDepartureModes renders a day's allowed departure modes (the
// non-exclusive source of truth the apply path writes) as a combined German
// label, e.g. "Fährt Bus / Wird abgeholt". An empty set means the child goes
// home alone.
func germanAllowedDepartureModes(modes []usersModels.DepartureMode) string {
	if len(modes) == 0 {
		return usersModels.DepartureAlone.GermanLabel()
	}
	parts := make([]string, 0, len(modes))
	for _, m := range modes {
		parts = append(parts, m.GermanLabel())
	}
	return strings.Join(parts, " / ")
}

// departureModeKeys returns the raw mode keys behind a day's allowed departure
// set, the structured counterpart to germanAllowedDepartureModes so a
// localized client can render the mode names in the guardian's language. An
// empty set means the child goes home alone, so it yields ["alone"] rather
// than an ambiguous empty slice.
func departureModeKeys(modes []usersModels.DepartureMode) []string {
	if len(modes) == 0 {
		return []string{string(usersModels.DepartureAlone)}
	}
	keys := make([]string, 0, len(modes))
	for _, m := range modes {
		keys = append(keys, string(m))
	}
	return keys
}

func careDashIfEmpty(s string) string {
	if strings.TrimSpace(s) == "" {
		return "—"
	}
	return s
}

// isCareRequestPendingUniqueViolation detects the partial-unique-index
// violation (one open request per student) behind the repository's
// DatabaseError wrapper.
func isCareRequestPendingUniqueViolation(err error) bool {
	return modelBase.IsUniqueViolationOn(err, careRequestPendingUniqueIndex)
}
