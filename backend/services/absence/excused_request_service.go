// Parent absence approval requests use the legacy-named excused request table
// and service. AbsenceStatus distinguishes sick from excused submissions. A
// pending request changes no status day, so the child stays expected until staff
// approve it in the central request queue.
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
	"github.com/moto-nrw/project-phoenix/internal/careplanning"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/realtime"
	notificationsService "github.com/moto-nrw/project-phoenix/services/notifications"
	"github.com/moto-nrw/project-phoenix/services/parentmessaging"
	userContextService "github.com/moto-nrw/project-phoenix/services/usercontext"
	usersService "github.com/moto-nrw/project-phoenix/services/users"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// excusedRequestMaxReasonLen bounds the staff reject reason (in runes).
const excusedRequestMaxReasonLen = 2000

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

var (
	// ErrExcusedRequestForbidden is the legacy-named authorization error for a
	// sick or excused absence request.
	ErrExcusedRequestForbidden = errors.New("active: excused request forbidden")
	// ErrExcusedRequestGuardianAccessRevoked means the submitting guardian is no
	// longer a linked guardian of the child with parent_portal.access, so
	// approving (which writes parent-sourced requested status days and posts a
	// parent-visible "bestätigt" pill for a recipient who can no longer read it)
	// is refused. Staff wind such a request down by REJECTING it — the reject
	// path is deliberately not gated on the guardian link, mirroring the
	// care-schedule request flow.
	ErrExcusedRequestGuardianAccessRevoked = errors.New("active: excused request guardian access revoked")
	// ErrExcusedRequestOverlap is the legacy-named error for a guardian who already has a PENDING absence request
	// whose dates intersect (but are not identical to) the new submission. An
	// identical resubmit is treated as an idempotent retry (the existing row is
	// returned, no error); a partial overlap is refused so two pending requests
	// can't cover the same day with contradictory outcomes. Disjoint date sets are
	// allowed. Maps to a 409 on the parent write path.
	ErrExcusedRequestOverlap = errors.New("active: excused request overlaps an existing pending request")
	// ErrExcusedRequestStatusConflict means a full-day absence request cannot
	// proceed for one of the requested dates because a competing absence already
	// owns that day: either a newer sick / class-trip / excused status (approval
	// would overwrite it), or a planned partial-day excusal (create and approve
	// both refuse so a pending request cannot sit unusable). Maps to a 409.
	ErrExcusedRequestStatusConflict = errors.New("active: excused request superseded by a newer status")
	// ErrExcusedRequestNoDates means the legacy-named absence request carried no dates.
	ErrExcusedRequestNoDates = errors.New("active: excused request requires at least one date")
	// ErrExcusedRequestEmptyNote means the mandatory absence note was blank.
	ErrExcusedRequestEmptyNote = errors.New("active: excused request note must not be empty")
	// ErrExcusedRequestNoteTooLong means the absence note exceeded the bound.
	ErrExcusedRequestNoteTooLong = errors.New("active: excused request note too long")
	// ErrAbsenceRequestInvalidStatus means the requested absence was neither
	// sick nor excused. Class trips remain a staff-only status.
	ErrAbsenceRequestInvalidStatus = errors.New("active: absence request status must be sick or excused")
	// ErrExcusedRequestRejectReasonRequired means staff rejected without a reason.
	ErrExcusedRequestRejectReasonRequired = errors.New("active: reject reason is required")
	// ErrExcusedRequestRejectReasonTooLong means the reason exceeded the bound.
	ErrExcusedRequestRejectReasonTooLong = errors.New("active: reject reason too long")
)

// ExcusedRequestReviewItem is one request enriched with the child's name for
// the staff review queue.
type ExcusedRequestReviewItem struct {
	Request      *activeModels.ExcusedAbsenceRequest
	FirstName    string
	LastName     string
	BulkEligible bool
	// BulkIneligibleReason is the stable code; BulkIneligibleText the German
	// sentence the client falls back to for codes it does not know.
	BulkIneligibleReason string
	BulkIneligibleText   string
	// CurrentStatusByDate says what each REQUESTED day looks like today
	// (present, sick, excused, class_trip), so the review card can put
	// „Aktuell“ next to „Gewünscht“ instead of showing the wish alone.
	CurrentStatusByDate map[string]string
	// CurrentValueChanged is true when one of those days was reported or
	// changed after the request was filed. Nil where it was not resolved.
	CurrentValueChanged *bool
}

// ExcusedRequestHistoryItem is one decided request enriched with the child's
// name and the reviewer's display name for the staff history.
type ExcusedRequestHistoryItem struct {
	Request      *activeModels.ExcusedAbsenceRequest
	FirstName    string
	LastName     string
	ReviewerName string // "" when the row carries no reviewer (withdrawn)
}

// ExcusedRequestDecideInput carries a staff decision on one pending request.
type ExcusedRequestDecideInput struct {
	RequestID int64
	Approve   bool
	Reason    string
	// ReasonRequired says the school's reason policy
	// (operations.parent_request_reason_policy) asks the deciding staff member
	// for a reason on an APPROVAL. A rejection always needs one (#2267).
	ReasonRequired  bool
	ExpectedVersion string
	// ReviewedBy is the acting staff ACCOUNT id (auth.accounts).
	ReviewedBy int64
}

// ExcusedAbsenceRequestService owns the legacy-named absence request lifecycle. All
// methods run inside an ambient tenant transaction: the staff paths under the
// request's TenantTxMiddleware, the parent paths inside the tenant.WithTenantTx
// the parent service opens after resolving the child.
type ExcusedAbsenceRequestService interface {
	// CreateRequest stores a guardian's pending request, then (after commit)
	// posts the "Anfrage erstellt" pill. The caller has already authorized the
	// guardian for this child; note is required and length-bounded here.
	CreateRequest(ctx context.Context, studentID, guardianAccountID int64, dates []timezone.Date, note string) (*activeModels.ExcusedAbsenceRequest, error)
	// CreateRequestForStatus stores either a sick or excused parent absence in
	// the same approval queue. CreateRequest remains the excused compatibility
	// entry point for existing callers.
	CreateRequestForStatus(ctx context.Context, studentID, guardianAccountID int64, dates []timezone.Date, note, absenceStatus string) (*activeModels.ExcusedAbsenceRequest, error)
	// ListForStudent returns the child's pending requests plus any decided since
	// `recentSince`, newest-first — the parent read view.
	ListForStudent(ctx context.Context, studentID int64, recentSince time.Time) ([]*activeModels.ExcusedAbsenceRequest, error)
	// ListPending returns pending requests for the current tenant, newest
	// submission first, enriched with child names — the working list of the
	// Anfragen module. The filters narrow and page the query in SQL; their
	// zero value returns the whole queue.
	ListPending(ctx context.Context, filters modelBase.RequestQueueFilters) (items []*ExcusedRequestReviewItem, next *usersService.HistoryCursor, err error)
	// ListHistory returns decided requests newest-decision-first, keyset
	// paginated on (updated_at, id). A zero BeforeInstant returns the first
	// page; next is nil when no older rows exist beyond this page.
	ListHistory(ctx context.Context, filters modelBase.RequestQueueFilters) (items []*ExcusedRequestHistoryItem, next *usersService.HistoryCursor, err error)
	// PendingByStudentForDate returns, per student, the newest pending request
	// whose dates cover the given calendar day — for the inline planning badge.
	// A student with no pending request for the day is absent from the map.
	PendingByStudentForDate(ctx context.Context, date timezone.Date) (map[int64]*activeModels.ExcusedAbsenceRequest, error)
	// Decide approves (writes the requested status days, then stamps) or rejects
	// (reason required) one pending request and returns the refreshed row.
	Decide(ctx context.Context, input ExcusedRequestDecideInput) (*ExcusedRequestReviewItem, error)
	// EditRequest lets the submitting guardian correct their own still-pending
	// request instead of withdrawing and refiling it (#2267).
	EditRequest(ctx context.Context, input ExcusedRequestEditInput) (*activeModels.ExcusedAbsenceRequest, error)
	// Create is the create path that carries the school's reason policy.
	// CreateRequest / CreateRequestForStatus stay the mandatory-note entry
	// points for callers that have not resolved it.
	Create(ctx context.Context, input ExcusedRequestCreateInput) (*activeModels.ExcusedAbsenceRequest, error)
	ListExcusedBulkCandidates(ctx context.Context) ([]usersService.ExcusedBulkCandidate, error)
	GetExcusedBulkCandidate(ctx context.Context, requestID int64) (*usersService.ExcusedBulkCandidate, error)
	LockExcusedBulkRequest(ctx context.Context, requestID int64) error
	ApproveExcusedBulk(ctx context.Context, requestID int64, reason string, reviewerID int64, expectedVersion string) error
}

