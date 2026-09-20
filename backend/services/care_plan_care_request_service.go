// Care request bindings connect native Care Plan commands to identity, audit,
// realtime delivery, and parent notifications at the composition root.
package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/carerequests"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/legacy/jwt"
	userContextService "github.com/moto-nrw/project-phoenix/modules/identityaccess/legacy/usercontext"
	"github.com/moto-nrw/project-phoenix/realtime"
	"github.com/moto-nrw/project-phoenix/services/parentmessaging"
	usersService "github.com/moto-nrw/project-phoenix/services/users"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// German pill texts. The staff portal renders these directly; the parents
// portal localizes from the structured event fields instead.
const (
	careRequestCreatedBody     = "Anfrage: Dauerhafte Betreuungszeiten ändern"
	careRequestConfirmedBody   = "Anfrage bestätigt, Betreuungszeiten übernommen"
	pickupRequestCreatedBody   = "Abholzeit angefragt"
	pickupRequestConfirmedBody = "Abholzeit bestätigt"
	pickupRequestRejectedBody  = "Abholzeit abgelehnt"
)

// pickupPillTerms reads the requested day and time of a pickup-change
// request. It is deliberately lenient about everything else in the payload:
// the reason may be blank under a school's reason policy, and a pill that
// cannot name the day must fall back to the plain body rather than fail.
func pickupPillTerms(req *carerequests.Request) (timezone.Date, time.Time, bool) {
	if req == nil || req.RequestKind != "pickup_change" {
		return timezone.Date(""), time.Time{}, false
	}
	return carerequests.PickupTerms(req.Payload)
}

// germanDateLayout renders calendar dates as dd.mm.yyyy in user-facing copy.
// The moved timetable services keep their own copy (#3218).
const germanDateLayout = "02.01.2006"

// pickupPillDetail names the requested day and time of a pickup change the
// way the staff thread shows it ("15.09.2026, 14:30 Uhr"), or "" when the
// payload does not carry both. The pill names the pickup appointment; the
// message timestamp stays the moment of the event (#3135).
func pickupPillDetail(req *carerequests.Request) string {
	date, pickup, ok := pickupPillTerms(req)
	if !ok {
		return ""
	}
	return date.Format(germanDateLayout) + ", " + pickup.Format("15:04") + " Uhr"
}

// withPickupDetail appends the requested day and time to a pickup pill body.
// Weekly-plan requests and undecodable payloads keep the body unchanged.
func withPickupDetail(body string, req *carerequests.Request) string {
	if detail := pickupPillDetail(req); detail != "" {
		return body + ": " + detail
	}
	return body
}

// pickupPillPayload is the structured twin of pickupPillDetail: the requested
// calendar day, the requested time and, when the request recorded one, the
// pickup time that applied before. Clients that localize render from it; the
// German body stays authoritative for the staff portal. nil for weekly-plan
// requests and undecodable payloads.
func pickupPillPayload(req *carerequests.Request) map[string]any {
	terms := pickupChangeTerms(req)
	if terms == nil {
		return nil
	}
	payload := map[string]any{
		"date":        terms.Date.String(),
		"pickup_time": terms.PickupTime,
	}
	if terms.PreviousPickupTime != "" {
		payload["previous_pickup_time"] = terms.PreviousPickupTime
	}
	return payload
}

// careRequestPillType maps a request kind to the pill's request_type token.
// The German bodies above are authoritative only for the staff portal; the
// localized parents portal and the decision push render from this token, so a
// one-day pickup change must not travel as a permanent care_schedule change.
func careRequestPillType(requestKind string) string {
	if requestKind == "pickup_change" {
		return usersModels.ParentMessageRequestPickupChange
	}
	return usersModels.ParentMessageRequestCareSchedule
}

func pickupChangeTerms(req *carerequests.Request) *carerequests.PickupChangeTerms {
	if req == nil || req.RequestKind != "pickup_change" {
		return nil
	}
	return carerequests.StoredPickupTerms(req.Payload)
}

