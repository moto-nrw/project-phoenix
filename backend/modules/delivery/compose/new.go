package compose

import (
	"context"
	"errors"
	"log/slog"

	deliveryContract "github.com/moto-nrw/project-phoenix/modules/delivery/contract"
	"github.com/moto-nrw/project-phoenix/modules/delivery/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/modules/delivery/internal/application"
	"github.com/moto-nrw/project-phoenix/modules/delivery/internal/ports"
	delivery "github.com/moto-nrw/project-phoenix/modules/delivery/runtime"
	"github.com/uptrace/bun"
)

// ModuleDependencies are the enqueue and state side of Delivery, which the
// producers and Identity & Access need before the mail renderers exist.
type ModuleDependencies struct {
	DB      *bun.DB
	People  GuardianDisplayResolver
	Observe func(deliveryContract.Observation)
	// EmailProvider is the mail provider of this deployment; its zero value is
	// plain SMTP.
	EmailProvider EmailProviderConfig
	Logger        *slog.Logger
}

type Runtime application.Runtime
type Provider interface{ application.Provider }
type GuardianDisplayResolver interface{ ports.GuardianDirectory }

func NewWorker(runtime *Runtime, provider Provider) (*delivery.Worker, error) {
	return application.NewWorkerBinding((*application.Runtime)(runtime), provider)
}
func NewPushDispatcher(runtime *Runtime, provider Provider) *delivery.PushDispatcher {
	return application.NewPushDispatcher((*application.Runtime)(runtime), provider)
}
func (r *Runtime) EmailStatus(ctx context.Context, tenantID, id int64) (deliveryContract.State, bool, error) {
	return (*application.Runtime)(r).EmailStatus(ctx, tenantID, id)
}

// NewModule composes the enqueue capability and the e-mail state without the
// worker; NewWorker adds the worker once the mail renderers exist.
func NewModule(dependencies ModuleDependencies) (*Runtime, error) {
	if dependencies.DB == nil || dependencies.People == nil || dependencies.Observe == nil {
		return nil, errors.New("delivery compose: all dependencies are required")
	}
	if err := dependencies.EmailProvider.Validate(); err != nil {
		return nil, err
	}
	raw := postgres.New(postgres.RequireTenantDatabase, postgres.RequireAdminDatabase)
	store := application.NewWorkerCommands(raw, NewTenantTransactions(dependencies.DB).WithinSystem)
	observe := application.NewObservationAdapter(dependencies.Observe)
	webhooks, reconciliation := newEmailEvents(dependencies.EmailProvider, store, dependencies.Logger, observe)
	return &Runtime{
		Module: application.NewService(store, dependencies.People, observe), Store: store, Observe: observe, EmailWakeups: postgres.EmailWakeups(dependencies.DB),
		EmailWebhooks: webhooks, EmailReconciliation: reconciliation,
	}, nil
}

// EmailWebhookIngest is the capability behind the webhook routes.
func (r *Runtime) EmailWebhookIngest() delivery.EmailWebhooks {
	return (*application.Runtime)(r).EmailWebhooks
}

// EmailSuppressionLift frees corrected addresses.
func (r *Runtime) EmailSuppressionLift() delivery.EmailSuppressions {
	return (*application.Runtime)(r).EmailReconciliation
}