// AbsenceNotifierSetter injects the notification producer after construction.
// The notification stack is built after this service in the factory.
type AbsenceNotifierSetter interface {
	SetAbsenceNotifier(notifier notificationsService.AbsenceNotifier)
}

type excusedAbsenceRequestService struct {
	requestRepo   activeModels.ExcusedAbsenceRequestRepository
	statusDayRepo activeModels.StudentStatusDayRepository
	pickupRepo    scheduleModels.StudentPickupExceptionRepository
	studentRepo   usersModels.StudentRepository
	personRepo    usersModels.PersonRepository
	userContext   userContextService.UserContextService
	emitter       *parentmessaging.Emitter
	broadcaster   realtime.Broadcaster
	absenceNotify notificationsService.AbsenceNotifier
	logger        *slog.Logger
	db            *bun.DB
	reviewPolicy  RequestReviewPolicy
	// shareVisibility answers who the parent explicitly shared a request
	// with; nil means nobody was, so every co-guardian gets the neutral line.
	shareVisibility RequestShareVisibilityResolver
	events          usersService.ParentRequestEventRecorder
	today           func() timezone.Date
}

func (s *excusedAbsenceRequestService) todayDate() timezone.Date {
	if s.today != nil {
		return s.today()
	}
	return timezone.TodayDate()
}

type RequestReviewPolicy interface {
	StudentFilter(context.Context, []string) (func(*usersModels.Student) bool, error)
	Allows(context.Context, []string, *usersModels.Student) (bool, error)
}

// NewExcusedAbsenceRequestServiceWithPolicy requires the production review
// policy at construction, so missing wiring cannot widen reviewer access.
func NewExcusedAbsenceRequestServiceWithPolicy(
	requestRepo activeModels.ExcusedAbsenceRequestRepository,
	statusDayRepo activeModels.StudentStatusDayRepository,
	pickupRepo scheduleModels.StudentPickupExceptionRepository,
	studentRepo usersModels.StudentRepository,
	personRepo usersModels.PersonRepository,
	userContext userContextService.UserContextService,
	emitter *parentmessaging.Emitter,
	broadcaster realtime.Broadcaster,
	reviewPolicy RequestReviewPolicy,
	events usersService.ParentRequestEventRecorder,
	logger *slog.Logger,
	db *bun.DB,
	today ...func() timezone.Date,
) ExcusedAbsenceRequestService {
	if reviewPolicy == nil {
		panic("excused absence request review policy is required")
	}
	return newExcusedAbsenceRequestService(
		requestRepo, statusDayRepo, pickupRepo, studentRepo, personRepo,
		userContext, emitter, broadcaster, reviewPolicy, events, logger, db, today...,
	)
}

func newExcusedAbsenceRequestService(
	requestRepo activeModels.ExcusedAbsenceRequestRepository,
	statusDayRepo activeModels.StudentStatusDayRepository,
	pickupRepo scheduleModels.StudentPickupExceptionRepository,
	studentRepo usersModels.StudentRepository,
	personRepo usersModels.PersonRepository,
	userContext userContextService.UserContextService,
	emitter *parentmessaging.Emitter,
	broadcaster realtime.Broadcaster,
	reviewPolicy RequestReviewPolicy,
	events usersService.ParentRequestEventRecorder,
	logger *slog.Logger,
	db *bun.DB,
	today ...func() timezone.Date,
) ExcusedAbsenceRequestService {
	if logger == nil {
		logger = slog.Default()
	}
	service := &excusedAbsenceRequestService{
		requestRepo:   requestRepo,
		statusDayRepo: statusDayRepo,
		pickupRepo:    pickupRepo,
		studentRepo:   studentRepo,
		personRepo:    personRepo,
		userContext:   userContext,
		emitter:       emitter,
		broadcaster:   broadcaster,
		reviewPolicy:  reviewPolicy,
		events:        events,
		logger:        logger,
		db:            db,
	}
	if len(today) > 0 {
		service.today = today[0]
	}
	return service
}

// absenceWritable is the per-child visibility predicate shared by the review
// queue and the pending badge.
func (s *excusedAbsenceRequestService) absenceWritable(ctx context.Context) (func(*usersModels.Student) bool, error) {
	if s.reviewPolicy == nil {
		writable := authorize.AbsenceWritableStudentFilter(ctx, jwt.PermissionsFromCtx(ctx), s.userContext)
		return func(student *usersModels.Student) bool { return writable(student) }, nil
	}
	filter, err := s.reviewPolicy.StudentFilter(ctx, jwt.PermissionsFromCtx(ctx))
	if err != nil {
		return nil, fmt.Errorf("active: resolve request reviewer scope: %w", err)
	}
	return filter, nil
}

func (s *excusedAbsenceRequestService) ListExcusedBulkCandidates(ctx context.Context) ([]usersService.ExcusedBulkCandidate, error) {
	rows, _, err := s.ListPending(ctx, modelBase.RequestQueueFilters{})
	if err != nil {
		return nil, err
	}
	result := make([]usersService.ExcusedBulkCandidate, 0, len(rows))
	for _, row := range rows {
		result = append(result, usersService.ExcusedBulkCandidate{
			ID: row.Request.ID, StudentID: row.Request.StudentID, UpdatedAt: row.Request.UpdatedAt,
			Eligible: row.BulkEligible,
		})
	}
	return result, nil
}

