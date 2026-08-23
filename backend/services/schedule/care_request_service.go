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
	"crypto/sha256"
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
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/realtime"
	"github.com/moto-nrw/project-phoenix/services/parentmessaging"
	userContextService "github.com/moto-nrw/project-phoenix/services/usercontext"
	usersService "github.com/moto-nrw/project-phoenix/services/users"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// careRequestMaxReasonLen bounds the staff reject reason (in runes) so a
// direct API client cannot persist an unbounded payload. Matches the
// message-body limit the chat used for the same field.
const careRequestMaxReasonLen = 2000

// careRequestPendingUniqueIndex is the partial unique index enforcing one
// open weekly-schedule request per student;
// pickupChangePendingUniqueIndex enforces one open pickup-change request per
// student and requested day (payload date). A violation of either maps to
// ErrCareRequestAlreadyPending.
const (
	careRequestPendingUniqueIndex  = "uniq_care_schedule_change_requests_pending"
	pickupChangePendingUniqueIndex = "uniq_care_schedule_change_requests_pending_pickup_date"
)

// German pill texts. The staff portal renders these directly; the parents
// portal localizes from the structured event fields instead.
const (
	careRequestCreatedBody     = "Anfrage: Dauerhafte Betreuungszeiten ändern"
	careRequestConfirmedBody   = "Anfrage bestätigt, Betreuungszeiten übernommen"
	careRequestWithdrawnBody   = "Anfrage zurückgezogen"
	pickupRequestCreatedBody   = "Anfrage: Abholzeit ändern"
	pickupRequestConfirmedBody = "Abholzeit bestätigt"
)

// careRequestPillType maps a request kind to the pill's request_type token.
// The German bodies above are authoritative only for the staff portal; the
// localized parents portal and the decision push render from this token, so a
// one-day pickup change must not travel as a permanent care_schedule change.
func careRequestPillType(requestKind string) string {
	if requestKind == scheduleModels.CareRequestKindPickupChange {
		return usersModels.ParentMessageRequestPickupChange
	}
	return usersModels.ParentMessageRequestCareSchedule
}

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
	// ErrCareRequestRejectReasonRequired means staff rejected without a reason.
	ErrCareRequestRejectReasonRequired = errors.New("schedule: reject reason is required")
	// ErrCareRequestRejectReasonTooLong means the reason exceeded the bound.
	ErrCareRequestRejectReasonTooLong = errors.New("schedule: reject reason too long")
	// ErrCareDayManagedByBooking prevents a permanent care-day request from
	// pretending to remove a day whose pickup baseline still comes from an
	// active offering booking. The booking must be changed first (#2416).
	ErrCareDayManagedByBooking      = errors.New("schedule: care day is managed by an offering booking")
	ErrPickupChangeConflict         = errors.New("schedule: pickup change conflicts with a staff exception")
	ErrPickupChangeAlreadyCompleted = errors.New("schedule: pickup change cannot be approved after checkout")
	// ErrPickupChangeExpired means the requested day has passed while the
	// request sat in the queue. Approving would write an exception for a day
	// that is over; staff close such a request by rejecting it.
	ErrPickupChangeExpired        = errors.New("schedule: pickup change date has passed")
	ErrPickupChangeImpactChanged  = errors.New("schedule: pickup change affected blocks changed")
	ErrPickupChangeImpactRequired = errors.New("schedule: pickup change impact token is required")
)

// Diff care-kind discriminators (see RequestDiffEntry.CareKind). Stable wire
// tokens — the localized parents portal maps them to its own labels.
const (
	DiffCareKindArrival       = "arrival"
	DiffCareKindPickup        = "pickup"
	DiffCareKindDepartureMode = "departure_mode"
	DiffCareKindScheduled     = "scheduled"
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
	Request         *scheduleModels.CareScheduleChangeRequest
	FirstName       string
	LastName        string
	Diff            []RequestDiffEntry
	Reason          *string
	AffectedBlocks  []scheduleModels.PartialAbsenceBlock
	ImpactAvailable bool
	ImpactToken     string
}

// CareRequestHistoryItem is one decided request enriched with the child's
// name, the reviewer's display name, and the payload-derived "requested"
// summary. There is NO live "current → requested" diff: current data has
// moved on since the decision, so re-computing it would show a comparison
// that never existed. Diff instead replays the frozen decision snapshot
// (ADR 0002, #2430) when the row carries one; rows decided before the
// snapshot existed (and withdrawals) leave it nil and readers fall back to
// the stable payload-derived Requested summary.
type CareRequestHistoryItem struct {
	Request      *scheduleModels.CareScheduleChangeRequest
	FirstName    string
	LastName     string
	ReviewerName string // "" when the row carries no reviewer (withdrawn)
	Requested    []RequestDiffEntry
	Diff         []RequestDiffEntry // frozen alt → neu, nil without a snapshot
}

// CareRequestDecideInput carries a staff decision on one pending request.
type CareRequestDecideInput struct {
	RequestID int64
	Approve   bool
	Reason    string
	// ReviewedBy is the acting staff ACCOUNT id (auth.accounts), stamped as
	// reviewed_by and used as the pill's actor.
	ReviewedBy int64
	// ExpectedImpactToken pins the staff-visible pickup impact. A nil
	// pointer is reserved for trusted internal callers.
	ExpectedImpactToken *string
	// RequireImpactToken distinguishes untrusted HTTP approvals from internal
	// service callers. It is enforced only for pickup-change requests.
	RequireImpactToken bool
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
	WithdrawPickupChangeRequest(ctx context.Context, requestID, studentID, guardianAccountID int64) (*scheduleModels.CareScheduleChangeRequest, error)
	// GetPendingForStudent returns the child's open request (nil when none)
	// with its live "current → requested" diff, for the parent read view.
	GetPendingForStudent(ctx context.Context, studentID int64) (*scheduleModels.CareScheduleChangeRequest, []RequestDiffEntry, error)
	// ListPending returns pending requests of both kinds for the current
	// tenant, newest submission first, enriched with child names and live
	// diffs — the working list of the Anfragen module. The filters narrow and
	// page the query in SQL; their zero value returns the whole queue.
	ListPending(ctx context.Context, filters modelBase.RequestQueueFilters) (items []*CareRequestReviewItem, next *usersService.HistoryCursor, err error)
	// ListHistory returns decided care-schedule requests
	// newest-decision-first, keyset paginated on (updated_at, id). A zero
	// BeforeInstant returns the first page; next is nil when no older rows
	// exist beyond this page.
	ListHistory(ctx context.Context, filters modelBase.RequestQueueFilters) (items []*CareRequestHistoryItem, next *usersService.HistoryCursor, err error)
	CreatePickupChangeRequest(ctx context.Context, studentID, guardianAccountID int64, date timezone.Date, pickupTime time.Time, reason string) (*scheduleModels.CareScheduleChangeRequest, error)
	ListPendingPickupChanges(ctx context.Context) ([]*CareRequestReviewItem, error)
	ListPickupChangeRequests(ctx context.Context, studentID int64, since time.Time) ([]*scheduleModels.CareScheduleChangeRequest, error)
	// Decide approves (applies the weekly plan, then stamps) or rejects
	// (reason required) one pending request and returns the refreshed row.
	// After commit it posts the decision pill and fires the schedule cache
	// invalidations.
	Decide(ctx context.Context, input CareRequestDecideInput) (*CareRequestReviewItem, error)
}

