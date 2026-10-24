package compose

import (
	"context"
	"errors"

	recordports "github.com/moto-nrw/project-phoenix/modules/communication/ports"

	"github.com/moto-nrw/project-phoenix/modules/communication"
	"github.com/moto-nrw/project-phoenix/modules/communication/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/modules/communication/internal/application"
	"github.com/moto-nrw/project-phoenix/modules/communication/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/communication/internal/ports"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

type Observation = ports.Observation

// ViewerNames binds announcement viewer names to People Directory.
type (
	ViewerNames    = ports.ViewerNames
	OperationStats = domain.OperationStats
)

type Dependencies struct {
	DB            *bun.DB
	Organizations recordports.OrganizationTargets
	Viewers       ViewerNames
	Audit         recordports.Audit
	Observe       func(Observation)
}

func New(dependencies Dependencies, audience ports.Audience) (communication.Capability, error) {
	if audience == nil || dependencies.DB == nil || dependencies.Organizations == nil || dependencies.Viewers == nil || dependencies.Audit == nil || dependencies.Observe == nil {
		return nil, errors.New("communication compose: all dependencies are required")
	}
	return application.New(
		postgres.New(AnnouncementDatabase()),
		audience,
		application.NewTargets(dependencies.Organizations),
		dependencies.Viewers,
		application.NewAudit(dependencies.Audit),
		transaction{},
		dependencies.Observe,
	), nil
}

type transaction struct{}

func (transaction) RunAdmin(ctx context.Context, callback func(context.Context) error) error {
	return tenant.WithinAdminRetry(ctx, callback)
}

// AnnouncementDatabase supplies the existing administrative runtime to the root.
func AnnouncementDatabase() func(context.Context) (bun.IDB, error) {
	return postgres.NewAnnouncementDatabase(tenant.TransactionFromContext)
}

func (transaction) HasTransaction(ctx context.Context) bool {
	return application.TransactionPresent(ctx, tenant.TransactionFromContext)
}
