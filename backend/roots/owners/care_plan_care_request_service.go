package owners

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	eventContract "github.com/moto-nrw/project-phoenix/modules/communication/eventcontract"

	peopleValues "github.com/moto-nrw/project-phoenix/modules/peopledirectory"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/carerequests"
	"github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	"github.com/moto-nrw/project-phoenix/realtime"

	peoplePorts "github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
	timezone "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

type CareScheduleRequestService struct {
	RequestRecords    RequestRecords
	People            RequestPeople
	Arrival           RequestArrivalPlans
	Pickup            RequestPickupPlans
	Attendance        PickupChangePresence
	PickupAutoExcusal careplan.PickupAutoExcusal
	DayLocker         careplan.CareDayLocker
	UserContext       CareRequestStaff
	StudentAudit      requestStudentAudit
	Logger            *slog.Logger
	ReviewPolicy      RequestReviewPolicy
	// PickupPermissionCommands writes the pickup permission an approved
	// pickup permission request grants (#4780).
	PickupPermissionCommands careplan.GuardianPickupPermissions
	// PickupPermissions serves the pickup permission requests from the native
	// submissions and reads (#4780).
	carerequests.PickupPermissions
	// permissions reads the permissions of the request claims the review
	// policy decides on.
	Permissions func(context.Context) []string
	Today       func() timezone.Date

	// requests holds the native request capabilities, bound once at construction.
	Requests requestCapabilities
}

// careRequestOptions are the optional collaborators of the care request
// service.
type careRequestOptions struct {
	today func() timezone.Date
	// shares answers who the parent explicitly shared a request with; nil
	// means nobody was, so every co-guardian gets the neutral line.
	shares eventContract.ShareVisibilityResolver
	// service applies settings that bind further collaborators to the built
	// service, like the pickup permission commands (#4780).
	service []func(*CareScheduleRequestService)
}

// CareRequestOption configures an optional collaborator of the care request
// service at construction.
type CareRequestOption func(*careRequestOptions)

// WithCareRequestToday pins the calendar day the request window starts from.
// Without it the service reads the Berlin wall clock.
func WithCareRequestToday(today func() timezone.Date) CareRequestOption {
	return func(o *careRequestOptions) { o.today = today }
}

// WithPickupPermissions wires Care Plan's pickup permission commands an
// approved pickup permission request writes through (#4780).
func WithPickupPermissions(permissions careplan.GuardianPickupPermissions) CareRequestOption {
	return func(o *careRequestOptions) {
		o.service = append(o.service, func(s *CareScheduleRequestService) { s.PickupPermissionCommands = permissions })
	}
}

// WithRequestShareVisibility wires the sharing resolver Care Plan's
// co-guardian notices (#2267) read. The composition root passes a resolver
// that reaches the later-built parents service lazily.
func WithRequestShareVisibility(resolver eventContract.ShareVisibilityResolver) CareRequestOption {
	return func(o *careRequestOptions) { o.shares = resolver }
}

func (s *CareScheduleRequestService) todayDate() timezone.Date {
	if s.Today != nil {
		return s.Today()
	}
	return timezone.TodayDate()
}

// requestStudentAudit is the People Directory audit a weekly approval
// records its departure-mode change through.
type requestStudentAudit interface {
	RecordChangesForActor(ctx context.Context, before, after *peopleValues.StudentState, editedBy int64) error
}

type RequestReviewPolicy interface {
	Allows(context.Context, []string, *peopleValues.StudentState) (bool, error)
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
	userContext CareRequestStaff,
	emitter eventContract.Emitter,
	broadcaster realtime.Broadcaster,
	reviewPolicy RequestReviewPolicy,
	permissions func(context.Context) []string,
	events peopleValues.ParentRequestEvents,
	logger *slog.Logger,
	studentAudit peoplePorts.StudentChangeRecorder,
	options ...CareRequestOption,
) carerequests.Service {
	if reviewPolicy == nil || permissions == nil {
		panic("care schedule request review policy and request permissions are required")
	}
	if requestRecords == nil {
		panic("care schedule request records are required")
	}
	if logger == nil {
		logger = slog.Default()
	}
	var config careRequestOptions
	for _, option := range options {
		option(&config)
	}
	svc := &CareScheduleRequestService{
		RequestRecords: requestRecords, People: people, Arrival: arrival, Pickup: pickup,
		Attendance: attendance, PickupAutoExcusal: pickupAutoExcusal, DayLocker: dayLocker,
		UserContext: userContext, StudentAudit: studentAudit, Logger: logger,
		ReviewPolicy: reviewPolicy, Permissions: permissions, Today: config.today,
	}
	for _, apply := range config.service {
		apply(svc)
	}
	ledger := NewParentRequestLedger(events)
	messenger, effects, err := newCareRequestEffects(svc, emitter, broadcaster, ledger, config.shares)
	if err != nil {
		panic(fmt.Errorf("care request effects: %w", err))
	}
	requests, err := newRequestCapabilities(svc, messenger, effects)
	if err != nil {
		panic(err)
	}
	svc.Requests = requests
	svc.PickupPermissions = struct {
		carerequests.Submissions
		carerequests.Reads
	}{requests.submissions, requests.reads}
	return svc
}

