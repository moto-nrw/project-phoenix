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
	"github.com/moto-nrw/project-phoenix/modules/staffannouncementaudience/contract"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

type Observation = ports.Observation

// ViewerNames resolves the display names of announcement viewers; the root
// binds it to People Directory.
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

func New(dependencies Dependencies, audience ports.Audience) (*communication.Module, error) {
	if audience == nil || dependencies.DB == nil || dependencies.Organizations == nil || dependencies.Viewers == nil || dependencies.Audit == nil || dependencies.Observe == nil {
		return nil, errors.New("communication compose: all dependencies are required")
	}
	store := postgres.New(AnnouncementDatabase())
	service := application.New(
		store,
		audience,
		application.NewTargets(dependencies.Organizations),
		dependencies.Viewers,
		application.NewAudit(dependencies.Audit),
		transaction{},
		application.NewAnnouncementObserver(dependencies.Observe),
	)
	return communication.NewModule(application.NewAnnouncementEngine(service, dependencies.Observe)), nil
}

type transaction struct{}

func (transaction) RunAdmin(ctx context.Context, callback func(context.Context) error) error {
	if _, ok := tenant.TransactionFromContext(ctx); ok {
		return callback(ctx)
	}
	return tenant.WithinAdminRetry(ctx, callback)
}

func AudienceMapping() contract.Rows[domain.Announcement, domain.AnnouncementViewDetail, domain.OperationStats] {
	return contract.Rows[domain.Announcement, domain.AnnouncementViewDetail, domain.OperationStats]{
		Announcement: func(row contract.Announcement) domain.Announcement { return domain.Announcement(row) },
		Target: func(value domain.Announcement) contract.Target {
			return contract.Target{PublishedAt: value.PublishedAt, TargetRoles: value.TargetRoles, TargetOrgIDs: value.TargetOrgIDs, TargetTenantIDs: value.TargetTenantIDs}
		},
		View: func(row contract.ViewDetail) domain.AnnouncementViewDetail { return domain.AnnouncementViewDetail(row) },
		Stats: func(stats contract.Stats) domain.OperationStats {
			return domain.OperationStats{Queries: stats.Queries, Rows: stats.Rows, StatementDuration: stats.StatementDuration}
		},
	}
}

// AnnouncementDatabase supplies the existing administrative runtime to the root.
func AnnouncementDatabase() func(context.Context) (bun.IDB, error) {
	return postgres.NewAnnouncementDatabase(tenant.TransactionFromContext)
}
