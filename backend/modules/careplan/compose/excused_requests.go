package compose

import (
	"context"
	"errors"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/application"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/ports"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// Consumer-owned ports of the excused-absence workflow, re-exported so the
// composition root can implement them without reaching into the module.
type (
	ExcusedRequestStudents = ports.ExcusedRequestStudents
	ReviewStudent          = ports.ReviewStudent
	PersonName             = ports.PersonName
	ReviewScope            = ports.ReviewScope
	ReviewScopeResolver    = ports.ReviewScopeResolver
	RequestEvent           = ports.RequestEvent
	DecisionAudience       = ports.DecisionAudience
	RequestMessenger       = ports.RequestMessenger
	StaffBroadcaster       = ports.StaffBroadcaster
	AbsenceReport          = ports.AbsenceReport
	AbsenceNotifier        = ports.AbsenceNotifier
	RequestLedgerEntry     = ports.RequestLedgerEntry
	RequestLedger          = ports.RequestLedger
	ShareVisibility        = ports.ShareVisibility
)

// ExcusedAbsenceRequests is the composed workflow behind the public
// careplan.ExcusedAbsenceRequests contract. Every dependency is bound at
// construction; a root whose effect sinks exist later hands in a port that
// resolves them lazily.
type ExcusedAbsenceRequests = application.ExcusedRequests

// ExcusedRequestDependencies wires the workflow. CarePlan, Students, Scope,
// and Today are required. Effect ports may be nil; the workflow then skips
// that effect, exactly like the legacy service tolerated unwired effects.
type ExcusedRequestDependencies struct {
	CarePlan    careplan.Capability
	Students    ExcusedRequestStudents
	Scope       ReviewScopeResolver
	Today       func() careplan.Date
	Observe     func(Observation)
	Logger      *slog.Logger
	Messenger   RequestMessenger
	Broadcaster StaffBroadcaster
	Notifier    AbsenceNotifier
	Ledger      RequestLedger
	Shares      ShareVisibility
}

// NewExcusedAbsenceRequests composes the excused-absence workflow over the
// Care Plan capability and the tenant runtime's transaction hooks.
func NewExcusedAbsenceRequests(deps ExcusedRequestDependencies) (*ExcusedAbsenceRequests, error) {
	if deps.CarePlan == nil || deps.Students == nil || deps.Scope == nil || deps.Today == nil {
		return nil, errors.New("care plan excused requests: care plan capability, student directory, review scope, and clock are required")
	}
	observe := ports.Observer(func(ports.Observation) {})
	if deps.Observe != nil {
		observe = ports.Observer(deps.Observe)
	}
	return application.NewExcusedRequests(application.ExcusedRequestDependencies{
		CarePlan: deps.CarePlan, Students: deps.Students, Scope: deps.Scope, Hooks: tenantHooks{},
		Today: deps.Today, Observe: observe, Logger: deps.Logger,
		Messenger: deps.Messenger, Broadcaster: deps.Broadcaster, Notifier: deps.Notifier,
		Ledger: deps.Ledger, Shares: deps.Shares,
	})
}

// tenantHooks binds the workflow's after-commit effects to the ambient
// tenant transaction.
type tenantHooks struct{}

func (tenantHooks) TenantID(ctx context.Context) int64 { return tenant.FromContext(ctx) }

func (tenantHooks) AfterCommit(ctx context.Context, fn func()) { tenant.RegisterAfterCommit(ctx, fn) }
