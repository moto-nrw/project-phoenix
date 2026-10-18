// Care request bindings connect native Care Plan commands to identity, audit,
// realtime delivery, and parent notifications at the composition root.
package api

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

type careScheduleRequestService struct {
	requestRecords    RequestRecords
	people            RequestPeople
	arrival           RequestArrivalPlans
	pickup            RequestPickupPlans
	attendance        PickupChangePresence
	pickupAutoExcusal careplan.PickupAutoExcusal
	dayLocker         careplan.CareDayLocker
	userContext       CareRequestStaff
	studentAudit      requestStudentAudit
	logger            *slog.Logger
	reviewPolicy      RequestReviewPolicy
	// permissions reads the permissions of the request claims the review
	// policy decides on.
	permissions func(context.Context) []string
	today       func() timezone.Date

	// requests holds the native request capabilities, bound once at construction.
	requests requestCapabilities
}

// careRequestOptions are the optional collaborators of the care request
// service.
type careRequestOptions struct {
	today func() timezone.Date
	// shares answers who the parent explicitly shared a request with; nil
	// means nobody was, so every co-guardian gets the neutral line.
	shares eventContract.ShareVisibilityResolver
}

// CareRequestOption configures an optional collaborator of the care request
// service at construction.
type CareRequestOption func(*careRequestOptions)

// WithCareRequestToday pins the calendar day the request window starts from.
// Without it the service reads the Berlin wall clock.
func WithCareRequestToday(today func() timezone.Date) CareRequestOption {
	return func(o *careRequestOptions) { o.today = today }
}

// WithRequestShareVisibility wires the sharing resolver Care Plan's
// co-guardian notices (#2267) read. The composition root passes a resolver
// that reaches the later-built parents service lazily.
func WithRequestShareVisibility(resolver eventContract.ShareVisibilityResolver) CareRequestOption {
	return func(o *careRequestOptions) { o.shares = resolver }
}

