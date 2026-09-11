package services

import (
	"context"
	"errors"
	"log/slog"

	authjwt "github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	carePlanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	"github.com/moto-nrw/project-phoenix/modules/delivery/application/notifications"
	"github.com/moto-nrw/project-phoenix/realtime"
	"github.com/moto-nrw/project-phoenix/services/parentmessaging"
	"github.com/moto-nrw/project-phoenix/services/users"
)

// excusedRequestReviewPolicy is the one method of the parent-request review
// policy the Care Plan excused-absence workflow consumes.
type excusedRequestReviewPolicy interface {
	Scope(ctx context.Context, permissions []string) (schoolWide bool, groupIDs []int64, err error)
}

// excusedRequestWiring is the legacy root's view of the Care Plan
// excused-absence workflow dependencies. Nil effects are skipped.
type excusedRequestWiring struct {
	carePlan    careplan.Capability
	students    usersModels.StudentRepository
	persons     usersModels.PersonRepository
	scope       carePlanCompose.ReviewScopeResolver
	emitter     *parentmessaging.Emitter
	broadcaster realtime.Broadcaster
	events      users.ParentRequestEventRecorder
	notifier    notifications.AbsenceNotifier
	shares      carePlanCompose.ShareVisibility
	observe     CarePlanObserver
	logger      *slog.Logger
	today       func() timezone.Date
}

// CarePlanObserver receives the Care Plan workflow observations (stable
// operation, duration, store statistics, error). The root binds it to the
// same sink as the Care Plan capability so both report as one owner.
type CarePlanObserver func(carePlanCompose.Observation)

// shareVisibilityFunc adapts a closure to the workflow's sharing port. The
// root uses it to resolve the parents domain, which is composed after the
// request services it serves, at call time instead of at construction.
type shareVisibilityFunc func(ctx context.Context, studentID int64, requestType string, requestID int64) ([]int64, error)

func (f shareVisibilityFunc) SharedRecipientAccountIDs(ctx context.Context, studentID int64, requestType string, requestID int64) ([]int64, error) {
	return f(ctx, studentID, requestType, requestID)
}

func newExcusedAbsenceRequests(wiring excusedRequestWiring) (*carePlanCompose.ExcusedAbsenceRequests, error) {
	var messenger carePlanCompose.RequestMessenger
	if wiring.emitter != nil {
		messenger = excusedRequestMessenger{emitter: wiring.emitter}
	}
	var broadcaster carePlanCompose.StaffBroadcaster
	if wiring.broadcaster != nil {
		broadcaster = excusedRequestBroadcaster{broadcaster: wiring.broadcaster}
	}
	var ledger carePlanCompose.RequestLedger
	if wiring.events != nil {
		ledger = excusedRequestLedger{events: wiring.events}
	}
	var notifier carePlanCompose.AbsenceNotifier
	if wiring.notifier != nil {
		notifier = excusedAbsenceNotifier{notifier: wiring.notifier}
	}
	var today func() careplan.Date
	if wiring.today != nil {
		today = func() careplan.Date { return careplan.Date(wiring.today()) }
	}
	return repositories.NewExcusedAbsenceRequests(repositories.ExcusedRequestWiring{
		CarePlan: wiring.carePlan, Students: wiring.students, Persons: wiring.persons,
		Scope: wiring.scope, Today: today, Logger: wiring.logger, Observe: wiring.observe,
		Messenger: messenger, Broadcaster: broadcaster, Ledger: ledger,
		Notifier: notifier, Shares: wiring.shares,
	})
}

// parentRequestReviewScope adapts the identity-owned review policy to the
// workflow's data-only port: the permissions come from the request claims.
func parentRequestReviewScope(policy excusedRequestReviewPolicy) carePlanCompose.ReviewScopeResolver {
	return func(ctx context.Context) (carePlanCompose.ReviewScope, error) {
		schoolWide, groupIDs, err := policy.Scope(ctx, authjwt.PermissionsFromCtx(ctx))
		if err != nil {
			return carePlanCompose.ReviewScope{}, err
		}
		return carePlanCompose.ReviewScope{SchoolWide: schoolWide, GroupIDs: groupIDs}, nil
	}
}

// excusedRequestMessenger maps the workflow's request events onto the
// Communication parent-event emitter.
type excusedRequestMessenger struct{ emitter *parentmessaging.Emitter }

func (m excusedRequestMessenger) EnqueueRequestDecision(ctx context.Context, tenantID, studentID, guardianAccountID int64, event carePlanCompose.RequestEvent) error {
	return m.emitter.EnqueueRequestDecision(ctx, tenantID, studentID, guardianAccountID, childEvent(event))
}