func (s *excusedAbsenceRequestService) GetExcusedBulkCandidate(ctx context.Context, requestID int64) (*usersService.ExcusedBulkCandidate, error) {
	req, err := s.requestRepo.FindByID(ctx, requestID)
	if errors.Is(err, activeModels.ErrExcusedRequestNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if req.Status != activeModels.ExcusedRequestStatusPending {
		return nil, nil
	}
	students, err := s.studentRepo.FindByIDs(ctx, []int64{req.StudentID})
	if err != nil {
		return nil, err
	}
	student := students[req.StudentID]
	writable, err := s.absenceWritable(ctx)
	if err != nil {
		return nil, err
	}
	if student == nil || !writable(student) || student.IsAlumnus() || student.CareEndedOn(s.todayDate()) {
		return nil, nil
	}
	view, err := s.currentStatusView(ctx, req)
	if err != nil {
		return nil, err
	}
	eligible, _, _, err := s.excusedBulkEligibility(ctx, req, student, view)
	if err != nil {
		return nil, err
	}
	return &usersService.ExcusedBulkCandidate{ID: req.ID, StudentID: req.StudentID, UpdatedAt: req.UpdatedAt, Eligible: eligible}, nil
}

func (s *excusedAbsenceRequestService) ApproveExcusedBulk(
	ctx context.Context,
	requestID int64,
	reason string,
	reviewerID int64,
	expectedVersion string,
) error {
	_, err := s.Decide(ctx, ExcusedRequestDecideInput{
		RequestID: requestID, Approve: true, Reason: reason, ReviewedBy: reviewerID,
		ExpectedVersion: expectedVersion,
	})
	if errors.Is(err, activeModels.ErrExcusedRequestNotPending) {
		return usersService.ErrParentRequestDecisionRace
	}
	return err
}

func (s *excusedAbsenceRequestService) LockExcusedBulkRequest(ctx context.Context, requestID int64) error {
	_, err := s.requestRepo.FindPendingByIDForUpdate(ctx, requestID)
	return err
}

// SetAbsenceNotifier implements AbsenceNotifierSetter.
func (s *excusedAbsenceRequestService) SetAbsenceNotifier(notifier notificationsService.AbsenceNotifier) {
	s.absenceNotify = notifier
}

func (s *excusedAbsenceRequestService) CreateRequest(ctx context.Context, studentID, guardianAccountID int64, dates []timezone.Date, note string) (*activeModels.ExcusedAbsenceRequest, error) {
	return s.CreateRequestForStatus(ctx, studentID, guardianAccountID, dates, note, activeModels.StudentStatusDayExcused)
}

// CreateRequestForStatus keeps the mandatory-note behaviour every existing
// caller relies on. A caller that has resolved
// operations.parent_request_reason_policy uses Create instead and says whether
// the note is required (#2267, story 28).
func (s *excusedAbsenceRequestService) CreateRequestForStatus(ctx context.Context, studentID, guardianAccountID int64, dates []timezone.Date, note, absenceStatus string) (*activeModels.ExcusedAbsenceRequest, error) {
	return s.Create(ctx, ExcusedRequestCreateInput{
		StudentID:         studentID,
		GuardianAccountID: guardianAccountID,
		Dates:             dates,
		Note:              note,
		AbsenceStatus:     absenceStatus,
		NoteRequired:      true,
	})
}

// ExcusedRequestCreateInput is one guardian absence submission. NoteRequired
// carries the school's reason policy, resolved by the caller that knows the
// tenant.
type ExcusedRequestCreateInput struct {
	StudentID         int64
	GuardianAccountID int64
	Dates             []timezone.Date
	Note              string
	AbsenceStatus     string
	NoteRequired      bool
}

func (s *excusedAbsenceRequestService) Create(ctx context.Context, input ExcusedRequestCreateInput) (*activeModels.ExcusedAbsenceRequest, error) {
	studentID, guardianAccountID := input.StudentID, input.GuardianAccountID
	dates, note, absenceStatus := input.Dates, input.Note, input.AbsenceStatus
	sorted, trimmed, err := validateAbsenceRequestInput(dates, note, absenceStatus, input.NoteRequired)
	if err != nil {
		return nil, err
	}
	if err := s.ensureNoPartialAbsence(ctx, &activeModels.ExcusedAbsenceRequest{
		StudentID: studentID,
		Dates:     sorted,
	}); err != nil {
		return nil, err
	}
	existing, err := s.findMatchingPendingRequest(ctx, studentID, sorted, absenceStatus)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return existing, nil
	}
	req := &activeModels.ExcusedAbsenceRequest{
		StudentID:     studentID,
		SubmittedBy:   guardianAccountID,
		Dates:         sorted,
		Note:          trimmed,
		AbsenceStatus: absenceStatus,
		Status:        activeModels.ExcusedRequestStatusPending,
	}
	if err := s.requestRepo.Create(ctx, req); err != nil {
		return nil, fmt.Errorf("active: create absence request: %w", err)
	}
	if err := usersService.RecordParentRequestEvent(ctx, s.events, usersService.ParentRequestEventInput{
		StudentID:      req.StudentID,
		RequestType:    usersModels.ParentRequestTypeExcusedAbsence,
		RequestID:      req.ID,
		EventType:      usersModels.ParentRequestEventSubmitted,
		ActorAccountID: guardianAccountID,
		UpdatedAt:      req.UpdatedAt,
		Payload:        map[string]any{"status": absenceStatus, "days": len(sorted)},
	}); err != nil {
		return nil, fmt.Errorf("active: record absence request event: %w", err)
	}
	// A new pending request adds the "Freigabe ausstehend" badge on the child;
	// wake staff tabs so planning/search views pick it up without a manual
	// refetch.
	s.broadcastRequestTransition(ctx, req.TenantID, req.StudentID)
	s.emitCreatedRequestPill(ctx, req, guardianAccountID)
	return req, nil
}

// validateAbsenceRequestInput is the shared gate of the create and the
// guardian edit path. noteRequired carries the school's reason policy
// (operations.parent_request_reason_policy): a school that asks nobody for a
// reason accepts a blank note, everything else about the request is validated
// the same either way.
func validateAbsenceRequestInput(dates []timezone.Date, note, absenceStatus string, noteRequired bool) ([]timezone.Date, string, error) {
	if len(dates) == 0 {
		return nil, "", ErrExcusedRequestNoDates
	}
	if absenceStatus != activeModels.StudentStatusDaySick && absenceStatus != activeModels.StudentStatusDayExcused {
		return nil, "", ErrAbsenceRequestInvalidStatus
	}
	trimmed := strings.TrimSpace(note)
	if trimmed == "" && noteRequired {
		return nil, "", ErrExcusedRequestEmptyNote
	}
	if utf8.RuneCountInString(trimmed) > excusedRequestMaxReasonLen {
		return nil, "", ErrExcusedRequestNoteTooLong
	}
	return dedupeSortedDates(dates), trimmed, nil
}

func (s *excusedAbsenceRequestService) findMatchingPendingRequest(ctx context.Context, studentID int64, dates []timezone.Date, absenceStatus string) (*activeModels.ExcusedAbsenceRequest, error) {
	// Lock after the care-day locks so concurrent partial and full-day writes use
	// one ordering. The advisory lock serializes the read-then-insert below.
	if err := s.requestRepo.LockStudentRequests(ctx, studentID); err != nil {
		return nil, err
	}
	existing, err := s.requestRepo.ListPendingForStudent(ctx, studentID)
	if err != nil {
		return nil, fmt.Errorf("active: check existing pending absence requests: %w", err)
	}
	for _, pending := range existing {
		sameStatus := normalizedAbsenceRequestStatus(pending.AbsenceStatus) == absenceStatus
		if sameStatus && sameDateSet(pending.Dates, dates) {
			return pending, nil
		}
		if datesIntersect(pending.Dates, dates) {
			return nil, ErrExcusedRequestOverlap
		}
	}
	return nil, nil
}

func (s *excusedAbsenceRequestService) emitCreatedRequestPill(ctx context.Context, req *activeModels.ExcusedAbsenceRequest, guardianAccountID int64) {
	createdBody, _, requestType := absenceRequestCopy(req.AbsenceStatus)
	s.emitRequestPillAfterCommit(ctx, req, parentmessaging.ChildEvent{
		EventType:      usersModels.ParentMessageEventRequestCreated,
		ActorKind:      usersModels.ParentMessageSenderGuardian,
		ActorAccountID: guardianAccountID,
		Body:           createdBody,
		RequestType:    requestType,
		RequestStatus:  usersModels.ParentMessageRequestStatusOpen,
	})
}

// ExcusedRequestEditInput is one guardian edit of their own still-open
// absence request (#2267, story 37). The absence kind is NOT editable: a
// Krankmeldung that turns into a Terminabwesenheit is a different request and
// may be gated differently by the school.
type ExcusedRequestEditInput struct {
	RequestID         int64
	StudentID         int64
	GuardianAccountID int64
	ExpectedVersion   string
	// NoteRequired carries the school's reason policy, resolved by the caller
	// that knows the tenant (#2267, story 28).
	NoteRequired bool
	Dates        []timezone.Date
	Note         string
}

