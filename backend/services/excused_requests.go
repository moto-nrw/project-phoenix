package services

import (
	"context"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	carePlanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	"github.com/moto-nrw/project-phoenix/modules/delivery/application/notifications"
	authjwt "github.com/moto-nrw/project-phoenix/modules/identityaccess/legacy/jwt"
	"github.com/moto-nrw/project-phoenix/realtime"
	"github.com/moto-nrw/project-phoenix/services/parentmessaging"
)

// excusedRequestReviewPolicy is the one method of the parent-request review
// policy the Care Plan excused-absence workflow consumes.
type excusedRequestReviewPolicy interface {
	AbsenceScope(ctx context.Context, permissions []string) (schoolWide bool, groupIDs []int64, err error)
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
	events      usersModels.ParentRequestEventRepository
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
		ledger = newParentRequestLedger(wiring.events)
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
		schoolWide, groupIDs, err := policy.AbsenceScope(ctx, authjwt.PermissionsFromCtx(ctx))
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
		RefTable: event.RefTable, Payload: event.Payload,
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