type careScheduleRequestService struct {
	requestRecords    RequestRecords
	people            RequestPeople
	arrival           RequestArrivalPlans
	pickup            RequestPickupPlans
	attendance        PickupChangePresence
	pickupAutoExcusal careplan.PickupAutoExcusal
	dayLocker         careplan.CareDayLocker
	userContext       userContextService.UserContextService
	emitter           *parentmessaging.Emitter
	broadcaster       realtime.Broadcaster
	studentAudit      usersService.StudentChangeRecorder
	logger            *slog.Logger
	reviewPolicy      RequestReviewPolicy
	// shareVisibility answers who the parent explicitly shared a request
	// with; nil means nobody was, so every co-guardian gets the neutral line.
	shareVisibility parentmessaging.ShareVisibilityResolver
	events          usersService.ParentRequestEventRecorder
	today           func() timezone.Date
}

// CareRequestOption configures an optional collaborator of the care request
// service at construction.
type CareRequestOption func(*careScheduleRequestService)

// WithCareRequestToday pins the calendar day the request window starts from.
// Without it the service reads the Berlin wall clock.
func WithCareRequestToday(today func() timezone.Date) CareRequestOption {
	return func(s *careScheduleRequestService) { s.today = today }
}

func (s *careScheduleRequestService) todayDate() timezone.Date {
	if s.today != nil {
		return s.today()
	}
	return timezone.TodayDate()
}

type RequestReviewPolicy interface {
	Allows(context.Context, []string, *usersModels.Student) (bool, error)
}

// NewCareScheduleRequestServiceWithPickupChangesAndPolicy requires the
// production review policy at construction, so missing wiring cannot widen
// reviewer access.
func NewCareScheduleRequestServiceWithPickupChangesAndPolicy(
	requestRecords RequestRecords,
	people RequestPeople,
	arrival RequestArrivalPlans,
	pickup RequestPickupPlans,
	attendance PickupChangePresence,
	pickupAutoExcusal careplan.PickupAutoExcusal,
	dayLocker careplan.CareDayLocker,
	userContext userContextService.UserContextService,
	emitter *parentmessaging.Emitter,
	broadcaster realtime.Broadcaster,
	reviewPolicy RequestReviewPolicy,
	events usersService.ParentRequestEventRecorder,
	logger *slog.Logger,
	studentAudit usersService.StudentChangeRecorder,
	options ...CareRequestOption,
) carerequests.Service {
	if reviewPolicy == nil {
		panic("care schedule request review policy is required")
	}
	svc := newCareScheduleRequestService(
		requestRecords, people, arrival, pickup, userContext,
		emitter, broadcaster, reviewPolicy, events, logger, studentAudit,
	)
	svc.attendance = attendance
	svc.pickupAutoExcusal = pickupAutoExcusal
	svc.dayLocker = dayLocker
	for _, option := range options {
		option(svc)
	}
	return svc
}