// EditRequest rewrites the submitter's own pending request. It replaces the
// withdraw flow: a guardian who mistyped a date corrects it instead of
// cancelling and refiling, so the request keeps its id, its share and its
// history. A request that is not the caller's own is reported as not found,
// never as forbidden — a stranger must not learn that the id exists.
func (s *excusedAbsenceRequestService) EditRequest(
	ctx context.Context, input ExcusedRequestEditInput,
) (*activeModels.ExcusedAbsenceRequest, error) {
	// Lock regardless of status so ownership is checked BEFORE the
	// pending-status distinction, like the decide path.
	req, err := s.requestRepo.FindByIDForUpdate(ctx, input.RequestID)
	if err != nil {
		return nil, err
	}
	if req.SubmittedBy != input.GuardianAccountID || req.StudentID != input.StudentID {
		return nil, activeModels.ErrExcusedRequestNotFound
	}
	if req.Status != activeModels.ExcusedRequestStatusPending {
		return nil, activeModels.ErrExcusedRequestNotPending
	}
	if input.ExpectedVersion != "" && usersService.ParentRequestVersion(req.UpdatedAt) != input.ExpectedVersion {
		return nil, usersService.ErrParentRequestStale
	}
	absenceStatus := normalizedAbsenceRequestStatus(req.AbsenceStatus)
	// Same validators the create path runs — an edit may not produce a request
	// the create path would have refused.
	sorted, trimmed, err := validateAbsenceRequestInput(input.Dates, input.Note, absenceStatus, input.NoteRequired)
	if err != nil {
		return nil, err
	}
	if err := s.ensureNoPartialAbsence(ctx, &activeModels.ExcusedAbsenceRequest{
		StudentID: req.StudentID,
		Dates:     sorted,
	}); err != nil {
		return nil, err
	}
	if err := s.requestRepo.UpdatePending(ctx, req.ID, sorted, trimmed, absenceStatus); err != nil {
		return nil, err
	}
	row, err := s.requestRepo.FindByID(ctx, req.ID)
	if err != nil {
		return nil, fmt.Errorf("active: reload edited absence request: %w", err)
	}
	if err := usersService.RecordParentRequestEvent(ctx, s.events, usersService.ParentRequestEventInput{
		StudentID:      row.StudentID,
		RequestType:    usersModels.ParentRequestTypeExcusedAbsence,
		RequestID:      row.ID,
		EventType:      usersModels.ParentRequestEventGuardianEdit,
		ActorAccountID: input.GuardianAccountID,
		UpdatedAt:      row.UpdatedAt,
		Payload:        map[string]any{"days": len(sorted)},
	}); err != nil {
		return nil, fmt.Errorf("active: record absence edit event: %w", err)
	}
	// The pending badge does not change, but the dates staff see do — wake the
	// staff tabs the same way a create does.
	s.broadcastRequestTransition(ctx, row.TenantID, row.StudentID)
	return row, nil
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
		// APPROVED requests, though: they normally surface as status days,
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

func (s *excusedAbsenceRequestService) ListHistory(ctx context.Context, filters modelBase.RequestQueueFilters) ([]*ExcusedRequestHistoryItem, *usersService.HistoryCursor, error) {
	// limit+1 probes for an older page without a second count query.
	rows, err := s.requestRepo.ListDecidedForTenant(ctx, probeLimit(filters))
	if err != nil {
		return nil, nil, fmt.Errorf("active: list decided excused requests: %w", err)
	}
	// The cursor points at the last DB row (not the last visible item): the
	// per-child scope filters after the DB limit, so a cursor built from the
	// filtered page would skip rows.
	rows, next := usersService.NextCursor(rows, filters.Limit, func(r *activeModels.ExcusedAbsenceRequest) (time.Time, int64) {
		return r.UpdatedAt, r.ID
	})
	if len(rows) == 0 {
		return []*ExcusedRequestHistoryItem{}, next, nil
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
		return nil, nil, fmt.Errorf("active: load students for excused request history: %w", err)
	}
	personIDs := make([]int64, 0, len(students))
	for _, st := range students {
		personIDs = append(personIDs, st.PersonID)
	}
	persons, err := s.personRepo.FindByIDs(ctx, personIDs)
	if err != nil {
		return nil, nil, fmt.Errorf("active: load persons for excused request history: %w", err)
	}
	reviewers, err := s.personRepo.FindByAccountIDs(ctx, reviewerIDs)
	if err != nil {
		return nil, nil, fmt.Errorf("active: load reviewers for excused request history: %w", err)
	}

	// Same per-child scope as ListPending: absence write gate + alumnus skip.
	writable, err := s.absenceWritable(ctx)
	if err != nil {
		return nil, nil, err
	}

	items := make([]*ExcusedRequestHistoryItem, 0, len(rows))
	for _, r := range rows {
		st := students[r.StudentID]
		if st == nil || !writable(st) || st.IsAlumnus() {
			continue
		}
		item := &ExcusedRequestHistoryItem{
			Request:      r,
			ReviewerName: usersService.ReviewerDisplayName(reviewers, r.ReviewedBy),
		}
		if p, ok := persons[st.PersonID]; ok {
			item.FirstName = p.FirstName
			item.LastName = p.LastName
		}
		items = append(items, item)
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

func (s *excusedAbsenceRequestService) ListPending(ctx context.Context, filters modelBase.RequestQueueFilters) ([]*ExcusedRequestReviewItem, *usersService.HistoryCursor, error) {
	// limit+1 probes for an older page without a second count query.
	rows, err := s.requestRepo.ListPendingForTenant(ctx, probeLimit(filters))
	if err != nil {
		return nil, nil, fmt.Errorf("active: list pending excused requests: %w", err)
	}
	rows, next := usersService.NextCursor(rows, filters.Limit, func(r *activeModels.ExcusedAbsenceRequest) (time.Time, int64) {
		return r.CreatedAt, r.ID
	})
	if len(rows) == 0 {
		return []*ExcusedRequestReviewItem{}, next, nil
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
		return nil, nil, fmt.Errorf("active: load students for excused requests: %w", err)
	}
	personIDs := make([]int64, 0, len(students))
	for _, st := range students {
		personIDs = append(personIDs, st.PersonID)
	}
	persons, err := s.personRepo.FindByIDs(ctx, personIDs)
	if err != nil {
		return nil, nil, fmt.Errorf("active: load persons for excused requests: %w", err)
	}

	// Scope the queue to children whose absences the caller may WRITE — the
	// same gate as Decide, so a staffer only ever sees requests they can act
	// on. This also scopes the sidebar badge.
	writable, err := s.absenceWritable(ctx)
	if err != nil {
		return nil, nil, err
	}

	items := make([]*ExcusedRequestReviewItem, 0, len(rows))
	for _, r := range rows {
		if !writable(students[r.StudentID]) {
			continue
		}
		// A graduated child is soft-deleted: their pending requests survive the
		// graduation (the hard delete it replaced cascaded them away), so without
		// this they would sit in the staff queue and the sidebar badge and could
		// still be approved onto an alumnus. A revert brings them back (#405 review).
		// A child whose care has ended leaves the queue the same way: the
		// effect-day pass closes their open requests, and until it runs the
		// queue must not offer a decision on a departed child (#2487).
		if students[r.StudentID].IsAlumnus() || students[r.StudentID].CareEndedOn(s.todayDate()) {
			continue
		}
		view, viewErr := s.currentStatusView(ctx, r)
		if viewErr != nil {
			return nil, nil, fmt.Errorf("active: resolve current absence statuses: %w", viewErr)
		}
		eligible, reasonCode, reasonText, eligibilityErr := s.excusedBulkEligibility(ctx, r, students[r.StudentID], view)
		if eligibilityErr != nil {
			return nil, nil, fmt.Errorf("active: resolve absence bulk eligibility: %w", eligibilityErr)
		}
		currentValueChanged := view.changedSinceRequest
		item := &ExcusedRequestReviewItem{
			Request: r, BulkEligible: eligible,
			BulkIneligibleReason: reasonCode, BulkIneligibleText: reasonText,
			CurrentStatusByDate: view.byDate,
			CurrentValueChanged: &currentValueChanged,
		}
		if st, ok := students[r.StudentID]; ok {
			if p, ok := persons[st.PersonID]; ok {
				item.FirstName = p.FirstName
				item.LastName = p.LastName
			}
		}
		items = append(items, item)
	}
	return items, next, nil
}

// excusedBulkEligibility answers whether this request can ride a bulk
// approval. It takes the already-resolved status view so the queue reads the
// child.s current days exactly once per request, whichever branch decides.
func (s *excusedAbsenceRequestService) excusedBulkEligibility(
	ctx context.Context,
	req *activeModels.ExcusedAbsenceRequest,
	student *usersModels.Student,
	view currentStatusView,
) (bool, string, string, error) {
	if !usersService.AbsenceBulkEligible(req.Dates, s.todayDate()) {
		return false, usersService.BulkIneligiblePast, "Mindestens ein Tag ist vorbei.", nil
	}
	if requestExtendsBeyondCare(req, student) {
		return false, usersService.BulkIneligibleChildUnavailable, "Mindestens ein Tag liegt nach dem Betreuungsende.", nil
	}
	partial, err := s.hasManualPartialAbsence(ctx, req)
	if err != nil {
		return false, "", "", err
	}
	if partial {
		return false, usersService.BulkIneligibleConflict, "Für mindestens einen Tag ist bereits eine Teilabwesenheit eingetragen.", nil
	}
	if s.emitter != nil {
		hasAccess, accessErr := s.emitter.GuardianHasChildAccess(ctx, req.StudentID, req.SubmittedBy)
		if accessErr != nil {
			return false, "", "", accessErr
		}
		if !hasAccess {
			return false, usersService.BulkIneligibleAccessRevoked, "Der Zugang der einreichenden Person ist nicht mehr aktiv.", nil
		}
	}
	if view.changedSinceRequest {
		return false, usersService.BulkIneligibleStale, "Für mindestens einen Tag gibt es einen neueren Abwesenheitsstatus.", nil
	}
	return true, "", "", nil
}

func (s *excusedAbsenceRequestService) hasManualPartialAbsence(
	ctx context.Context,
	req *activeModels.ExcusedAbsenceRequest,
) (bool, error) {
	if len(req.Dates) == 0 || s.pickupRepo == nil {
		return false, nil
	}
	rows, err := s.pickupRepo.FindByStudentIDAndDateRange(
		ctx, req.StudentID, req.Dates[0], req.Dates[len(req.Dates)-1],
	)
	if err != nil {
		return false, err
	}
	requested := make(map[timezone.Date]struct{}, len(req.Dates))
	for _, date := range req.Dates {
		requested[date] = struct{}{}
	}
	for _, row := range rows {
		if _, ok := requested[row.ExceptionDate]; ok && row.HasManualPartialAbsence() {
			return true, nil
		}
	}
	return false, nil
}

func (s *excusedAbsenceRequestService) PendingByStudentForDate(ctx context.Context, date timezone.Date) (map[int64]*activeModels.ExcusedAbsenceRequest, error) {
	rows, err := s.requestRepo.ListPendingForTenant(ctx, modelBase.RequestQueueFilters{})
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
	writable, err := s.absenceWritable(ctx)
	if err != nil {
		return nil, err
	}
	out := make(map[int64]*activeModels.ExcusedAbsenceRequest, len(candidates))
	for studentID, req := range candidates {
		// Alumni are excluded for the same reason as in the review queue: a
		// graduated child must not raise a pending-approval marker anywhere on the
		// staff surface (#405 review).
		if writable(students[studentID]) && !students[studentID].IsAlumnus() &&
			!students[studentID].CareEndedOn(s.todayDate()) {
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
	if !input.Approve && reason == "" {
		return nil, ErrExcusedRequestRejectReasonRequired
	}
	// An approval needs a reason only while the school's policy asks staff for
	// one; a rejection always does (#2267, story 28).
	if input.Approve && input.ReasonRequired && reason == "" {
		return nil, usersService.ErrParentRequestReasonRequired
	}
	// The length bound applies to both verdicts: an approval reason is stored
	// and shown in the history exactly like a rejection reason (#2267).
	if utf8.RuneCountInString(reason) > excusedRequestMaxReasonLen {
		return nil, ErrExcusedRequestRejectReasonTooLong
	}

	// Lock the row and re-verify it is still pending so two staff deciding the
	// same request (or a decide racing the guardian's withdrawal) serialize.
	req, err := s.requestRepo.FindPendingByIDForUpdate(ctx, input.RequestID)
	if err != nil {
		return nil, err
	}
	if input.ExpectedVersion != "" && usersService.ParentRequestVersion(req.UpdatedAt) != input.ExpectedVersion {
		return nil, usersService.ErrParentRequestStale
	}
	req.AbsenceStatus = normalizedAbsenceRequestStatus(req.AbsenceStatus)

	// Per-child write authorization: decided by the parent-request review
	// policy — administrators school-wide, group leaders only for their own
	// groups and only while the school enables it (#2267). The absence write
	// permissions gate the ROUTE; this decides the child.
	//
	// Taken FOR UPDATE so the alumnus gate below decides on a state a concurrent
	// grade transition cannot change underneath it: the transition apply locks
	// exactly this row before flipping it to alumnus, so an unlocked read could
	// see "active", let the approve through, and have the graduation commit
	// before the status-day writes land — absence rows written for, or cleared
	// from, a child who is an alumnus by then. Under the lock the two serialize.
	// This is the only student row this transaction locks, so the ascending-id
	// order every student-row locker follows is preserved (#405 review).
	student, err := s.studentRepo.FindByIDForUpdate(ctx, req.StudentID)
	if err != nil {
		return nil, fmt.Errorf("active: load student for absence request decision: %w", err)
	}
	// The child graduated after filing this request. The lookup is unfiltered, so
	// without this gate an approve would still write absence status days for an
	// alumnus. Same 404 the rest of the child surface returns for graduates
	// (#405 review).
	if student.IsAlumnus() {
		return nil, activeModels.ErrExcusedRequestNotFound
	}
	// The child left the OGS after filing this request; approving it would
	// write absence status days for a day they are no longer in care (#2487).
	if student.CareEndedOn(s.todayDate()) {
		return nil, activeModels.ErrExcusedRequestNotFound
	}
	allowed, authErr := s.canReviewStudent(ctx, student)
	if authErr != nil {
		return nil, authErr
	}
	if !allowed {
		return nil, ErrExcusedRequestForbidden
	}

	if input.Approve {
		// Approving a request whose days have all passed would write absence
		// records into a settled past. Staff either reject it or mark it done
		// (#2267, story 14). Rejecting stays allowed.
		if usersService.ParentRequestIsPast(excusedScopeEnd(req), s.todayDate()) {
			return nil, usersService.ErrParentRequestPast
		}
		if requestExtendsBeyondCare(req, student) {
			return nil, activeModels.ErrExcusedRequestNotFound
		}
		if err := s.ensureNoPartialAbsence(ctx, req); err != nil {
			return nil, err
		}
		// Refuse to APPLY when the submitting guardian has lost access to the
		// child (unlinked or parent_portal.access revoked) since the request was
		// filed. Approving writes parent-sourced requested status days and posts a
		// parent-visible "bestätigt" pill for a recipient the parent APIs now
		// hide; staff wind such a stale request down by REJECTING it (reject is
		// deliberately not gated). Mirrors the care-schedule request guard.
		if s.emitter != nil {
			hasAccess, err := s.emitter.GuardianHasChildAccess(ctx, req.StudentID, req.SubmittedBy)
			if err != nil {
				return nil, fmt.Errorf("active: absence request guardian link check: %w", err)
			}
			if !hasAccess {
				return nil, ErrExcusedRequestGuardianAccessRevoked
			}
		}
		// Refuse to APPLY when the child's absence for a requested date was created
		// or changed AFTER this request was filed (a newer sick / class-trip /
		// excused record from staff or another flow). applyAbsenceRequest clears
		// every other status for those dates and upserts the requested status, so a
		// blind approve would silently overwrite that newer decision. Detect it and
		// return a conflict; staff resolve it by rejecting the stale request.
		if err := s.ensureNoNewerStatus(ctx, req); err != nil {
			return nil, err
		}
		// Apply, status update and after-commit hooks all run in the ambient
		// tenant transaction. A mid-apply failure propagates as a plain error
		// (→ 500) so the WHOLE transaction rolls back rather than committing a
		// half-applied absence.
		if err := s.applyAbsenceRequest(ctx, req); err != nil {
			return nil, err
		}
	}

	newStatus := activeModels.ExcusedRequestStatusApproved
	pillStatus := usersModels.ParentMessageRequestStatusDone
	_, pillBody, requestType := absenceRequestCopy(req.AbsenceStatus)
	var reasonPtr *string
	// A staff reason is kept for BOTH verdicts: without it the history shows an
	// approval with no explanation ("Historie ohne Grund", #2267).
	if reason != "" {
		reasonPtr = &reason
	}
	if !input.Approve {
		newStatus = activeModels.ExcusedRequestStatusRejected
		pillStatus = usersModels.ParentMessageRequestStatusRejected
		pillBody = absenceRequestRejectedBody(req.AbsenceStatus) + ": " + reason
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
				Status:             req.AbsenceStatus,
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
			s.logger.Info("absence request approved",
				slog.Int64("request_id", req.ID),
				slog.Int64("student_id", req.StudentID),
				slog.Int64("tenant_id", req.TenantID),
				slog.Int64("reviewed_by", input.ReviewedBy),
				slog.String("status", req.AbsenceStatus),
				slog.Int("days", len(req.Dates)),
			)
		})
	} else {
		tenant.RegisterAfterCommit(ctx, func() {
			s.logger.Info("absence request rejected",
				slog.Int64("request_id", req.ID),
				slog.Int64("student_id", req.StudentID),
				slog.Int64("tenant_id", req.TenantID),
				slog.Int64("reviewed_by", input.ReviewedBy),
				slog.String("status", req.AbsenceStatus),
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
		RequestType:    requestType,
		RequestStatus:  pillStatus,
		DecisionReason: reason,
	})

	// Every other guardian of this child gets a neutral line: the care
	// changed, on these days. No reason, no author (#2267, story 47).
	// Explicit share recipients get the submitter's pill verbatim (body and
	// reason included); the helper derives the neutral line for everyone else.
	s.notifyOtherGuardiansAfterCommit(ctx, req, parentmessaging.ChildEvent{
		EventType:      usersModels.ParentMessageEventRequestStatus,
		ActorKind:      usersModels.ParentMessageSenderStaff,
		ActorAccountID: input.ReviewedBy,
		Body:           pillBody,
		RequestType:    requestType,
		RequestStatus:  pillStatus,
		DecisionReason: reason,
	})

	row, err := s.requestRepo.FindByID(ctx, req.ID)
	if err != nil {
		return nil, fmt.Errorf("active: reload decided absence request: %w", err)
	}
	// Inside the decision's own transaction: a rolled-back decision must not
	// leave a "decided" event behind.
	if err := usersService.RecordParentRequestEvent(ctx, s.events, usersService.ParentRequestEventInput{
		StudentID:      row.StudentID,
		RequestType:    usersModels.ParentRequestTypeExcusedAbsence,
		RequestID:      row.ID,
		EventType:      usersModels.ParentRequestEventDecided,
		ActorAccountID: input.ReviewedBy,
		UpdatedAt:      row.UpdatedAt,
		Payload:        map[string]any{"approve": input.Approve, "reason": reason},
	}); err != nil {
		return nil, fmt.Errorf("active: record absence decision event: %w", err)
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

func (s *excusedAbsenceRequestService) canReviewStudent(ctx context.Context, student *usersModels.Student) (bool, error) {
	if s.reviewPolicy == nil {
		ok, _ := authorize.CanManageStudentAbsence(ctx, jwt.PermissionsFromCtx(ctx), student, s.userContext)
		return ok, nil
	}
	ok, err := s.reviewPolicy.Allows(ctx, jwt.PermissionsFromCtx(ctx), student)
	if err != nil {
		return false, fmt.Errorf("active: resolve request reviewer scope: %w", err)
	}
	return ok, nil
}

func requestExtendsBeyondCare(req *activeModels.ExcusedAbsenceRequest, student *usersModels.Student) bool {
	if req == nil || student == nil || student.EnrolledUntil == nil {
		return false
	}
	for _, date := range req.Dates {
		if date.After(*student.EnrolledUntil) {
			return true
		}
	}
	return false
}

func (s *excusedAbsenceRequestService) ensureNoPartialAbsence(
	ctx context.Context, req *activeModels.ExcusedAbsenceRequest,
) error {
	if len(req.Dates) == 0 {
		return nil
	}
	if s.db == nil && s.pickupRepo == nil {
		return nil
	}
	if s.db == nil {
		return errors.New("active: database is not configured")
	}
	if s.pickupRepo == nil {
		return errors.New("active: pickup exception repository is not configured")
	}
	// Sort before locking so multi-day writers always take care-day keys in
	// the same order (avoids AB-BA deadlocks across concurrent requests).
	// Student row first, then care-day — same order as partial-absence writes
	// (LockStudentAndExceptionDay). Care-day-only here deadlocks when the
	// request INSERT's student FK waits on a partial writer that holds the
	// student row and is waiting for care-day.
	sortedDates := append([]timezone.Date(nil), req.Dates...)
	sort.Slice(sortedDates, func(i, j int) bool {
		return sortedDates[i].Before(sortedDates[j])
	})
	for _, date := range sortedDates {
		if err := careplanning.LockStudentAndExceptionDay(ctx, s.db, req.StudentID, date); err != nil {
			return err
		}
	}
	rows, err := s.pickupRepo.FindByStudentIDAndDateRange(
		ctx, req.StudentID, sortedDates[0], sortedDates[len(sortedDates)-1],
	)
	if err != nil {
		return err
	}
	requested := make(map[timezone.Date]struct{}, len(req.Dates))
	for _, date := range req.Dates {
		requested[date] = struct{}{}
	}
	for _, row := range rows {
		// Only manual partial absences conflict; auto-derived excusals
		// (pulled-forward pickup time, #2360) coexist with a full-day status.
		if _, ok := requested[row.ExceptionDate]; ok && row.HasManualPartialAbsence() {
			return ErrExcusedRequestStatusConflict
		}
	}
	return nil
}

// ensureNoNewerStatus refuses the approval when any ACTIVE status day on a
// requested date was reported after the request was created — i.e. a newer
// absence decision (sick, class trip, or an updated excused note) that approving
// would clobber. It compares each active row's reported_at against the request's
// created_at: a status set BEFORE the request is the older state the parent's
// request legitimately supersedes (no conflict); a status set AFTER is a fresher
// decision that must win. Runs inside the ambient tenant transaction, right
// before applyAbsenceRequest, so the window between check and write is minimal.
func (s *excusedAbsenceRequestService) ensureNoNewerStatus(ctx context.Context, req *activeModels.ExcusedAbsenceRequest) error {
	view, err := s.currentStatusView(ctx, req)
	if err != nil {
		return err
	}
	if view.changedSinceRequest {
		return ErrExcusedRequestStatusConflict
	}
	return nil
}

// currentStatusView is what the child's days look like RIGHT NOW, for exactly
// the days this request asks about. Two readers share it, so they can never
// disagree: the approve gate (which refuses when a day changed after the
// request was filed) and the review queue (which shows staff „Aktuell →
// Gewünscht“ per day, and warns when the value moved).
//
// It is one query — the same one the approve gate always ran — so adding the
// staff-facing view to the queue costs nothing extra.
type currentStatusView struct {
	// byDate maps every REQUESTED day to what the child's record says today:
	// sick, excused, class_trip, or present when no status day is active.
	byDate map[string]string
	// changedSinceRequest is true when at least one of those days was
	// reported or changed after the request was filed. Approving would
	// overwrite that newer decision.
	changedSinceRequest bool
}

func (s *excusedAbsenceRequestService) currentStatusView(
	ctx context.Context,
	req *activeModels.ExcusedAbsenceRequest,
) (currentStatusView, error) {
	view := currentStatusView{byDate: make(map[string]string, len(req.Dates))}
	if len(req.Dates) == 0 {
		return view, nil
	}
	// Every requested day starts as "present"; the rows below overwrite the
	// days that carry a status, so a day with no record is still answered for.
	for _, date := range req.Dates {
		view.byDate[date.String()] = activeModels.StudentStatusDayPresent
	}
	// req.Dates is stored sorted ascending (dedupeSortedDates), so first/last bound
	// the range; the query spans min..max and we filter back to the exact dates.
	rows, err := s.statusDayRepo.FindActiveByStudentAndDateRange(
		ctx, req.StudentID, req.Dates[0], req.Dates[len(req.Dates)-1],
	)
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

// applyAbsenceRequest writes the approved request using its requested status,
// mirroring the direct parent path. It also updates today's live sick flag.
func (s *excusedAbsenceRequestService) applyAbsenceRequest(ctx context.Context, req *activeModels.ExcusedAbsenceRequest) error {
	if len(req.Dates) == 0 {
		return ErrExcusedRequestNoDates
	}
	now := time.Now()
	note := req.Note
	notePtr := &note

	for _, other := range activeModels.StudentStatusDayStatusesExcept(req.AbsenceStatus) {
		if err := s.statusDayRepo.MarkClearedForDates(ctx, req.StudentID, other, req.Dates, now, activeModels.StudentStatusSourceParent); err != nil {
			return err
		}
	}
	for _, d := range req.Dates {
		guardianAccountID := req.SubmittedBy
		if err := s.statusDayRepo.UpsertReported(ctx, &activeModels.StudentStatusDay{
			StudentID:         req.StudentID,
			Date:              d,
			Status:            req.AbsenceStatus,
			ReportedAt:        now,
			Source:            activeModels.StudentStatusSourceParent,
			GuardianAccountID: &guardianAccountID,
			Note:              notePtr,
		}); err != nil {
			return err
		}
	}

	if containsDate(req.Dates, s.todayDate()) {
		fresh, err := s.studentRepo.FindByIDForUpdate(ctx, req.StudentID)
		if err != nil {
			return err
		}
		if req.AbsenceStatus == activeModels.StudentStatusDaySick {
			trueVal := true
			falseVal := false
			fresh.Sick = &trueVal
			fresh.SickSince = &now
			fresh.Excused = &falseVal
			fresh.ExcusedSince = nil
		} else {
			falseVal := false
			fresh.Sick = &falseVal
			fresh.SickSince = nil
		}
		if err := s.studentRepo.Update(ctx, fresh); err != nil {
			return err
		}
	}
	return nil
}

func absenceRequestCopy(absenceStatus string) (created, confirmed, requestType string) {
	if absenceStatus == activeModels.StudentStatusDaySick {
		return sickRequestCreatedBody, sickRequestConfirmedBody, usersModels.ParentMessageRequestSickAbsence
	}
	return excusedRequestCreatedBody, excusedRequestConfirmedBody, usersModels.ParentMessageRequestExcusedAbsence
}

func absenceRequestRejectedBody(absenceStatus string) string {
	if absenceStatus == activeModels.StudentStatusDaySick {
		return sickRequestRejectedBody
	}
	return excusedRequestRejectedBody
}

// Before absence_status existed, every row represented an excused absence.
// The migration backfills persisted rows; this fallback keeps legacy in-memory
// repository implementations and zero-value test doubles compatible too.
func normalizedAbsenceRequestStatus(absenceStatus string) string {
	if absenceStatus == "" {
		return activeModels.StudentStatusDayExcused
	}
	return absenceStatus
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

// broadcastRequestTransition wakes BOTH audiences after any absence-request
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

// excusedScopeEnd is the last day this request covers. Empty dates mean no
// scope, which ParentRequestIsPast reads as "never past".
func excusedScopeEnd(req *activeModels.ExcusedAbsenceRequest) timezone.Date {
	var last timezone.Date
	for _, date := range req.Dates {
		if last.IsZero() || date.After(last) {
			last = date
		}
	}
	return last
}

// MarkDone closes a request whose days have all passed. Approving it would
// write absence days into the past and rejecting it would tell the family
// their wish was refused — neither is what happened, so this is its own
// terminal state (#2267, story 15).
//
// It applies nothing. The only writes are the status, the reviewer stamp and
// the neutral pill.
func (s *excusedAbsenceRequestService) MarkDone(
	ctx context.Context,
	requestID int64,
	expectedVersion, reason string,
	reviewedBy int64,
) error {
	if requestID <= 0 {
		return activeModels.ErrExcusedRequestNotFound
	}
	req, err := s.requestRepo.FindPendingByIDForUpdate(ctx, requestID)
	if err != nil {
		return err
	}
	if expectedVersion != "" && usersService.ParentRequestVersion(req.UpdatedAt) != expectedVersion {
		return usersService.ErrParentRequestStale
	}
	student, err := s.studentRepo.FindByIDForUpdate(ctx, req.StudentID)
	if err != nil {
		return fmt.Errorf("active: load student for absence request completion: %w", err)
	}
	allowed, authErr := s.canReviewStudent(ctx, student)
	if authErr != nil {
		return authErr
	}
	if !allowed {
		return ErrExcusedRequestForbidden
	}
	if !usersService.ParentRequestIsPast(excusedScopeEnd(req), s.todayDate()) {
		return usersService.ErrParentRequestNotPast
	}
	trimmed := strings.TrimSpace(reason)
	if utf8.RuneCountInString(trimmed) > excusedRequestMaxReasonLen {
		return ErrExcusedRequestRejectReasonTooLong
	}
	var reasonPtr *string
	if trimmed != "" {
		reasonPtr = &trimmed
	}
	reviewer := reviewedBy
	if err := s.requestRepo.Decide(
		ctx, req.ID, activeModels.ExcusedRequestStatusDone, reasonPtr, &reviewer, false,
	); err != nil {
		return err
	}
	if err := usersService.RecordParentRequestEvent(ctx, s.events, usersService.ParentRequestEventInput{
		StudentID:      req.StudentID,
		RequestType:    usersModels.ParentRequestTypeExcusedAbsence,
		RequestID:      req.ID,
		EventType:      usersModels.ParentRequestEventMarkedDone,
		ActorAccountID: reviewedBy,
		UpdatedAt:      req.UpdatedAt,
		Payload:        map[string]any{"reason": trimmed},
	}); err != nil {
		return fmt.Errorf("active: record absence marked-done event: %w", err)
	}
	s.broadcastRequestTransition(ctx, req.TenantID, req.StudentID)
	_, _, requestType := absenceRequestCopy(req.AbsenceStatus)
	s.emitRequestPillAfterCommit(ctx, req, parentmessaging.ChildEvent{
		EventType:      usersModels.ParentMessageEventRequestStatus,
		ActorKind:      usersModels.ParentMessageSenderStaff,
		ActorAccountID: reviewedBy,
		Body:           parentRequestDoneBody,
		RequestType:    requestType,
		RequestStatus:  usersModels.ParentMessageRequestStatusDone,
		DecisionReason: trimmed,
	})
	return nil
}

// Correct rewrites a decision staff already took (#2267, stories 21-23). The
// old decision is not erased — the ledger keeps it — but the child's record is
// brought in line with the new verdict.
//
// approved → rejected has to UNDO the absence days this request wrote, and it
// only does so when it can prove the days on record are still the ones this
// request wrote: same status, parent source, same submitting guardian, and
// untouched since the approval. If anything moved, the correction refuses
// rather than clearing somebody else's newer entry.
//
// rejected → approved re-runs the ordinary approve path, so every guard that
// applies to a fresh approval applies again.
func (s *excusedAbsenceRequestService) Correct(
	ctx context.Context,
	requestID int64,
	approve bool,
	expectedVersion, reason string,
	reviewedBy int64,
) error {
	if requestID <= 0 {
		return activeModels.ErrExcusedRequestNotFound
	}
	trimmed := strings.TrimSpace(reason)
	if utf8.RuneCountInString(trimmed) > excusedRequestMaxReasonLen {
		return ErrExcusedRequestRejectReasonTooLong
	}
	req, err := s.requestRepo.FindByIDForUpdate(ctx, requestID)
	if err != nil {
		return err
	}
	if !isCorrectableExcusedStatus(req.Status) {
		return usersService.ErrParentRequestNotDecided
	}
	if expectedVersion != "" && usersService.ParentRequestVersion(req.UpdatedAt) != expectedVersion {
		return usersService.ErrParentRequestStale
	}
	req.AbsenceStatus = normalizedAbsenceRequestStatus(req.AbsenceStatus)
	// Re-authorize against the CURRENT policy: a group leader who has since
	// lost the group must not be able to correct its decisions.
	student, err := s.studentRepo.FindByIDForUpdate(ctx, req.StudentID)
	if err != nil {
		return fmt.Errorf("active: load student for absence request correction: %w", err)
	}
	allowed, authErr := s.canReviewStudent(ctx, student)
	if authErr != nil {
		return authErr
	}
	if !allowed {
		return ErrExcusedRequestForbidden
	}
	if usersService.ParentRequestIsPast(excusedScopeEnd(req), s.todayDate()) {
		// Correcting into an approval would write absence days into a settled
		// past, exactly as a fresh approval would.
		if approve {
			return usersService.ErrParentRequestPast
		}
	}
	if approve {
		if err := s.applyAbsenceRequest(ctx, req); err != nil {
			return err
		}
	} else if err := s.revertApprovedAbsence(ctx, req); err != nil {
		return err
	}

	var reasonPtr *string
	if trimmed != "" {
		reasonPtr = &trimmed
	}
	newStatus := activeModels.ExcusedRequestStatusRejected
	if approve {
		newStatus = activeModels.ExcusedRequestStatusApproved
	}
	if err := s.requestRepo.Redecide(ctx, req.ID, newStatus, reasonPtr, reviewedBy, approve); err != nil {
		return err
	}
	// The ledger keeps BOTH decisions: the correction is a new entry naming
	// what it replaced, never an edit of the old one. Written inside the
	// correction's own transaction, so a rollback leaves no trace of it.
	if err := usersService.RecordParentRequestEvent(ctx, s.events, usersService.ParentRequestEventInput{
		StudentID:      req.StudentID,
		RequestType:    usersModels.ParentRequestTypeExcusedAbsence,
		RequestID:      req.ID,
		EventType:      usersModels.ParentRequestEventCorrected,
		ActorAccountID: reviewedBy,
		UpdatedAt:      req.UpdatedAt,
		Payload:        correctionEventPayload(approve, trimmed, req.Status, newStatus, req.ReviewedBy, req.DecisionReason),
	}); err != nil {
		return fmt.Errorf("active: record absence correction event: %w", err)
	}
	s.broadcastRequestTransition(ctx, req.TenantID, req.StudentID)
	pillBody, pillStatus := correctedAbsencePill(req.AbsenceStatus, approve)
	_, _, requestType := absenceRequestCopy(req.AbsenceStatus)
	s.emitRequestPillAfterCommit(ctx, req, parentmessaging.ChildEvent{
		EventType:      usersModels.ParentMessageEventRequestStatus,
		ActorKind:      usersModels.ParentMessageSenderStaff,
		ActorAccountID: reviewedBy,
		Body:           pillBody,
		RequestType:    requestType,
		RequestStatus:  pillStatus,
		DecisionReason: trimmed,
	})
	return nil
}

func isCorrectableExcusedStatus(status string) bool {
	return status == activeModels.ExcusedRequestStatusApproved ||
		status == activeModels.ExcusedRequestStatusRejected
}

// correctedAbsencePill names the new outcome for the family. The wording says
// the decision changed, because "bestätigt" alone after a rejection reads like
// a duplicate message rather than a correction.
func correctedAbsencePill(absenceStatus string, approve bool) (string, string) {
	if approve {
		_, confirmed, _ := absenceRequestCopy(absenceStatus)
		return "Entscheidung geändert: " + confirmed, usersModels.ParentMessageRequestStatusDone
	}
	return "Entscheidung geändert: " + absenceRequestRejectedBody(absenceStatus),
		usersModels.ParentMessageRequestStatusRejected
}

// revertApprovedAbsence clears exactly the days this request wrote, and only
// while they are provably still its own. A day that was re-reported after the
// approval belongs to whoever wrote it last; clearing it would silently drop
// their entry, so the correction refuses instead and names why.
func (s *excusedAbsenceRequestService) revertApprovedAbsence(
	ctx context.Context,
	req *activeModels.ExcusedAbsenceRequest,
) error {
	if req.Status != activeModels.ExcusedRequestStatusApproved || len(req.Dates) == 0 {
		return nil
	}
	rows, err := s.statusDayRepo.FindActiveByStudentAndDateRange(
		ctx, req.StudentID, req.Dates[0], req.Dates[len(req.Dates)-1],
	)
	if err != nil {
		return err
	}
	requested := make(map[timezone.Date]struct{}, len(req.Dates))
	for _, date := range req.Dates {
		requested[date] = struct{}{}
	}
	for _, row := range rows {
		if _, ok := requested[row.Date]; !ok || row.Status != req.AbsenceStatus {
			continue
		}
		if !absenceRowWrittenByRequest(row, req) {
			return fmt.Errorf("%w: der Eintrag für den %s wurde nach der Entscheidung geändert",
				usersService.ErrParentRequestCorrectionUnsupported, row.Date.Format("02.01.2006"))
		}
	}
	return s.statusDayRepo.MarkClearedForDates(
		ctx, req.StudentID, req.AbsenceStatus, req.Dates, time.Now(),
		activeModels.StudentStatusSourceParent,
	)
}

// absenceRowWrittenByRequest reports whether this status day is still the one
// the request's approval produced: parent-sourced, from the same guardian, and
// not re-reported since the decision.
func absenceRowWrittenByRequest(
	row *activeModels.StudentStatusDay,
	req *activeModels.ExcusedAbsenceRequest,
) bool {
	if row.Source != activeModels.StudentStatusSourceParent {
		return false
	}
	if row.GuardianAccountID == nil || *row.GuardianAccountID != req.SubmittedBy {
		return false
	}
	// reviewed_at is when the approval happened; anything reported after it is
	// a later entry, whoever made it.
	return req.ReviewedAt == nil || !row.ReportedAt.After(*req.ReviewedAt)
}

// notifyOtherGuardiansAfterCommit posts the neutral „Betreuungsstand
// geändert“ line to every OTHER portal guardian of the child (#2267, story
// 47). The submitting guardian already has the full pill; the others learn
// that the care changed and on which days, without the reason or the author.
//
// The guardians are resolved INSIDE the transaction (the caller's tenant scope
// applies); only the pills are posted after commit.
func (s *excusedAbsenceRequestService) notifyOtherGuardiansAfterCommit(
	ctx context.Context,
	req *activeModels.ExcusedAbsenceRequest,
	ev parentmessaging.ChildEvent,
) {
	if s.emitter == nil {
		return
	}
	// WHO hears what is a tenant-scoped read and has to happen here, inside
	// the transaction; the pills go out after it commits.
	audience, err := s.emitter.ResolveDecisionAudience(
		ctx, req.StudentID, req.SubmittedBy, s.sharedRecipients(ctx, req),
	)
	if err != nil {
		s.logger.Warn("co-guardian notice: resolving guardians failed",
			slog.Int64("request_id", req.ID),
			slog.Int64("student_id", req.StudentID),
			slog.String("error", err.Error()),
		)
		return
	}
	if len(audience.Full) == 0 && len(audience.Neutral) == 0 {
		return
	}
	tenantID := req.TenantID
	if tenantID <= 0 {
		tenantID = tenant.FromContext(ctx)
	}
	neutral := ev
	neutral.Body = coGuardianNoticeBody(req)
	studentID := req.StudentID
	tenant.RegisterAfterCommit(ctx, func() {
		s.emitter.EmitDecisionAudience(tenantID, studentID, audience, ev, neutral)
	})
}

// coGuardianNoticeBody names WHAT changed and WHEN, and nothing else. The date
// range is what makes the line actionable — a co-guardian can check whether it
// affects a day they were going to collect the child.
func coGuardianNoticeBody(req *activeModels.ExcusedAbsenceRequest) string {
	kind := "Abmeldung"
	if req.AbsenceStatus == activeModels.StudentStatusDaySick {
		kind = "Krankmeldung"
	}
	return "Betreuungsstand geändert: " + kind + " " + absenceDateRangeLabel(req.Dates)
}

// absenceDateRangeLabel renders one day as a date and several as a range —
// the shortest form that still says which days are affected.
func absenceDateRangeLabel(dates []timezone.Date) string {
	if len(dates) == 0 {
		return ""
	}
	first := dates[0].Format("02.01.2006")
	if len(dates) == 1 {
		return first
	}
	return first + " bis " + dates[len(dates)-1].Format("02.01.2006")
}

// correctionEventPayload is the local spelling of the shared ledger payload,
// kept so the call site above stays one line.
func correctionEventPayload(
	approve bool, reason, fromStatus, toStatus string, priorReviewer *int64, priorReason *string,
) map[string]any {
	return usersService.CorrectionEventPayload(approve, reason, fromStatus, toStatus, priorReviewer, priorReason)
}

// RequestShareVisibilityResolver is the shared port, aliased so this package
// reads naturally and the factory can name either spelling.
type RequestShareVisibilityResolver = parentmessaging.ShareVisibilityResolver

// SetRequestShareVisibility wires the resolver after construction.
func (s *excusedAbsenceRequestService) SetRequestShareVisibility(resolver RequestShareVisibilityResolver) {
	if s != nil {
		s.shareVisibility = resolver
	}
}

// sharedRecipients resolves the explicit recipients of one request, tolerating
// an unwired resolver. An error is not fatal: the fallback is that everyone
// gets the neutral line, which is the safe direction — a co-guardian seeing
// less than they were entitled to is a nuisance, seeing more is a leak.
func (s *excusedAbsenceRequestService) sharedRecipients(
	ctx context.Context, req *activeModels.ExcusedAbsenceRequest,
) []int64 {
	if s.shareVisibility == nil {
		return nil
	}
	accountIDs, err := s.shareVisibility.SharedRecipientAccountIDs(
		ctx, req.StudentID, usersModels.ParentRequestTypeExcusedAbsence, req.ID,
	)
	if err != nil {
		s.logger.Warn("resolving explicit request recipients failed, falling back to neutral notices",
			slog.Int64("request_id", req.ID),
			slog.Int64("student_id", req.StudentID),
			slog.String("error", err.Error()),
		)
		return nil
	}
	return accountIDs
}

// The factory wires the co-guardian resolver by type assertion, so a service
// that silently stopped satisfying this setter would leave its domain's
// co-guardians hearing nothing, with nothing failing. This makes that a
// compile error instead (#2267, story 47).
var _ interface {
	SetRequestShareVisibility(parentmessaging.ShareVisibilityResolver)
} = (*excusedAbsenceRequestService)(nil)
