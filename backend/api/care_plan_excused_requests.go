package api

import (
	"context"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	carePlanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	communicationCompose "github.com/moto-nrw/project-phoenix/modules/communication/composition"
	deliveryCompose "github.com/moto-nrw/project-phoenix/modules/delivery/compose"
	"github.com/moto-nrw/project-phoenix/realtime"
	timezone "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// excusedRequestReviewPolicy is the one method of the parent-request review
// policy the Care Plan excused-absence workflow consumes.
type excusedRequestReviewPolicy interface {
	AbsenceScope(ctx context.Context, permissions []string) (schoolWide bool, groupIDs []int64, err error)
}

// CarePlanObserver receives the Care Plan workflow observations (stable
// operation, duration, store statistics, error). The root binds it to the
// same sink as the Care Plan capability so both report as one owner.
type CarePlanObserver func(carePlanCompose.Observation)

// NewExcusedAbsenceRequestService composes the Care Plan excused-absence
// workflow over the legacy root's dependencies. Nil effects are skipped.
func NewExcusedAbsenceRequestService(
	carePlan careplan.Capability, students ExcusedRequestStudentRows, persons ExcusedRequestPersonRows,
	scope carePlanCompose.ReviewScopeResolver, emitter *communicationCompose.ParentEventEmitter,
	broadcaster realtime.Broadcaster, events peopledirectory.ParentRequestEvents, notifier deliveryCompose.AbsenceNotifier,
	shares carePlanCompose.ShareVisibility, observe CarePlanObserver, logger *slog.Logger, today func() timezone.Date,
) (*carePlanCompose.ExcusedAbsenceRequests, error) {
	var messenger carePlanCompose.RequestMessenger
	if emitter != nil {
		messenger = NewExcusedRequestMessenger(emitter)
	}
	var staff carePlanCompose.StaffBroadcaster
	if broadcaster != nil {
		staff = excusedRequestBroadcaster{broadcaster: broadcaster}
	}
	var ledger carePlanCompose.RequestLedger
	if events != nil {
		ledger = NewParentRequestLedger(events)
	}
	var absences carePlanCompose.AbsenceNotifier
	if notifier != nil {
		absences = excusedAbsenceNotifier{notifier: notifier}
	}
	var day func() careplan.Date
	if today != nil {
		day = func() careplan.Date { return careplan.Date(today()) }
	}
	return NewExcusedAbsenceRequests(ExcusedRequestWiring{
		CarePlan: carePlan, Students: students, Persons: persons,
		Scope: scope, Today: day, Logger: logger, Observe: observe,
		Messenger: messenger, Broadcaster: staff, Ledger: ledger,
		Notifier: absences, Shares: shares,
	})
}

// ParentRequestReviewScope adapts the identity-owned review policy to the
// workflow's data-only port: the permissions come from the request claims.
func ParentRequestReviewScope(policy excusedRequestReviewPolicy, permissions func(context.Context) []string) carePlanCompose.ReviewScopeResolver {
	return func(ctx context.Context) (carePlanCompose.ReviewScope, error) {
		schoolWide, groupIDs, err := policy.AbsenceScope(ctx, permissions(ctx))
		if err != nil {
			return carePlanCompose.ReviewScope{}, err
		}
		return carePlanCompose.ReviewScope{SchoolWide: schoolWide, GroupIDs: groupIDs}, nil
	}
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
type excusedAbsenceNotifier struct {
	notifier deliveryCompose.AbsenceNotifier
}

func (n excusedAbsenceNotifier) NotifyAbsenceReported(ctx context.Context, report carePlanCompose.AbsenceReport) error {
	dates := make([]timezone.Date, len(report.Dates))
	for i := range report.Dates {
		dates[i] = timezone.Date(report.Dates[i])
	}
	return n.notifier.NotifyAbsenceReported(ctx, deliveryCompose.AbsenceReport{
		TenantID: report.TenantID, StudentIDs: report.StudentIDs, Status: report.Status, Dates: dates,
		FromParent: report.FromParent, ActorAccountID: report.ActorAccountID, ExcludedAccountIDs: report.ExcludedAccountIDs,
	})
}