type requestCapabilities struct {
	diffs       carerequests.Diffs
	reads       carerequests.Reads
	submissions carerequests.Submissions
	edits       carerequests.Edits
	decisions   carerequests.Decisions
	corrections carerequests.Corrections
	conflicts   carerequests.Conflicts
}

// newRequestCapabilities builds the native request capabilities once, in
// dependency order, so a wiring gap fails at startup and not on a request.
// The messenger, when parent messaging exists, also answers a decision's
// guardian check; the ledger serves a correction's decision history.
func newRequestCapabilities(s *CareScheduleRequestService, messenger compose.RequestMessenger, effects compose.CareRequestEffects) (requestCapabilities, error) {
	var c requestCapabilities
	var err error
	diffAdapter := requestDiffAdapter{s}
	if c.diffs, err = compose.NewRequestDiffs(diffAdapter, diffAdapter, diffAdapter, s.Logger); err != nil {
		return c, fmt.Errorf("care request diffs: %w", err)
	}
	if c.reads, err = compose.NewRequestReads(s.RequestRecords, c.diffs, s.Logger); err != nil {
		return c, fmt.Errorf("care request reads: %w", err)
	}
	submissionAdapter := requestSubmissionAdapter{s}
	if c.submissions, err = compose.NewRequestSubmissions(s.RequestRecords, submissionAdapter, effects, s.todayDate); err != nil {
		return c, fmt.Errorf("care request submissions: %w", err)
	}
	editAdapter := requestEditAdapter{submissionAdapter}
	if c.edits, err = compose.NewRequestEdits(s.RequestRecords, editAdapter, effects, s.todayDate); err != nil {
		return c, fmt.Errorf("care request edits: %w", err)
	}
	decisionAdapter := requestDecisionAdapter{editAdapter}
	if c.decisions, err = compose.NewRequestDecisions(compose.RequestDecisionDependencies{Records: s.RequestRecords, People: decisionAdapter,
		Guardians: messenger, Plans: decisionAdapter, Effects: effects, Today: s.todayDate}); err != nil {
		return c, fmt.Errorf("care request decisions: %w", err)
	}
	if c.corrections, err = compose.NewRequestCorrections(compose.RequestCorrectionDependencies{Records: s.RequestRecords, People: decisionAdapter,
		Plans: decisionAdapter, Effects: effects}); err != nil {
		return c, fmt.Errorf("care request corrections: %w", err)
	}
	if c.conflicts, err = compose.NewRequestConflicts(s.RequestRecords, decisionAdapter, requestConflictPlans{s}, c.decisions, s.todayDate); err != nil {
		return c, fmt.Errorf("care request conflicts: %w", err)
	}
	return c, nil
}

// newCareRequestEffects binds Care Plan's request effects to the owners they
// report to: Communication's parent events, People Directory's request
// ledger, the realtime hub and the parent's sharing choice. A sink that does
// not exist stays unbound and its effect is skipped.
func newCareRequestEffects(
	s *CareScheduleRequestService,
	emitter eventContract.Emitter,
	broadcaster realtime.Broadcaster,
	ledger *ParentRequestLedger,
	shares eventContract.ShareVisibilityResolver,
) (compose.RequestMessenger, compose.CareRequestEffects, error) {
	ports := compose.CareRequestEffectPorts{Records: s.RequestRecords, Students: requestSubmissionAdapter{s}, Shares: shares, Logger: s.Logger}
	var messenger compose.RequestMessenger
	if emitter != nil {
		messenger = NewExcusedRequestMessenger(emitter)
		ports.Messenger = messenger
	}
	if broadcaster != nil {
		ports.Broadcaster = careRequestBroadcaster{broadcaster: broadcaster}
	}
	if ledger != nil {
		ports.Ledger = ledger
	}
	effects, err := compose.NewCareRequestEffects(ports)
	return messenger, effects, err
}