func (m excusedRequestMessenger) EmitChildEvent(tenantID, studentID, guardianAccountID int64, event carePlanCompose.RequestEvent) {
	m.emitter.EmitChildEvent(tenantID, studentID, guardianAccountID, childEvent(event))
}

func (m excusedRequestMessenger) BroadcastChildUpdateToGuardians(tenantID, studentID int64) {
	m.emitter.BroadcastChildUpdateToGuardians(tenantID, studentID)
}

func (m excusedRequestMessenger) GuardianHasChildAccess(ctx context.Context, studentID, guardianAccountID int64) (bool, error) {
	return m.emitter.GuardianHasChildAccess(ctx, studentID, guardianAccountID)
}

func (m excusedRequestMessenger) ResolveDecisionAudience(ctx context.Context, studentID, submitterAccountID int64, sharedAccountIDs []int64) (carePlanCompose.DecisionAudience, error) {
	audience, err := m.emitter.ResolveDecisionAudience(ctx, studentID, submitterAccountID, sharedAccountIDs)
	return carePlanCompose.DecisionAudience{Full: audience.Full, Neutral: audience.Neutral}, err
}

func (m excusedRequestMessenger) EmitDecisionAudience(tenantID, studentID int64, audience carePlanCompose.DecisionAudience, full, neutral carePlanCompose.RequestEvent) {
	m.emitter.EmitDecisionAudience(tenantID, studentID, parentmessaging.DecisionAudience{Full: audience.Full, Neutral: audience.Neutral}, childEvent(full), childEvent(neutral))
}

func childEvent(event carePlanCompose.RequestEvent) parentmessaging.ChildEvent {
	result := parentmessaging.ChildEvent{
		EventType: event.EventType, ActorKind: event.ActorKind, ActorAccountID: event.ActorAccountID, Body: event.Body,
		RequestType: event.RequestType, RequestStatus: event.RequestStatus, DecisionReason: event.DecisionReason,
		RefTable: event.RefTable,
	}
	if event.RefID > 0 {
		refID := event.RefID
		result.RefID = &refID
	}
	return result
}

// excusedRequestBroadcaster wakes staff tabs through the realtime hub.
type excusedRequestBroadcaster struct{ broadcaster realtime.Broadcaster }

const excusedRequestBroadcastSource = "excused_request"

func (b excusedRequestBroadcaster) StudentUpdated(tenantID int64) error {
	source := excusedRequestBroadcastSource
	return b.broadcaster.BroadcastToTenant(tenantID, realtime.NewEvent(realtime.EventStudentUpdated, "", realtime.EventData{Source: &source}))
}

func (b excusedRequestBroadcaster) ChangeRequestsChanged(tenantID int64) error {
	source := excusedRequestBroadcastSource
	return b.broadcaster.BroadcastToTenant(tenantID, realtime.NewEvent(realtime.EventChangeRequestsChanged, "", realtime.EventData{Source: &source}))
}

// excusedAbsenceNotifier hands approved absences to the Delivery producer.
type excusedAbsenceNotifier struct{ notifier notifications.AbsenceNotifier }

func (n excusedAbsenceNotifier) NotifyAbsenceReported(ctx context.Context, report carePlanCompose.AbsenceReport) error {
	dates := make([]timezone.Date, len(report.Dates))
	for i := range report.Dates {
		dates[i] = timezone.Date(report.Dates[i])
	}
	return n.notifier.NotifyAbsenceReported(ctx, notifications.AbsenceReport{
		TenantID: report.TenantID, StudentIDs: report.StudentIDs, Status: report.Status, Dates: dates,
		FromParent: report.FromParent, ActorAccountID: report.ActorAccountID, ExcludedAccountIDs: report.ExcludedAccountIDs,
	})
}

// excusedRequestLedger writes the workflow's history into the shared
// parent-request ledger.
type excusedRequestLedger struct {
	events users.ParentRequestEventRecorder
}

func (l excusedRequestLedger) Record(ctx context.Context, entry carePlanCompose.RequestLedgerEntry) error {
	return users.RecordParentRequestEvent(ctx, l.events, users.ParentRequestEventInput{
		StudentID: entry.StudentID, RequestType: entry.RequestType, RequestID: entry.RequestID,
		EventType: entry.EventType, ActorAccountID: entry.ActorAccountID, UpdatedAt: entry.UpdatedAt, Payload: entry.Payload,
	})
}

// excusedRequestCoordinatorPort presents the Care Plan workflow to the
// cross-kind parent-request coordinator in the coordinator's own vocabulary.
type excusedRequestCoordinatorPort struct {
	requests careplan.ExcusedAbsenceRequests
}