func newCareScheduleRequestService(
	requestRecords RequestRecords,
	people RequestPeople,
	arrival RequestArrivalPlans,
	pickup RequestPickupPlans,
	userContext userContextService.UserContextService,
	emitter *parentmessaging.Emitter,
	broadcaster realtime.Broadcaster,
	reviewPolicy RequestReviewPolicy,
	events usersService.ParentRequestEventRecorder,
	logger *slog.Logger,
	studentAudit usersService.StudentChangeRecorder,
) *careScheduleRequestService {
	if requestRecords == nil {
		panic("care schedule request records are required")
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &careScheduleRequestService{
		requestRecords: requestRecords,
		people:         people,
		arrival:        arrival,
		pickup:         pickup,
		userContext:    userContext,
		emitter:        emitter,
		broadcaster:    broadcaster,
		studentAudit:   studentAudit,
		reviewPolicy:   reviewPolicy,
		events:         events,
		logger:         logger,
	}
}

func (s *careScheduleRequestService) CreateRequest(ctx context.Context, studentID, guardianAccountID int64, payload json.RawMessage) (*carerequests.Request, error) {
	return s.submissions().CreateRequest(ctx, studentID, guardianAccountID, payload)
}

// CreatePickupChangeRequest keeps the mandatory-reason behaviour every
// existing caller relies on. A caller that has resolved the school's reason
// policy uses CreatePickupChange instead.
func (s *careScheduleRequestService) CreatePickupChangeRequest(ctx context.Context, studentID, guardianAccountID int64, date timezone.Date, pickupTime time.Time, reason string) (*carerequests.Request, error) {
	return s.CreatePickupChange(ctx, carerequests.PickupChangeCreateInput{
		StudentID: studentID, GuardianAccountID: guardianAccountID,
		Date: date, PickupTime: pickupTime, Reason: reason, ReasonRequired: true,
	})
}

func (s *careScheduleRequestService) CreatePickupChange(ctx context.Context, input carerequests.PickupChangeCreateInput) (*carerequests.Request, error) {
	return s.submissions().CreatePickupChange(ctx, input)
}

func (s *careScheduleRequestService) EditRequest(ctx context.Context, input carerequests.EditInput) (*carerequests.Request, error) {
	request, err := s.edits().EditRequest(ctx, input)
	if err != nil {
		return nil, legacyEditError(err)
	}
	return request, nil
}

// careRequestLedgerType maps the two care request kinds onto the ledger's
// request types. The weekly plan and the one-day pickup change are separate
// request types everywhere else too, so the history reads the same.
func careRequestLedgerType(req *carerequests.Request) string {
	if req.RequestKind == "pickup_change" {
		return usersModels.ParentRequestTypePickupChange
	}
	return usersModels.ParentRequestTypeCareSchedule
}

// recordCareRequestEvent writes one ledger entry inside the ambient
// transaction of the change it describes.
func (s *careScheduleRequestService) recordCareRequestEvent(
	ctx context.Context,
	req *carerequests.Request,
	eventType string,
	actorAccountID int64,
	payload map[string]any,
) error {
	if err := usersService.RecordParentRequestEvent(ctx, s.events, usersService.ParentRequestEventInput{
		StudentID:      req.StudentID,
		RequestType:    careRequestLedgerType(req),
		RequestID:      req.ID,
		EventType:      eventType,
		ActorAccountID: actorAccountID,
		UpdatedAt:      req.UpdatedAt,
		Payload:        payload,
	}); err != nil {
		return fmt.Errorf("schedule: record care request event: %w", err)
	}
	return nil
}

func (s *careScheduleRequestService) Decide(ctx context.Context, input carerequests.DecideInput) (*carerequests.ReviewItem, error) {
	item, err := s.decisions().Decide(ctx, input)
	if err != nil {
		return nil, legacyEditError(err)
	}
	return item, nil
}

func (s *careScheduleRequestService) canReviewStudent(ctx context.Context, student *usersModels.Student) (bool, error) {
	if s.reviewPolicy == nil {
		return false, errors.New("care request review policy is required")
	}
	ok, err := s.reviewPolicy.Allows(ctx, jwt.PermissionsFromCtx(ctx), student)
	if err != nil {
		return false, fmt.Errorf("schedule: resolve request reviewer scope: %w", err)
	}
	return ok, nil
}

type careDecisionState struct {
	pillStatus string
	pillBody   string
	reason     string
}

func newCareDecisionState(req *carerequests.Request, approve bool, reason string) careDecisionState {
	body := careRequestConfirmedBody
	rejectedBody := "Anfrage abgelehnt: " + reason
	if req.RequestKind == "pickup_change" {
		body = withPickupDetail(pickupRequestConfirmedBody, req)
		// The staff thread must show WHICH day was refused, not only that
		// something was (#3135). Without a decodable day the generic line stays.
		if detail := pickupPillDetail(req); detail != "" {
			rejectedBody = pickupRequestRejectedBody + ": " + detail + ". Grund: " + reason
		}
	}
	if approve {
		return careDecisionState{usersModels.ParentMessageRequestStatusDone, body, reason}
	}
	return careDecisionState{usersModels.ParentMessageRequestStatusRejected, rejectedBody, reason}
}

func (s *careScheduleRequestService) registerCareDecisionEffects(ctx context.Context, req *carerequests.Request, input carerequests.DecideInput, state careDecisionState, companionsChanged bool) error {
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
	decisionPill := parentmessaging.ChildEvent{
		EventType: "request_status", ActorKind: usersModels.ParentMessageSenderStaff,
		ActorAccountID: input.ReviewedBy, Body: state.pillBody,
		RequestType: careRequestPillType(req.RequestKind), RequestStatus: state.pillStatus,
		DecisionReason: state.reason,
		Payload:        pickupPillPayload(req),
	}
	if err := s.emitRequestPillAfterCommit(ctx, req, decisionPill); err != nil {
		return err
	}
	// Every other guardian of this child hears about the decision: the full
	// pill for explicit share recipients, a neutral line for the rest.
	s.notifyOtherGuardiansAfterCommit(ctx, req, decisionPill)
	s.wakeGuardiansAfterCommit(ctx, req)
	return nil
}

func (s *careScheduleRequestService) reloadCareDecisionItem(ctx context.Context, requestID int64) (*carerequests.ReviewItem, error) {
	row, err := s.requestRow(ctx, requestID)
	if err != nil {
		return nil, fmt.Errorf("schedule: reload decided care request: %w", err)
	}
	item := &carerequests.ReviewItem{Request: row}
	if row.RequestKind == "pickup_change" {
		item.Reason = pickupChangeReason(row)
	}
	student, err := s.people.FindStudentRecord(ctx, row.StudentID)
	if err != nil {
		return item, nil
	}
	person, err := s.people.FindPerson(ctx, student.PersonID)
	if err == nil {
		item.FirstName, item.LastName = person.FirstName, person.LastName
	}
	return item, nil
}

func pickupChangeReason(req *carerequests.Request) *string {
	if req == nil {
		return nil
	}
	var payload struct {
		Reason string `json:"reason"`
	}
	if json.Unmarshal(req.Payload, &payload) != nil {
		return nil
	}
	reason := payload.Reason
	trimmed := strings.TrimSpace(reason)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func (s *careScheduleRequestService) applyPickupChangeRequest(ctx context.Context, req *carerequests.Request, expectedImpactToken *string, requireImpactToken bool) (int64, error) {
	if s.requestRecords == nil || s.attendance == nil || s.pickupAutoExcusal == nil || s.userContext == nil || s.dayLocker == nil {
		return 0, errors.New("schedule: pickup change request dependencies not configured")
	}
	approvals, err := s.pickupApprovals()
	if err != nil {
		return 0, err
	}
	id, err := approvals.ApplyPickupApproval(ctx, carerequests.PickupApproval{
		TenantID: req.TenantID, StudentID: req.StudentID, Payload: req.Payload,
		ExpectedImpactToken: expectedImpactToken, RequireImpactToken: requireImpactToken,
	})
	return id, err
}

func (s *careScheduleRequestService) resolvePickupChangeStaff(ctx context.Context) (*usersModels.Staff, error) {
	staff, err := s.userContext.GetCurrentStaff(ctx)
	if err != nil {
		if errors.Is(err, userContextService.ErrUserNotLinkedToStaff) || errors.Is(err, userContextService.ErrUserNotLinkedToPerson) {
			return nil, carerequests.ErrCareRequestForbidden
		}
		return nil, fmt.Errorf("schedule: resolve acting staff for pickup request: %w", err)
	}
	if staff == nil {
		return nil, carerequests.ErrCareRequestForbidden
	}
	return staff, nil
}

// emitRequestPillAfterCommit writes the durable decision intent in the ambient
// transaction, then schedules the best-effort chat pill after commit.
func (s *careScheduleRequestService) emitRequestPillAfterCommit(ctx context.Context, req *carerequests.Request, ev parentmessaging.ChildEvent) error {
	if s.emitter == nil {
		return nil
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
	if err := s.emitter.EnqueueRequestDecision(ctx, tenantID, studentID, guardianAccountID, ev); err != nil {
		return fmt.Errorf("schedule: enqueue care request decision: %w", err)
	}
	tenant.RegisterAfterCommit(ctx, func() {
		s.emitter.EmitChildEvent(tenantID, studentID, guardianAccountID, ev)
	})
	return nil
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
func (s *careScheduleRequestService) wakeGuardiansAfterCommit(ctx context.Context, req *carerequests.Request) {
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
func (s *careScheduleRequestService) recordApplyAudit(req *carerequests.Request, accountID int64) {
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
func careScheduleAuditSummary(payload json.RawMessage) string {
	var p carerequests.WeeklyChange
	if json.Unmarshal(payload, &p) != nil {
		return ""
	}
	days := make([]string, 0, len(p.Weekdays))
	for _, wd := range p.Weekdays {
		days = append(days, strconv.Itoa(wd.Weekday))
	}
	return "weekdays:" + strings.Join(days, ",")
}

// parentRequestDoneBody is the neutral close for a request whose days have
// passed: nothing was applied and nothing was refused.
const parentRequestDoneBody = "Anfrage abgeschlossen"

func (s *careScheduleRequestService) MarkDone(ctx context.Context, requestID int64, expectedVersion, reason string, reviewedBy int64) error {
	return legacyEditError(s.decisions().MarkDone(ctx, requestID, expectedVersion, reason, reviewedBy))
}

func correctedCarePillBody(req *carerequests.Request, approve bool) string {
	if approve {
		return withPickupDetail(pickupRequestConfirmedBody, req)
	}
	return withPickupDetail(pickupRequestRejectedBody, req)
}

// Co-guardian notices (#2267, story 47). A staff decision on one parent's
// request changes the child's care for the whole family; the other guardians
// used to hear nothing. See the excused twin for the full rationale — the
// split is identical, only the wording differs per request kind.

// WithRequestShareVisibility wires the sharing resolver. The composition root
// passes a resolver that reaches the later-built parents service lazily.
func WithRequestShareVisibility(resolver parentmessaging.ShareVisibilityResolver) CareRequestOption {
	return func(s *careScheduleRequestService) { s.shareVisibility = resolver }
}

// sharedRecipients resolves the explicit recipients of one request, tolerating
// an unwired resolver. An error is not fatal: the fallback is that everyone
// gets the neutral line, which is the safe direction.
func (s *careScheduleRequestService) sharedRecipients(
	ctx context.Context, req *carerequests.Request,
) []int64 {
	if s.shareVisibility == nil {
		return nil
	}
	accountIDs, err := s.shareVisibility.SharedRecipientAccountIDs(
		ctx, req.StudentID, careRequestLedgerType(req), req.ID,
	)
	if err != nil {
		s.logger.Warn("resolving explicit request recipients failed, falling back to neutral notices",
			"request_id", req.ID,
			"student_id", req.StudentID,
			"error", err.Error(),
		)
		return nil
	}
	return accountIDs
}

// notifyOtherGuardiansAfterCommit posts the decision to the child's other
// guardians: the full pill to whoever the parent shared the request with, the
// neutral line to everyone else. The audience is resolved inside the
// transaction (a tenant-scoped read); only the pills go out after commit.
func (s *careScheduleRequestService) notifyOtherGuardiansAfterCommit(
	ctx context.Context,
	req *carerequests.Request,
	ev parentmessaging.ChildEvent,
) {
	if s.emitter == nil {
		return
	}
	audience, err := s.emitter.ResolveDecisionAudience(
		ctx, req.StudentID, req.SubmittedBy, s.sharedRecipients(ctx, req),
	)
	if err != nil {
		s.logger.Warn("co-guardian notice: resolving guardians failed",
			"request_id", req.ID,
			"student_id", req.StudentID,
			"error", err.Error(),
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
	neutral.Body = careCoGuardianNoticeBody(req)
	// The neutral line names the day but nothing the parent did not share:
	// no requested time, no previous time (#3135).
	neutral.Payload = nil
	studentID := req.StudentID
	tenant.RegisterAfterCommit(ctx, func() {
		s.emitter.EmitDecisionAudience(tenantID, studentID, audience, ev, neutral)
	})
}

// careCoGuardianNoticeBody names WHAT changed and, for a pickup change, WHEN.
// A weekly plan has no single date, so it names the area alone — enough for a
// co-guardian to know to look, which is the whole purpose of the line.
func careCoGuardianNoticeBody(req *carerequests.Request) string {
	if req.RequestKind != "pickup_change" {
		return "Betreuungsstand geändert: Betreuungszeiten"
	}
	var payload struct {
		Date string `json:"date"`
	}
	if json.Unmarshal(req.Payload, &payload) != nil {
		return "Betreuungsstand geändert: Abholzeit"
	}
	if date, err := timezone.ParseDate(payload.Date); err == nil {
		return "Betreuungsstand geändert: Abholzeit " + date.Format("02.01.2006")
	}
	return "Betreuungsstand geändert: Abholzeit"
}