// careRequestBroadcaster wakes staff tabs through the realtime hub after an
// approved care request.
type careRequestBroadcaster struct{ broadcaster realtime.Broadcaster }

const careRequestBroadcastSource = "parent_request"

func (b careRequestBroadcaster) StudentUpdated(tenantID int64) error {
	return b.broadcaster.BroadcastToTenant(tenantID, careRequestEvent(realtime.EventStudentUpdated))
}

func (b careRequestBroadcaster) StudentCompanionsChanged(tenantID int64) error {
	return b.broadcaster.BroadcastToTenant(tenantID, careRequestEvent(realtime.EventStudentCompanionsChanged))
}

func (b careRequestBroadcaster) ArrivalScheduleChanged() error {
	return b.broadcaster.BroadcastToAll(careRequestEvent(realtime.EventArrivalScheduleChanged))
}

func careRequestEvent(eventType realtime.EventType) realtime.Event {
	source := careRequestBroadcastSource
	return realtime.NewEvent(eventType, "", realtime.EventData{Source: &source})
}

func (s *CareScheduleRequestService) CreateRequest(ctx context.Context, studentID, guardianAccountID int64, payload json.RawMessage) (*carerequests.Request, error) {
	return s.Requests.submissions.CreateRequest(ctx, studentID, guardianAccountID, payload)
}

// CreatePickupChangeRequest is the legacy mandatory-reason entry point; the
// owner names its input.
func (s *CareScheduleRequestService) CreatePickupChangeRequest(ctx context.Context, studentID, guardianAccountID int64, date timezone.Date, pickupTime time.Time, reason string) (*carerequests.Request, error) {
	return s.CreatePickupChange(ctx, carerequests.MandatoryReasonPickupChange(studentID, guardianAccountID, date, pickupTime, reason))
}

func (s *CareScheduleRequestService) CreatePickupChange(ctx context.Context, input carerequests.PickupChangeCreateInput) (*carerequests.Request, error) {
	return s.Requests.submissions.CreatePickupChange(ctx, input)
}

func (s *CareScheduleRequestService) EditRequest(ctx context.Context, input carerequests.EditInput) (*carerequests.Request, error) {
	request, err := s.Requests.edits.EditRequest(ctx, input)
	if err != nil {
		return nil, legacyEditError(err)
	}
	return request, nil
}

func (s *CareScheduleRequestService) Decide(ctx context.Context, input carerequests.DecideInput) (*carerequests.ReviewItem, error) {
	item, err := s.Requests.decisions.Decide(ctx, input)
	if err != nil {
		return nil, legacyEditError(err)
	}
	return item, nil
}

func (s *CareScheduleRequestService) canReviewStudent(ctx context.Context, student *peopleValues.StudentState) (bool, error) {
	if s.ReviewPolicy == nil {
		return false, errors.New("care request review policy is required")
	}
	ok, err := s.ReviewPolicy.Allows(ctx, s.Permissions(ctx), student)
	if err != nil {
		return false, fmt.Errorf("schedule: resolve request reviewer scope: %w", err)
	}
	return ok, nil
}

func (s *CareScheduleRequestService) applyPickupChangeRequest(ctx context.Context, req *carerequests.Request, expectedImpactToken *string, requireImpactToken bool) (carerequests.PickupApplied, error) {
	if s.RequestRecords == nil || s.Attendance == nil || s.PickupAutoExcusal == nil || s.UserContext == nil || s.DayLocker == nil {
		return carerequests.PickupApplied{}, errors.New("schedule: pickup change request dependencies not configured")
	}
	approvals, err := s.pickupApprovals()
	if err != nil {
		return carerequests.PickupApplied{}, err
	}
	return approvals.ApplyPickupApproval(ctx, carerequests.PickupApproval{
		TenantID: req.TenantID, StudentID: req.StudentID, Payload: req.Payload,
		ExpectedImpactToken: expectedImpactToken, RequireImpactToken: requireImpactToken,
	})
}

func (s *CareScheduleRequestService) MarkDone(ctx context.Context, requestID int64, expectedVersion, reason string, reviewedBy int64) error {
	return legacyEditError(s.Requests.decisions.MarkDone(ctx, requestID, expectedVersion, reason, reviewedBy))
}