var (
	_ users.ExcusedBulkReviewPort     = excusedRequestCoordinatorPort{}
	_ users.ParentRequestConflictPort = excusedRequestCoordinatorPort{}
)

func (p excusedRequestCoordinatorPort) GetExcusedBulkCandidate(ctx context.Context, requestID int64) (*users.ExcusedBulkCandidate, error) {
	candidate, err := p.requests.GetExcusedBulkCandidate(ctx, requestID)
	if err != nil || candidate == nil {
		return nil, mapExcusedRequestError(err)
	}
	return &users.ExcusedBulkCandidate{ID: candidate.ID, StudentID: candidate.StudentID, UpdatedAt: candidate.UpdatedAt, Eligible: candidate.Eligible}, nil
}

func (p excusedRequestCoordinatorPort) LockExcusedBulkRequest(ctx context.Context, requestID int64) error {
	return mapExcusedRequestError(p.requests.LockExcusedBulkRequest(ctx, requestID))
}

func (p excusedRequestCoordinatorPort) ApproveExcusedBulk(ctx context.Context, requestID int64, reason string, reviewerID int64, expectedVersion string) error {
	return mapExcusedRequestError(p.requests.ApproveExcusedBulk(ctx, requestID, reason, reviewerID, expectedVersion))
}

func (p excusedRequestCoordinatorPort) ConflictCandidate(ctx context.Context, requestID int64) (*users.ParentRequestConflictCandidate, error) {
	candidate, err := p.requests.ConflictCandidate(ctx, requestID)
	if err != nil {
		return nil, mapExcusedRequestError(err)
	}
	return &users.ParentRequestConflictCandidate{StudentID: candidate.StudentID, UpdatedAt: candidate.UpdatedAt}, nil
}

func (p excusedRequestCoordinatorPort) LockConflictRequest(ctx context.Context, requestID int64) error {
	return mapExcusedRequestError(p.requests.LockConflictRequest(ctx, requestID))
}

func (p excusedRequestCoordinatorPort) DecideConflictRequest(ctx context.Context, decision users.ParentRequestConflictDecision) error {
	return mapExcusedRequestError(p.requests.DecideConflictRequest(ctx, careplan.ExcusedConflictDecision{
		RequestID: decision.RequestID, Approve: decision.Approve, Reason: decision.Reason,
		ReviewerID: decision.ReviewerID, ExpectedVersion: decision.ExpectedVersion,
	}))
}

// WriteStaffValue reads {"value": "<status>"} from the coordinator payload;
// anything else is the workflow's invalid-status sentinel, never a 500.
func (p excusedRequestCoordinatorPort) WriteStaffValue(ctx context.Context, write users.ParentRequestStaffValueWrite) error {
	status, ok := write.Value["value"].(string)
	if !ok {
		return careplan.ErrAbsenceRequestInvalidStatus
	}
	return mapExcusedRequestError(p.requests.WriteStaffValue(ctx, careplan.ExcusedStaffValueWrite{
		StudentID: write.StudentID, RequestIDs: write.RequestIDs, Reason: write.Reason, Status: status,
	}))
}

// mapExcusedRequestError translates the Care Plan sentinels into the ones
// the cross-kind coordinator and its routes match on. Domain-specific
// sentinels pass through unchanged.
func mapExcusedRequestError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, careplan.ErrExcusedRequestNotFound):
		return activeModels.ErrExcusedRequestNotFound
	case errors.Is(err, careplan.ErrExcusedRequestNotPending):
		return activeModels.ErrExcusedRequestNotPending
	case errors.Is(err, careplan.ErrExcusedRequestNotDecided):
		return activeModels.ErrExcusedRequestNotDecided
	case errors.Is(err, careplan.ErrParentRequestStale):
		return users.ErrParentRequestStale
	case errors.Is(err, careplan.ErrParentRequestDecisionRace):
		return users.ErrParentRequestDecisionRace
	case errors.Is(err, careplan.ErrParentRequestReasonRequired):
		return users.ErrParentRequestReasonRequired
	case errors.Is(err, careplan.ErrParentRequestPast):
		return users.ErrParentRequestPast
	case errors.Is(err, careplan.ErrParentRequestNotPast):
		return users.ErrParentRequestNotPast
	case errors.Is(err, careplan.ErrParentRequestNotDecided):
		return users.ErrParentRequestNotDecided
	case errors.Is(err, careplan.ErrParentRequestCorrectionUnsupported):
		return users.ErrParentRequestCorrectionUnsupported
	case errors.Is(err, careplan.ErrStaffValueUnsupported):
		return users.ErrStaffValueUnsupported
	default:
		return err
	}
}
