package compose

import (
	"context"
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/communication"
	"github.com/moto-nrw/project-phoenix/modules/communication/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/modules/communication/internal/application"
	"github.com/moto-nrw/project-phoenix/modules/communication/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/communication/internal/ports"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	"github.com/moto-nrw/project-phoenix/modules/staffannouncementaudience/contract"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

type AuditEntry struct {
	OperatorID int64
	Action     string
	ResourceID int64
	RequestIP  net.IP
	Changes    map[string]any
}

type Audit interface {
	AppendAnnouncementAudit(context.Context, AuditEntry) error
}

type AuditFunc func(context.Context, AuditEntry) error

func (f AuditFunc) AppendAnnouncementAudit(ctx context.Context, entry AuditEntry) error {
	return f(ctx, entry)
}

type Observation = ports.Observation

// ViewerNames resolves the display names of announcement viewers; the root
// binds it to People Directory.
type (
	ViewerNames    = ports.ViewerNames
	OperationStats = domain.OperationStats
)

type Dependencies struct {
	DB            *bun.DB
	Organizations organizationtenancy.Capability
	Viewers       ViewerNames
	Audit         Audit
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
		targets{capability: dependencies.Organizations},
		dependencies.Viewers,
		audit{port: dependencies.Audit},
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

type targets struct {
	capability organizationtenancy.Capability
}

func (t targets) CountOrganizationsByID(ctx context.Context, ids []int64) (int, domain.OperationStats, error) {
	count, err := t.capability.CountOrganizationsByID(ctx, ids)
	stats := domain.OperationStats{Queries: 1, Rows: int64(count)}
	if errors.Is(err, organizationtenancy.ErrInvalidOrganization) {
		return 0, stats, fmt.Errorf("%w: %v", domain.ErrInvalidTarget, err)
	}
	return count, stats, err
}

func (t targets) CountSchoolsByID(ctx context.Context, ids []int64) (int, domain.OperationStats, error) {
	count, err := t.capability.CountSchoolsByID(ctx, ids)
	stats := domain.OperationStats{Queries: 1, Rows: int64(count)}
	if errors.Is(err, organizationtenancy.ErrInvalidSchool) {
		return 0, stats, fmt.Errorf("%w: %v", domain.ErrInvalidTarget, err)
	}
	return count, stats, err
}

type audit struct{ port Audit }

func (a audit) Append(ctx context.Context, entry domain.AuditEntry) (domain.OperationStats, error) {
	started := time.Now()
	err := a.port.AppendAnnouncementAudit(ctx, AuditEntry{
		OperatorID: entry.OperatorID, Action: entry.Action, ResourceID: entry.ResourceID,
		RequestIP: entry.RequestIP, Changes: entry.Changes,
	})
	stats := domain.OperationStats{Queries: 1, StatementDuration: time.Since(started)}
	if err == nil {
		stats.Rows = 1
	}
	return stats, err
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