func (s *careScheduleRequestService) todayDate() timezone.Date {
	if s.today != nil {
		return s.today()
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
	svc := &careScheduleRequestService{
		requestRecords: requestRecords, people: people, arrival: arrival, pickup: pickup,
		attendance: attendance, pickupAutoExcusal: pickupAutoExcusal, dayLocker: dayLocker,
		userContext: userContext, studentAudit: studentAudit, logger: logger,
		reviewPolicy: reviewPolicy, permissions: permissions, today: config.today,
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
	svc.requests = requests
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
func newRequestCapabilities(s *careScheduleRequestService, messenger compose.RequestMessenger, effects compose.CareRequestEffects) (requestCapabilities, error) {
	var c requestCapabilities
	var err error
	diffAdapter := requestDiffAdapter{s}
	if c.diffs, err = compose.NewRequestDiffs(diffAdapter, diffAdapter, diffAdapter, s.logger); err != nil {
		return c, fmt.Errorf("care request diffs: %w", err)
	}
	if c.reads, err = compose.NewRequestReads(s.requestRecords, c.diffs, s.logger); err != nil {
		return c, fmt.Errorf("care request reads: %w", err)
	}
	submissionAdapter := requestSubmissionAdapter{s}
	if c.submissions, err = compose.NewRequestSubmissions(s.requestRecords, submissionAdapter, effects, s.todayDate); err != nil {
		return c, fmt.Errorf("care request submissions: %w", err)
	}
	editAdapter := requestEditAdapter{submissionAdapter}
	if c.edits, err = compose.NewRequestEdits(s.requestRecords, editAdapter, effects, s.todayDate); err != nil {
		return c, fmt.Errorf("care request edits: %w", err)
	}
	decisionAdapter := requestDecisionAdapter{editAdapter}
	if c.decisions, err = compose.NewRequestDecisions(compose.RequestDecisionDependencies{Records: s.requestRecords, People: decisionAdapter,
		Guardians: messenger, Plans: decisionAdapter, Effects: effects, Today: s.todayDate}); err != nil {
		return c, fmt.Errorf("care request decisions: %w", err)
	}
	if c.corrections, err = compose.NewRequestCorrections(compose.RequestCorrectionDependencies{Records: s.requestRecords, People: decisionAdapter,
		Plans: decisionAdapter, Effects: effects}); err != nil {
		return c, fmt.Errorf("care request corrections: %w", err)
	}
	if c.conflicts, err = compose.NewRequestConflicts(s.requestRecords, requestConflictPlans{s}, c.decisions, s.todayDate); err != nil {
		return c, fmt.Errorf("care request conflicts: %w", err)
	}
	return c, nil
}

// newCareRequestEffects binds Care Plan's request effects to the owners they
// report to: Communication's parent events, People Directory's request
// ledger, the realtime hub and the parent's sharing choice. A sink that does
// not exist stays unbound and its effect is skipped.
func newCareRequestEffects(
	s *careScheduleRequestService,
	emitter eventContract.Emitter,
	broadcaster realtime.Broadcaster,
	ledger *ParentRequestLedger,
	shares eventContract.ShareVisibilityResolver,
) (compose.RequestMessenger, compose.CareRequestEffects, error) {
	ports := compose.CareRequestEffectPorts{Records: s.requestRecords, Students: requestSubmissionAdapter{s}, Shares: shares, Logger: s.logger}
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

func (s *careScheduleRequestService) CreateRequest(ctx context.Context, studentID, guardianAccountID int64, payload json.RawMessage) (*carerequests.Request, error) {
	return s.requests.submissions.CreateRequest(ctx, studentID, guardianAccountID, payload)
}

// CreatePickupChangeRequest is the legacy mandatory-reason entry point; the
// owner names its input.
func (s *careScheduleRequestService) CreatePickupChangeRequest(ctx context.Context, studentID, guardianAccountID int64, date timezone.Date, pickupTime time.Time, reason string) (*carerequests.Request, error) {
	return s.CreatePickupChange(ctx, carerequests.MandatoryReasonPickupChange(studentID, guardianAccountID, date, pickupTime, reason))
}

func (s *careScheduleRequestService) CreatePickupChange(ctx context.Context, input carerequests.PickupChangeCreateInput) (*carerequests.Request, error) {
	return s.requests.submissions.CreatePickupChange(ctx, input)
}

func (s *careScheduleRequestService) EditRequest(ctx context.Context, input carerequests.EditInput) (*carerequests.Request, error) {
	request, err := s.requests.edits.EditRequest(ctx, input)
	if err != nil {
		return nil, legacyEditError(err)
	}
	return request, nil
}

func (s *careScheduleRequestService) Decide(ctx context.Context, input carerequests.DecideInput) (*carerequests.ReviewItem, error) {
	item, err := s.requests.decisions.Decide(ctx, input)
	if err != nil {
		return nil, legacyEditError(err)
	}
	return item, nil
}

func (s *careScheduleRequestService) canReviewStudent(ctx context.Context, student *peopleValues.StudentState) (bool, error) {
	if s.reviewPolicy == nil {
		return false, errors.New("care request review policy is required")
	}
	ok, err := s.reviewPolicy.Allows(ctx, s.permissions(ctx), student)
	if err != nil {
		return false, fmt.Errorf("schedule: resolve request reviewer scope: %w", err)
	}
	return ok, nil
}

func (s *careScheduleRequestService) applyPickupChangeRequest(ctx context.Context, req *carerequests.Request, expectedImpactToken *string, requireImpactToken bool) (carerequests.PickupApplied, error) {
	if s.requestRecords == nil || s.attendance == nil || s.pickupAutoExcusal == nil || s.userContext == nil || s.dayLocker == nil {
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

func (s *careScheduleRequestService) MarkDone(ctx context.Context, requestID int64, expectedVersion, reason string, reviewedBy int64) error {
	return legacyEditError(s.requests.decisions.MarkDone(ctx, requestID, expectedVersion, reason, reviewedBy))
}