type careScheduleRequestService struct {
	requestRepo       scheduleModels.CareScheduleChangeRequestRepository
	studentRepo       usersModels.StudentRepository
	personRepo        usersModels.PersonRepository
	arrival           ArrivalScheduleService
	pickup            PickupScheduleService
	pickupExceptions  scheduleModels.StudentPickupExceptionRepository
	attendance        activeModels.AttendanceRepository
	pickupAutoExcusal *PickupAutoExcusalSyncer
	userContext       userContextService.UserContextService
	emitter           *parentmessaging.Emitter
	broadcaster       realtime.Broadcaster
	studentAudit      usersService.StudentChangeRecorder
	logger            *slog.Logger
}

// NewCareScheduleRequestServiceWithPickupChanges wires one-day pickup requests
// in addition to the recurring weekly schedule requests.
func NewCareScheduleRequestServiceWithPickupChanges(
	requestRepo scheduleModels.CareScheduleChangeRequestRepository,
	studentRepo usersModels.StudentRepository,
	personRepo usersModels.PersonRepository,
	arrival ArrivalScheduleService,
	pickup PickupScheduleService,
	pickupExceptions scheduleModels.StudentPickupExceptionRepository,
	attendance activeModels.AttendanceRepository,
	pickupAutoExcusal *PickupAutoExcusalSyncer,
	userContext userContextService.UserContextService,
	emitter *parentmessaging.Emitter,
	broadcaster realtime.Broadcaster,
	logger *slog.Logger,
	studentAudits ...usersService.StudentChangeRecorder,
) CareScheduleRequestService {
	svc := NewCareScheduleRequestService(
		requestRepo,
		studentRepo,
		personRepo,
		arrival,
		pickup,
		userContext,
		emitter,
		broadcaster,
		logger,
		studentAudits...,
	)
	svc.(*careScheduleRequestService).pickupExceptions = pickupExceptions
	svc.(*careScheduleRequestService).attendance = attendance
	svc.(*careScheduleRequestService).pickupAutoExcusal = pickupAutoExcusal
	return svc
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
	studentAudits ...usersService.StudentChangeRecorder,
) CareScheduleRequestService {
	if logger == nil {
		logger = slog.Default()
	}
	var studentAudit usersService.StudentChangeRecorder
	if len(studentAudits) > 0 {
		studentAudit = studentAudits[0]
	}
	return &careScheduleRequestService{
		requestRepo:  requestRepo,
		studentRepo:  studentRepo,
		personRepo:   personRepo,
		arrival:      arrival,
		pickup:       pickup,
		userContext:  userContext,
		emitter:      emitter,
		broadcaster:  broadcaster,
		studentAudit: studentAudit,
		logger:       logger,
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
		RequestKind: scheduleModels.CareRequestKindWeeklySchedule,
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

func (s *careScheduleRequestService) CreatePickupChangeRequest(ctx context.Context, studentID, guardianAccountID int64, date timezone.Date, pickupTime time.Time, reason string) (*scheduleModels.CareScheduleChangeRequest, error) {
	reason = strings.TrimSpace(reason)
	if studentID <= 0 || guardianAccountID <= 0 || date.IsZero() || pickupTime.IsZero() || reason == "" || utf8.RuneCountInString(reason) > 255 {
		return nil, ErrInvalidCareRequestPayload
	}
	today := timezone.TodayDate()
	if date.Before(today) || date.After(timezone.NewDate(today.Year, today.Month+2, today.Day)) {
		return nil, ErrInvalidCareRequestPayload
	}

	payload := map[string]any{
		"date":        date.String(),
		"pickup_time": pickupTime.Format("15:04"),
		"reason":      reason,
	}
	if s.pickup != nil {
		effective, err := s.pickup.GetEffectivePickupTimeForDate(ctx, studentID, date)
		if err != nil {
			return nil, fmt.Errorf("schedule: resolve current pickup time: %w", err)
		}
		if effective != nil && effective.PickupTime != nil {
			payload["previous_pickup_time"] = effective.PickupTime.Format("15:04")
		}
	}

	req := &scheduleModels.CareScheduleChangeRequest{
		StudentID:   studentID,
		SubmittedBy: guardianAccountID,
		RequestKind: scheduleModels.CareRequestKindPickupChange,
		Payload:     payload,
		Status:      scheduleModels.CareRequestStatusPending,
	}
	if err := s.requestRepo.Create(ctx, req); err != nil {
		if isCareRequestPendingUniqueViolation(err) {
			return nil, ErrCareRequestAlreadyPending
		}
		return nil, fmt.Errorf("schedule: create pickup change request: %w", err)
	}
	s.emitRequestPillAfterCommit(ctx, req, parentmessaging.ChildEvent{
		EventType:      "request_created",
		ActorKind:      usersModels.ParentMessageSenderGuardian,
		ActorAccountID: guardianAccountID,
		Body:           pickupRequestCreatedBody,
		RequestType:    usersModels.ParentMessageRequestPickupChange,
		RequestStatus:  usersModels.ParentMessageRequestStatusOpen,
	})
	s.wakeGuardiansAfterCommit(ctx, req)
	return req, nil
}

func (s *careScheduleRequestService) WithdrawRequest(ctx context.Context, requestID, studentID, guardianAccountID int64) (*scheduleModels.CareScheduleChangeRequest, error) {
	return s.withdrawRequestOfKind(ctx, requestID, studentID, guardianAccountID, scheduleModels.CareRequestKindWeeklySchedule)
}

func (s *careScheduleRequestService) WithdrawPickupChangeRequest(ctx context.Context, requestID, studentID, guardianAccountID int64) (*scheduleModels.CareScheduleChangeRequest, error) {
	return s.withdrawRequestOfKind(ctx, requestID, studentID, guardianAccountID, scheduleModels.CareRequestKindPickupChange)
}

func (s *careScheduleRequestService) withdrawRequestOfKind(ctx context.Context, requestID, studentID, guardianAccountID int64, requestKind string) (*scheduleModels.CareScheduleChangeRequest, error) {
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
	if req.SubmittedBy != guardianAccountID || req.StudentID != studentID || req.RequestKind != requestKind {
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
		RequestType:    careRequestPillType(req.RequestKind),
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

func (s *careScheduleRequestService) ListPending(ctx context.Context, filters modelBase.RequestQueueFilters) ([]*CareRequestReviewItem, *usersService.HistoryCursor, error) {
	// limit+1 probes for an older page without a second count query.
	rows, err := s.requestRepo.ListPendingForTenant(ctx, probeLimit(filters))
	if err != nil {
		return nil, nil, fmt.Errorf("schedule: list pending care requests: %w", err)
	}
	rows, next := usersService.NextCursor(rows, filters.Limit, func(r *scheduleModels.CareScheduleChangeRequest) (time.Time, int64) {
		return r.CreatedAt, r.ID
	})
	items, err := s.buildPendingItems(ctx, rows)
	if err != nil {
		return nil, nil, err
	}
	return items, next, nil
}

// probeLimit asks the repository for one row more than the caller wants, so a
// present extra row proves an older page exists. An unbounded page stays
// unbounded.
func probeLimit(filters modelBase.RequestQueueFilters) modelBase.RequestQueueFilters {
	if filters.Limit > 0 {
		filters.Limit++
	}
	return filters
}

func (s *careScheduleRequestService) ListHistory(ctx context.Context, filters modelBase.RequestQueueFilters) ([]*CareRequestHistoryItem, *usersService.HistoryCursor, error) {
	// limit+1 probes for an older page without a second count query.
	rows, err := s.requestRepo.ListDecidedForTenant(ctx, probeLimit(filters))
	if err != nil {
		return nil, nil, fmt.Errorf("schedule: list decided care requests: %w", err)
	}
	// The cursor points at the last DB row (not the last visible item): the
	// per-child scope filters after the DB limit, so a cursor built from the
	// filtered page would skip rows.
	rows, next := usersService.NextCursor(rows, filters.Limit, func(r *scheduleModels.CareScheduleChangeRequest) (time.Time, int64) {
		return r.UpdatedAt, r.ID
	})
	if len(rows) == 0 {
		return []*CareRequestHistoryItem{}, next, nil
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
	students, err := s.studentRepo.FindByIDs(ctx, studentIDs)
	if err != nil {
		return nil, nil, fmt.Errorf("schedule: load students for care request history: %w", err)
	}
	personIDs := make([]int64, 0, len(students))
	for _, st := range students {
		personIDs = append(personIDs, st.PersonID)
	}
	persons, err := s.personRepo.FindByIDs(ctx, personIDs)
	if err != nil {
		return nil, nil, fmt.Errorf("schedule: load persons for care request history: %w", err)
	}
	reviewers, err := s.personRepo.FindByAccountIDs(ctx, reviewerIDs)
	if err != nil {
		return nil, nil, fmt.Errorf("schedule: load reviewers for care request history: %w", err)
	}

	// Same per-child scope as ListPending: write gate + alumnus skip, so the
	// history shows exactly the children the caller may act on.
	writable := authorize.WritableStudentFilter(ctx, jwt.PermissionsFromCtx(ctx), s.userContext)

	items := make([]*CareRequestHistoryItem, 0, len(rows))
	for _, r := range rows {
		st := students[r.StudentID]
		if !writable(st) || st.IsAlumnus() {
			continue
		}
		item := &CareRequestHistoryItem{
			Request:      r,
			ReviewerName: usersService.ReviewerDisplayName(reviewers, r.ReviewedBy),
			Requested:    careRequestedSummaryFrom(r.Payload),
			Diff:         careDiffEntriesFromSnapshot(r.DecisionSnapshot),
		}
		if p, ok := persons[st.PersonID]; ok {
			item.FirstName = p.FirstName
			item.LastName = p.LastName
		}
		items = append(items, item)
	}
	return items, next, nil
}

// careRequestedSummaryFrom renders the REQUESTED side of a decided
// weekly-schedule or pickup-change payload, with no live reads: the "current" side has moved on
// since the decision, so unlike careScheduleDiffFrom the Old fields stay empty.
// An undecodable payload yields an empty summary (the row still lists).
func careRequestedSummaryFrom(payload map[string]any) []RequestDiffEntry {
	if date, pickup, _, err := parsePickupChangePayload(payload); err == nil {
		return []RequestDiffEntry{{
			Label:    date.Format(germanDateLayout) + " · Abholzeit",
			New:      pickup.Format("15:04"),
			CareKind: DiffCareKindPickup,
		}}
	}
	p, err := decodeCarePayload[careSchedulePayload](payload)
	if err != nil {
		return nil
	}
	weekdays := append([]careWeekdayPayload(nil), p.Weekdays...)
	sort.Slice(weekdays, func(i, j int) bool { return weekdays[i].Weekday < weekdays[j].Weekday })

	var entries []RequestDiffEntry
	for _, wd := range weekdays {
		if wd.Weekday < 1 || wd.Weekday > 5 {
			continue
		}
		name := scheduleModels.WeekdayNames[wd.Weekday]
		if wd.Scheduled != nil {
			entries = append(entries, RequestDiffEntry{
				Label:    name + " · Betreuungstag",
				New:      careDayGermanLabel(*wd.Scheduled),
				Weekday:  wd.Weekday,
				CareKind: DiffCareKindScheduled,
			})
		}
		if wd.Mode != "" {
			entries = append(entries, RequestDiffEntry{
				Label:    name + " · Abholart",
				New:      usersModels.DepartureMode(wd.Mode).GermanLabel(),
				Weekday:  wd.Weekday,
				CareKind: DiffCareKindDepartureMode,
				NewMode:  wd.Mode,
			})
		}
		if wd.Arrival != "" {
			entries = append(entries, RequestDiffEntry{
				Label:    name + " · Bringzeit",
				New:      wd.Arrival,
				Weekday:  wd.Weekday,
				CareKind: DiffCareKindArrival,
			})
		}
		if wd.Pickup != "" {
			entries = append(entries, RequestDiffEntry{
				Label:    name + " · Abholzeit",
				New:      wd.Pickup,
				Weekday:  wd.Weekday,
				CareKind: DiffCareKindPickup,
			})
		}
	}
	return entries
}

// buildDecisionSnapshot materializes the alt → neu diff of a still-pending
// request into its frozen snapshot form (ADR 0002, #2430). Returns nil when
// the diff cannot be built — the decision must never hang on presentation.
func (s *careScheduleRequestService) buildDecisionSnapshot(ctx context.Context, req *scheduleModels.CareScheduleChangeRequest) *scheduleModels.CareRequestDecisionSnapshot {
	var diff []RequestDiffEntry
	var err error
	if req.RequestKind == scheduleModels.CareRequestKindPickupChange {
		diff, err = s.pickupChangeDiff(ctx, req)
	} else {
		diff, err = s.careScheduleDiffFrom(ctx, &careDiffSource{s: s, studentID: req.StudentID}, req.Payload)
	}
	if err != nil {
		s.logger.Warn("schedule: build care request decision snapshot failed",
			slog.Int64("request_id", req.ID),
			slog.String("error", err.Error()),
		)
		return nil
	}
	snapshot := &scheduleModels.CareRequestDecisionSnapshot{
		Diff: make([]scheduleModels.CareRequestSnapshotEntry, 0, len(diff)),
	}
	for _, e := range diff {
		snapshot.Diff = append(snapshot.Diff, scheduleModels.CareRequestSnapshotEntry{
			Label:    e.Label,
			Old:      e.Old,
			New:      e.New,
			Weekday:  e.Weekday,
			CareKind: e.CareKind,
			OldModes: e.OldModes,
			NewMode:  e.NewMode,
		})
	}
	return snapshot
}

// careDiffEntriesFromSnapshot maps a frozen decision snapshot back into the
// service diff shape shared with the pending queue. Nil in, nil out.
func careDiffEntriesFromSnapshot(snapshot *scheduleModels.CareRequestDecisionSnapshot) []RequestDiffEntry {
	if snapshot == nil {
		return nil
	}
	out := make([]RequestDiffEntry, 0, len(snapshot.Diff))
	for _, e := range snapshot.Diff {
		out = append(out, RequestDiffEntry{
			Label:    e.Label,
			Old:      e.Old,
			New:      e.New,
			Weekday:  e.Weekday,
			CareKind: e.CareKind,
			OldModes: e.OldModes,
			NewMode:  e.NewMode,
		})
	}
	return out
}

func (s *careScheduleRequestService) ListPendingPickupChanges(ctx context.Context) ([]*CareRequestReviewItem, error) {
	rows, err := s.requestRepo.ListPendingForTenantAndKind(ctx, scheduleModels.CareRequestKindPickupChange, modelBase.RequestQueueFilters{})
	if err != nil {
		return nil, fmt.Errorf("schedule: list pending pickup change requests: %w", err)
	}
	return s.buildPendingItems(ctx, rows)
}

func (s *careScheduleRequestService) ListPickupChangeRequests(ctx context.Context, studentID int64, since time.Time) ([]*scheduleModels.CareScheduleChangeRequest, error) {
	return s.requestRepo.ListRecentForStudentAndKind(ctx, studentID, scheduleModels.CareRequestKindPickupChange, since)
}

func (s *careScheduleRequestService) buildPendingItems(ctx context.Context, rows []*scheduleModels.CareScheduleChangeRequest) ([]*CareRequestReviewItem, error) {
	if len(rows) == 0 {
		return []*CareRequestReviewItem{}, nil
	}
	students, persons, err := s.loadPendingPeople(ctx, rows)
	if err != nil {
		return nil, err
	}
	writable := authorize.WritableStudentFilter(ctx, jwt.PermissionsFromCtx(ctx), s.userContext)
	items := make([]*CareRequestReviewItem, 0, len(rows))
	sources := map[int64]*careDiffSource{}
	for _, r := range rows {
		student := students[r.StudentID]
		// A child whose care has ended leaves the pending queue: the
		// effect-day pass closes their open requests, and until it runs the
		// queue must not offer a decision on a departed child (#2487).
		if !writable(student) || student.IsAlumnus() || student.CareEndedOn(timezone.TodayDate()) {
			continue
		}
		items = append(items, s.buildPendingItem(ctx, r, student, persons, sources))
	}
	return items, nil
}

func (s *careScheduleRequestService) loadPendingPeople(
	ctx context.Context, rows []*scheduleModels.CareScheduleChangeRequest,
) (map[int64]*usersModels.Student, map[int64]*usersModels.Person, error) {
	studentIDs := make([]int64, 0, len(rows))
	seen := make(map[int64]struct{}, len(rows))
	for _, row := range rows {
		if _, ok := seen[row.StudentID]; !ok {
			seen[row.StudentID] = struct{}{}
			studentIDs = append(studentIDs, row.StudentID)
		}
	}
	students, err := s.studentRepo.FindByIDs(ctx, studentIDs)
	if err != nil {
		return nil, nil, fmt.Errorf("schedule: load students for care requests: %w", err)
	}
	personIDs := make([]int64, 0, len(students))
	for _, student := range students {
		personIDs = append(personIDs, student.PersonID)
	}
	persons, err := s.personRepo.FindByIDs(ctx, personIDs)
	if err != nil {
		return nil, nil, fmt.Errorf("schedule: load persons for care requests: %w", err)
	}
	return students, persons, nil
}

func (s *careScheduleRequestService) buildPendingItem(
	ctx context.Context,
	request *scheduleModels.CareScheduleChangeRequest,
	student *usersModels.Student,
	persons map[int64]*usersModels.Person,
	sources map[int64]*careDiffSource,
) *CareRequestReviewItem {
	item := &CareRequestReviewItem{Request: request}
	if person := persons[student.PersonID]; person != nil {
		item.FirstName = person.FirstName
		item.LastName = person.LastName
	}
	diff, err := s.pendingRequestDiff(ctx, request, item, sources)
	if err == nil {
		item.Diff = diff
		return item
	}
	s.logger.Warn("schedule: build care request diff failed",
		slog.Int64("request_id", request.ID),
		slog.String("error", err.Error()),
	)
	item.Diff = careRequestedSummaryFrom(request.Payload)
	return item
}

func (s *careScheduleRequestService) pendingRequestDiff(
	ctx context.Context,
	request *scheduleModels.CareScheduleChangeRequest,
	item *CareRequestReviewItem,
	sources map[int64]*careDiffSource,
) ([]RequestDiffEntry, error) {
	if request.RequestKind == scheduleModels.CareRequestKindPickupChange {
		item.Reason = pickupChangeReason(request)
		item.AffectedBlocks, item.ImpactAvailable = s.previewPickupChangeBlocks(ctx, request)
		if item.ImpactAvailable {
			item.ImpactToken = pickupImpactToken(item.AffectedBlocks)
		}
		return s.pickupChangeDiff(ctx, request)
	}
	source := sources[request.StudentID]
	if source == nil {
		source = &careDiffSource{s: s, studentID: request.StudentID}
		sources[request.StudentID] = source
	}
	return s.careScheduleDiffFrom(ctx, source, request.Payload)
}

func (s *careScheduleRequestService) previewPickupChangeBlocks(
	ctx context.Context, req *scheduleModels.CareScheduleChangeRequest,
) ([]scheduleModels.PartialAbsenceBlock, bool) {
	if s.pickupAutoExcusal == nil {
		return nil, false
	}
	date, pickupTime, _, err := parsePickupChangePayload(req.Payload)
	if err != nil {
		return nil, false
	}
	blocks, err := s.pickupAutoExcusal.Preview(ctx, req.StudentID, date, pickupTime)
	if err != nil {
		s.logger.Warn("schedule: preview pickup change blocks failed",
			slog.Int64("request_id", req.ID),
			slog.String("error", err.Error()),
		)
		return nil, false
	}
	return blocks, true
}

func (s *careScheduleRequestService) pickupChangeDiff(ctx context.Context, req *scheduleModels.CareScheduleChangeRequest) ([]RequestDiffEntry, error) {
	date, pickupTime, _, err := parsePickupChangePayload(req.Payload)
	if err != nil {
		return nil, err
	}
	old, _ := req.Payload["previous_pickup_time"].(string)
	if old == "" && s.pickupExceptions != nil {
		existing, findErr := s.pickupExceptions.FindByStudentIDAndDate(ctx, req.StudentID, date)
		if findErr != nil {
			return nil, findErr
		}
		if existing != nil && existing.PickupTime != nil {
			old = existing.PickupTime.Format("15:04")
		}
	}
	if old == "" && s.pickup != nil {
		effective, findErr := s.pickup.GetEffectivePickupTimeForDate(ctx, req.StudentID, date)
		if findErr != nil {
			return nil, findErr
		}
		if effective.PickupTime != nil {
			old = effective.PickupTime.Format("15:04")
		}
	}
	return []RequestDiffEntry{{
		Label:    date.Format(germanDateLayout) + " · Abholzeit",
		Old:      old,
		New:      pickupTime.Format("15:04"),
		CareKind: DiffCareKindPickup,
	}}, nil
}

func (s *careScheduleRequestService) Decide(ctx context.Context, input CareRequestDecideInput) (*CareRequestReviewItem, error) {
	reason, err := validateCareRequestDecision(input)
	if err != nil {
		return nil, err
	}
	req, err := s.loadAuthorizedCareDecision(ctx, input.RequestID)
	if err != nil {
		return nil, err
	}
	snapshot := s.buildDecisionSnapshot(ctx, req)
	companionsChanged, err := s.applyApprovedCareDecision(ctx, req, input)
	if err != nil {
		return nil, err
	}
	state := newCareDecisionState(req, input.Approve, reason)
	if err := s.persistCareDecision(ctx, req, input, state, snapshot); err != nil {
		return nil, err
	}
	s.registerCareDecisionEffects(ctx, req, input, state, companionsChanged)
	return s.reloadCareDecisionItem(ctx, req.ID)
}

func validateCareRequestDecision(input CareRequestDecideInput) (string, error) {
	if input.RequestID <= 0 {
		return "", scheduleModels.ErrCareRequestNotFound
	}
	reason := strings.TrimSpace(input.Reason)
	if input.Approve {
		return reason, nil
	}
	if reason == "" {
		return "", ErrCareRequestRejectReasonRequired
	}
	if utf8.RuneCountInString(reason) > careRequestMaxReasonLen {
		return "", ErrCareRequestRejectReasonTooLong
	}
	return reason, nil
}

func (s *careScheduleRequestService) loadAuthorizedCareDecision(ctx context.Context, requestID int64) (*scheduleModels.CareScheduleChangeRequest, error) {
	req, err := s.requestRepo.FindPendingByIDForUpdate(ctx, requestID)
	if err != nil {
		return nil, err
	}
	student, err := s.studentRepo.FindByIDForUpdate(ctx, req.StudentID)
	if err != nil {
		return nil, fmt.Errorf("schedule: load student for care request decision: %w", err)
	}
	if student.IsAlumnus() {
		return nil, scheduleModels.ErrCareRequestNotFound
	}
	// The child left the OGS after filing this request; approving it would
	// write a weekly plan nobody will follow (#2487).
	if student.CareEndedOn(timezone.TodayDate()) {
		return nil, scheduleModels.ErrCareRequestNotFound
	}
	if ok, _ := authorize.CanUpdateStudent(ctx, jwt.PermissionsFromCtx(ctx), student, s.userContext); !ok {
		return nil, ErrCareRequestForbidden
	}
	return req, nil
}

func (s *careScheduleRequestService) applyApprovedCareDecision(ctx context.Context, req *scheduleModels.CareScheduleChangeRequest, input CareRequestDecideInput) (bool, error) {
	if !input.Approve {
		return false, nil
	}
	if s.emitter != nil {
		hasAccess, err := s.emitter.GuardianHasChildAccess(ctx, req.StudentID, req.SubmittedBy)
		if err != nil {
			return false, fmt.Errorf("schedule: care request guardian link check: %w", err)
		}
		if !hasAccess {
			return false, ErrCareRequestGuardianAccessRevoked
		}
	}
	if req.RequestKind == scheduleModels.CareRequestKindPickupChange {
		err := s.applyPickupChangeRequest(ctx, req, input.ExpectedImpactToken, input.RequireImpactToken)
		return false, err
	}
	return s.applyCareScheduleRequest(ctx, req, input.ReviewedBy)
}

type careDecisionState struct {
	status     string
	pillStatus string
	pillBody   string
	reason     string
	reasonPtr  *string
}

func newCareDecisionState(req *scheduleModels.CareScheduleChangeRequest, approve bool, reason string) careDecisionState {
	body := careRequestConfirmedBody
	if req.RequestKind == scheduleModels.CareRequestKindPickupChange {
		body = pickupRequestConfirmedBody
	}
	if approve {
		return careDecisionState{scheduleModels.CareRequestStatusApproved, usersModels.ParentMessageRequestStatusDone, body, reason, nil}
	}
	return careDecisionState{scheduleModels.CareRequestStatusRejected, usersModels.ParentMessageRequestStatusRejected, "Anfrage abgelehnt: " + reason, reason, &reason}
}

func (s *careScheduleRequestService) persistCareDecision(ctx context.Context, req *scheduleModels.CareScheduleChangeRequest, input CareRequestDecideInput, state careDecisionState, snapshot *scheduleModels.CareRequestDecisionSnapshot) error {
	reviewedBy := input.ReviewedBy
	if err := s.requestRepo.Decide(ctx, req.ID, state.status, state.reasonPtr, &reviewedBy, input.Approve); err != nil {
		return err
	}
	if snapshot == nil {
		return nil
	}
	if err := s.requestRepo.UpdateDecisionSnapshot(ctx, req.ID, snapshot); err != nil {
		return fmt.Errorf("schedule: store care request decision snapshot: %w", err)
	}
	return nil
}

func (s *careScheduleRequestService) registerCareDecisionEffects(ctx context.Context, req *scheduleModels.CareScheduleChangeRequest, input CareRequestDecideInput, state careDecisionState, companionsChanged bool) {
	if input.Approve {
		tenant.RegisterAfterCommit(ctx, func() { s.recordApplyAudit(req, input.ReviewedBy) })
		s.broadcastCareScheduleChanges(ctx, req.TenantID, req.StudentID, companionsChanged)
	} else {
		tenant.RegisterAfterCommit(ctx, func() {
			s.logger.Info("care request rejected",
				"request_id", req.ID,
				"student_id", req.StudentID,
				"tenant_id", req.TenantID,
				"reviewed_by", input.ReviewedBy,
			)
		})
	}
	s.emitRequestPillAfterCommit(ctx, req, parentmessaging.ChildEvent{
		EventType: "request_status", ActorKind: usersModels.ParentMessageSenderStaff,
		ActorAccountID: input.ReviewedBy, Body: state.pillBody,
		RequestType: careRequestPillType(req.RequestKind), RequestStatus: state.pillStatus,
		DecisionReason: state.reason,
	})
	s.wakeGuardiansAfterCommit(ctx, req)
}

func (s *careScheduleRequestService) reloadCareDecisionItem(ctx context.Context, requestID int64) (*CareRequestReviewItem, error) {
	row, err := s.requestRepo.FindByID(ctx, requestID)
	if err != nil {
		return nil, fmt.Errorf("schedule: reload decided care request: %w", err)
	}
	item := &CareRequestReviewItem{Request: row}
	if row.RequestKind == scheduleModels.CareRequestKindPickupChange {
		item.Reason = pickupChangeReason(row)
	}
	student, err := s.studentRepo.FindByID(ctx, row.StudentID)
	if err != nil || student == nil {
		return item, nil
	}
	person, err := s.personRepo.FindByID(ctx, student.PersonID)
	if err == nil && person != nil {
		item.FirstName, item.LastName = person.FirstName, person.LastName
	}
	return item, nil
}

func parsePickupChangePayload(payload map[string]any) (timezone.Date, time.Time, string, error) {
	dateRaw, dateOK := payload["date"].(string)
	pickupRaw, pickupOK := payload["pickup_time"].(string)
	reason, reasonOK := payload["reason"].(string)
	date, dateErr := timezone.ParseDate(dateRaw)
	pickup, pickupErr := parseCareWallClock(pickupRaw)
	reason = strings.TrimSpace(reason)
	if !dateOK || !pickupOK || !reasonOK || dateErr != nil || pickupErr != nil || reason == "" || utf8.RuneCountInString(reason) > 255 {
		return timezone.Date{}, time.Time{}, "", ErrInvalidCareRequestPayload
	}
	return date, pickup, reason, nil
}

func pickupChangeReason(req *scheduleModels.CareScheduleChangeRequest) *string {
	if req == nil {
		return nil
	}
	reason, ok := req.Payload["reason"].(string)
	trimmed := strings.TrimSpace(reason)
	if !ok || trimmed == "" {
		return nil
	}
	return &trimmed
}

func (s *careScheduleRequestService) applyPickupChangeRequest(
	ctx context.Context,
	req *scheduleModels.CareScheduleChangeRequest,
	expectedImpactToken *string,
	requireImpactToken bool,
) error {
	if s.pickupExceptions == nil || s.attendance == nil || s.pickupAutoExcusal == nil || s.userContext == nil {
		return errors.New("schedule: pickup change request dependencies not configured")
	}
	date, pickupTime, reason, err := parsePickupChangePayload(req.Payload)
	if err != nil {
		return err
	}
	if date.Before(timezone.TodayDate()) {
		return ErrPickupChangeExpired
	}
	if err := LockCareExceptionDay(ctx, s.pickupAutoExcusal.db, req.StudentID, date); err != nil {
		return fmt.Errorf("schedule: lock pickup request care day: %w", err)
	}
	if err := s.verifyPickupChangeImpact(ctx, req.StudentID, date, pickupTime, expectedImpactToken, requireImpactToken); err != nil {
		return err
	}
	if err := s.ensurePickupChangeNotCompleted(ctx, req.StudentID, date); err != nil {
		return err
	}
	staff, err := s.resolvePickupChangeStaff(ctx)
	if err != nil {
		return err
	}
	exceptionID, err := s.saveApprovedPickupException(ctx, req, date, pickupTime, reason, staff.ID)
	if err != nil {
		return err
	}
	if _, err := s.pickupAutoExcusal.Sync(ctx, exceptionID); err != nil {
		return fmt.Errorf("schedule: sync approved pickup exception: %w", err)
	}
	return nil
}

func (s *careScheduleRequestService) verifyPickupChangeImpact(
	ctx context.Context, studentID int64, date timezone.Date, pickupTime time.Time,
	expectedImpactToken *string,
	requireImpactToken bool,
) error {
	if expectedImpactToken == nil {
		if requireImpactToken {
			return ErrPickupChangeImpactRequired
		}
		return nil
	}
	blocks, err := s.pickupAutoExcusal.Preview(ctx, studentID, date, pickupTime)
	if err != nil {
		return fmt.Errorf("schedule: verify pickup request impact: %w", err)
	}
	if pickupImpactToken(blocks) != *expectedImpactToken {
		return ErrPickupChangeImpactChanged
	}
	return nil
}

func pickupImpactToken(blocks []scheduleModels.PartialAbsenceBlock) string {
	hash := sha256.New()
	for _, block := range blocks {
		hash.Write([]byte(strconv.FormatInt(block.ID, 10)))
		hash.Write([]byte{0})
		hash.Write([]byte(block.Title))
		hash.Write([]byte{0})
		hash.Write([]byte(block.StartTime.Format("15:04:05")))
		hash.Write([]byte{0})
		hash.Write([]byte(block.EndTime.Format("15:04:05")))
		hash.Write([]byte{0})
	}
	return fmt.Sprintf("%x", hash.Sum(nil))
}

func (s *careScheduleRequestService) saveApprovedPickupException(ctx context.Context, req *scheduleModels.CareScheduleChangeRequest, date timezone.Date, pickupTime time.Time, reason string, staffID int64) (int64, error) {
	existing, err := s.pickupExceptions.FindByStudentIDAndDate(ctx, req.StudentID, date)
	if err != nil {
		return 0, fmt.Errorf("schedule: load pickup exception for request: %w", err)
	}
	if existing != nil {
		if existing.Source == scheduleModels.ExceptionSourceStaff || existing.HasManualPartialAbsence() {
			return 0, ErrPickupChangeConflict
		}
		existing.PickupTime = &pickupTime
		existing.Reason = &reason
		existing.Source = scheduleModels.ExceptionSourceStaff
		existing.CreatedBy = staffID
		existing.CreatedByGuardian = nil
		existing.NormalizeWallClockTimes()
		if err := s.pickupExceptions.Update(ctx, existing); err != nil {
			return 0, fmt.Errorf("schedule: update approved pickup exception: %w", err)
		}
		return existing.ID, nil
	}
	exception := &scheduleModels.StudentPickupException{
		TenantModel:   modelBase.TenantModel{TenantID: req.TenantID},
		StudentID:     req.StudentID,
		ExceptionDate: date,
		PickupTime:    &pickupTime,
		Reason:        &reason,
		Source:        scheduleModels.ExceptionSourceStaff,
		CreatedBy:     staffID,
	}
	if err := s.pickupExceptions.Create(ctx, exception); err != nil {
		if modelBase.IsUniqueViolation(err) {
			return 0, ErrPickupChangeConflict
		}
		return 0, fmt.Errorf("schedule: create approved pickup exception: %w", err)
	}
	return exception.ID, nil
}

func (s *careScheduleRequestService) ensurePickupChangeNotCompleted(
	ctx context.Context, studentID int64, date timezone.Date,
) error {
	if date != timezone.TodayDate() {
		return nil
	}
	if err := s.attendance.LockStudentAttendance(ctx, studentID); err != nil {
		return fmt.Errorf("schedule: lock attendance for pickup request: %w", err)
	}
	rows, err := s.attendance.FindByStudentAndDate(ctx, studentID, date)
	if err != nil {
		return fmt.Errorf("schedule: load attendance for pickup request: %w", err)
	}
	hasOpenAttendance := false
	hasCompletedAttendance := false
	for _, row := range rows {
		if row == nil {
			continue
		}
		if row.CheckOutTime == nil {
			hasOpenAttendance = true
		} else {
			hasCompletedAttendance = true
		}
	}
	if hasCompletedAttendance && !hasOpenAttendance {
		return ErrPickupChangeAlreadyCompleted
	}
	return nil
}

func (s *careScheduleRequestService) resolvePickupChangeStaff(ctx context.Context) (*usersModels.Staff, error) {
	staff, err := s.userContext.GetCurrentStaff(ctx)
	if err != nil {
		if errors.Is(err, userContextService.ErrUserNotLinkedToStaff) || errors.Is(err, userContextService.ErrUserNotLinkedToPerson) {
			return nil, ErrCareRequestForbidden
		}
		return nil, fmt.Errorf("schedule: resolve acting staff for pickup request: %w", err)
	}
	if staff == nil {
		return nil, ErrCareRequestForbidden
	}
	return staff, nil
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
func (s *careScheduleRequestService) applyCareScheduleRequest(ctx context.Context, req *scheduleModels.CareScheduleChangeRequest, reviewedBy int64) (bool, error) {
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
	before := *student

	if err := s.applyDepartureModeChanges(ctx, student, changes.modes, changes.scheduled); err != nil {
		return false, err
	}
	if err := s.applyCareDayChanges(ctx, studentID, changes.scheduled); err != nil {
		return false, err
	}
	if err := s.applyArrivalChanges(ctx, studentID, staffID, changes.arrivals); err != nil {
		return false, err
	}
	if err := s.applyPickupChanges(ctx, studentID, staffID, changes.pickups); err != nil {
		return false, err
	}
	if s.studentAudit != nil {
		if err := s.studentAudit.RecordChangesForActor(ctx, &before, student, reviewedBy); err != nil {
			return false, fmt.Errorf("schedule: audit care request departure: %w", err)
		}
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
func (s *careScheduleRequestService) applyDepartureModeChanges(ctx context.Context, student *usersModels.Student, changes map[string]usersModels.DepartureMode, scheduled map[int]bool) error {
	if len(changes) == 0 && len(scheduled) == 0 {
		return nil
	}
	merged := usersModels.AllowedDepartureModes{}
	for day, modes := range student.AllowedDepartureModes {
		merged[day] = append([]usersModels.DepartureMode(nil), modes...)
	}
	for day, mode := range changes {
		merged[day] = []usersModels.DepartureMode{mode}
	}
	for weekday, active := range scheduled {
		if !active {
			delete(merged, usersModels.PickupDayOrder[weekday-1])
		}
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

func (s *careScheduleRequestService) applyCareDayChanges(ctx context.Context, studentID int64, changes map[int]bool) error {
	if len(changes) == 0 {
		return nil
	}
	arrivals, err := s.arrival.GetStudentArrivalSchedules(ctx, studentID)
	if err != nil {
		return fmt.Errorf("apply care days: load arrivals: %w", err)
	}
	pickups, err := s.pickup.GetStudentPickupSchedules(ctx, studentID)
	if err != nil {
		return fmt.Errorf("apply care days: load pickups: %w", err)
	}
	for weekday, active := range changes {
		if !active {
			managed, managedErr := s.pickup.HasBookedOfferingPickupForWeekday(ctx, studentID, weekday)
			if managedErr != nil {
				return fmt.Errorf("apply care days: check booked offering: %w", managedErr)
			}
			if managed {
				return ErrCareDayManagedByBooking
			}
		}
	}
	for weekday, active := range changes {
		if active {
			continue
		}
		for _, row := range arrivals {
			if row.Weekday == weekday {
				if err := s.arrival.DeleteStudentArrivalSchedule(ctx, row.ID); err != nil {
					return fmt.Errorf("apply care day weekday %d: delete arrival: %w", weekday, err)
				}
			}
		}
		for _, row := range pickups {
			if row.Weekday == weekday {
				if err := s.pickup.DeleteStudentPickupSchedule(ctx, row.ID); err != nil {
					return fmt.Errorf("apply care day weekday %d: delete pickup: %w", weekday, err)
				}
			}
		}
	}
	return nil
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
	Weekday   int    `json:"weekday"`
	Scheduled *bool  `json:"scheduled,omitempty"`
	Mode      string `json:"mode,omitempty"`    // alone|bus|pickup, "" = unchanged
	Arrival   string `json:"arrival,omitempty"` // HH:MM, "" = unchanged
	Pickup    string `json:"pickup,omitempty"`  // HH:MM, "" = unchanged
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
	scheduled map[int]bool
	modes     map[string]usersModels.DepartureMode
	arrivals  map[int]string
	pickups   map[int]string
}

// CareScheduleRequestedFields reports which independently configurable field
// groups a validated permanent-care request contains.
type CareScheduleRequestedFields struct {
	Scheduled     bool
	Arrival       bool
	Pickup        bool
	DepartureMode bool
}

// RequestedCareScheduleFields validates a request payload through the same
// parser used by create/apply and exposes only its field groups to the parent
// policy layer.
func RequestedCareScheduleFields(payload map[string]any) (CareScheduleRequestedFields, error) {
	changes, err := buildCareScheduleChanges(payload)
	if err != nil {
		return CareScheduleRequestedFields{}, err
	}
	if changes.isEmpty() {
		return CareScheduleRequestedFields{}, fmt.Errorf("%w: no changes", ErrInvalidCareRequestPayload)
	}
	return CareScheduleRequestedFields{
		Scheduled:     len(changes.scheduled) > 0,
		Arrival:       len(changes.arrivals) > 0,
		Pickup:        len(changes.pickups) > 0,
		DepartureMode: len(changes.modes) > 0,
	}, nil
}

func (c careScheduleChanges) isEmpty() bool {
	return len(c.scheduled) == 0 && len(c.modes) == 0 && len(c.arrivals) == 0 && len(c.pickups) == 0
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
		if scheduled, ok := c.scheduled[wd]; ok {
			entry.Scheduled = &scheduled
			present = true
		}
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
		scheduled: map[int]bool{},
		modes:     map[string]usersModels.DepartureMode{},
		arrivals:  map[int]string{},
		pickups:   map[int]string{},
	}
	for _, wd := range p.Weekdays {
		if wd.Weekday < 1 || wd.Weekday > 5 {
			return careScheduleChanges{}, fmt.Errorf("%w: weekday %d", ErrInvalidCareRequestPayload, wd.Weekday)
		}
		abbrev := usersModels.PickupDayOrder[wd.Weekday-1]
		if wd.Scheduled != nil {
			out.scheduled[wd.Weekday] = *wd.Scheduled
		}
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
	for weekday, active := range out.scheduled {
		abbrev := usersModels.PickupDayOrder[weekday-1]
		_, hasMode := out.modes[abbrev]
		_, hasArrival := out.arrivals[weekday]
		_, hasPickup := out.pickups[weekday]
		if active && (!hasPickup || !hasMode) {
			return careScheduleChanges{}, fmt.Errorf("%w: scheduled weekday %d needs pickup and mode", ErrInvalidCareRequestPayload, weekday)
		}
		if !active && (hasPickup || hasArrival || hasMode) {
			return careScheduleChanges{}, fmt.Errorf("%w: inactive weekday %d contains plan values", ErrInvalidCareRequestPayload, weekday)
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
	arrivalDays map[int]bool
	arrivalErr  error
	arrivalDone bool

	pickupMap  map[int]string
	pickupErr  error
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

func (d *careDiffSource) getArrival(ctx context.Context) (map[int]string, error) {
	if !d.arrivalDone {
		d.arrivalDone = true
		d.arrivalMap = map[int]string{}
		d.arrivalDays = map[int]bool{}
		cur, err := d.s.arrival.GetStudentArrivalSchedules(ctx, d.studentID)
		if err != nil {
			d.arrivalErr = fmt.Errorf("schedule: load arrival schedules for diff: %w", err)
			return nil, d.arrivalErr
		}
		for _, a := range cur {
			d.arrivalDays[a.Weekday] = true
			if a.ExpectedArrival.IsZero() {
				// Care day without a time: the class carries none yet (#2414).
				continue
			}
			d.arrivalMap[a.Weekday] = a.ExpectedArrival.Format("15:04")
		}
	}
	return d.arrivalMap, d.arrivalErr
}

func (d *careDiffSource) getPickup(ctx context.Context) (map[int]string, error) {
	if !d.pickupDone {
		d.pickupDone = true
		d.pickupMap = map[int]string{}
		cur, err := d.s.pickup.GetStudentPickupSchedules(ctx, d.studentID)
		if err != nil {
			d.pickupErr = fmt.Errorf("schedule: load pickup schedules for diff: %w", err)
			return nil, d.pickupErr
		}
		for _, pc := range cur {
			d.pickupMap[pc.Weekday] = pc.PickupTime.Format("15:04")
		}
	}
	return d.pickupMap, d.pickupErr
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

	arrivalMap, err := src.getArrival(ctx)
	if err != nil {
		return nil, err
	}
	pickupMap, err := src.getPickup(ctx)
	if err != nil {
		return nil, err
	}
	hasCarePlan := len(src.arrivalDays) > 0 || len(pickupMap) > 0

	weekdays := append([]careWeekdayPayload(nil), p.Weekdays...)
	sort.Slice(weekdays, func(i, j int) bool { return weekdays[i].Weekday < weekdays[j].Weekday })

	var entries []RequestDiffEntry
	for _, wd := range weekdays {
		if wd.Weekday < 1 || wd.Weekday > 5 {
			continue
		}
		name := scheduleModels.WeekdayNames[wd.Weekday]
		abbrev := usersModels.PickupDayOrder[wd.Weekday-1]
		if wd.Scheduled != nil {
			oldStatus := CareDayUnknown
			if src.arrivalDays[wd.Weekday] || pickupMap[wd.Weekday] != "" {
				oldStatus = CareDayScheduled
			} else if hasCarePlan {
				oldStatus = CareDayNotScheduled
			}
			entries = append(entries, RequestDiffEntry{
				Label:    name + " · Betreuungstag",
				Old:      careDayStatusGermanLabel(oldStatus),
				New:      careDayGermanLabel(*wd.Scheduled),
				Weekday:  wd.Weekday,
				CareKind: DiffCareKindScheduled,
			})
		}
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

func careDayStatusGermanLabel(status CareDayStatus) string {
	if status == CareDayUnknown {
		return "Keine Angaben"
	}
	return careDayGermanLabel(status == CareDayScheduled)
}

func careDayGermanLabel(scheduled bool) string {
	if scheduled {
		return "In der OGS"
	}
	return "Nicht in der OGS"
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
	return modelBase.IsUniqueViolationOn(err, careRequestPendingUniqueIndex) ||
		modelBase.IsUniqueViolationOn(err, pickupChangePendingUniqueIndex)
}
