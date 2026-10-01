package compose

import (
	"context"
	"errors"
	"log/slog"

	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/application"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/ports"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// Consumer-owned ports of the offering-change review and the pickup
// adjustment (#3561), re-exported so the composition root can implement them
// without reaching into the module.
type (
	OfferingChangeRows       = ports.OfferingChangeRows
	OfferingChangeCatalog    = ports.OfferingChangeCatalog
	OfferingCarePeriod       = ports.OfferingCarePeriod
	OfferingChangeChild      = ports.OfferingChangeChild
	OfferingCatalogState     = ports.OfferingCatalogState
	OfferingChangeEnrollment = ports.OfferingChangeEnrollment
	OfferingChangeStudents   = ports.OfferingChangeStudents
	OfferingChangeSettings   = ports.OfferingChangeSettings
	OfferingChangePlanning   = ports.OfferingChangePlanning
	ManualPlanningOccurrence = ports.ManualPlanningOccurrence
	CourseOfferingReference  = ports.CourseOfferingReference
	CourseGroup              = ports.CourseGroup
	PickupAdjustmentStudents = ports.PickupAdjustmentStudents
	PickupAdjustmentSettings = ports.PickupAdjustmentSettings
	PickupPlanAudit          = ports.PickupPlanAudit
)

// OfferingChangeDependencies bind the offering-change review to its owners.
// Rows, Catalog, Enrollment, Students, Settings, Planning, Bookings and Scope
// are required; the effect ports may be nil and the effect is then skipped.
type OfferingChangeDependencies struct {
	Rows       OfferingChangeRows
	Catalog    OfferingChangeCatalog
	Enrollment OfferingChangeEnrollment
	Students   OfferingChangeStudents
	Settings   OfferingChangeSettings
	Planning   OfferingChangePlanning
	// Bookings is the booking materialization (#3560) approvals and direct
	// corrections apply through.
	Bookings careplan.BookingMaterializationCapability
	Scope    ReviewScopeResolver

	Messenger RequestMessenger
	Ledger    RequestLedger
	Shares    ShareVisibility

	Today  func() calendar.Date
	Logger *slog.Logger
}

// NewOfferingChanges composes the offering-change review over the tenant
// runtime's transaction hooks.
func NewOfferingChanges(deps OfferingChangeDependencies) (careplan.OfferingChangeCapability, error) {
	if deps.Bookings == nil {
		return nil, errors.New("offering changes: booking materialization is required")
	}
	return application.NewOfferingChanges(application.OfferingChangeDependencies{
		Rows: deps.Rows, Catalog: deps.Catalog, Enrollment: deps.Enrollment, Students: deps.Students,
		Settings: deps.Settings, Planning: deps.Planning, Bookings: deps.Bookings, Scope: deps.Scope,
		Hooks: tenantHooks{}, Messenger: deps.Messenger, Ledger: deps.Ledger, Shares: deps.Shares,
		Today: deps.Today, Logger: deps.Logger,
	})
}

// PickupAdjustmentDependencies bind the pickup adjustment to Care Plan's
// schedule services and records, the offering-change review, and the People
// Directory, Audit Platform and Security Runtime through ports.
type PickupAdjustmentDependencies struct {
	PickupSchedules  careplan.PickupScheduleService
	ArrivalSchedules careplan.ArrivalScheduleService
	Records          ports.PickupAdjustmentSchedules
	Baselines        careplan.PickupBaselineReader
	Offerings        careplan.DirectOfferingAdjustments
	Settings         PickupAdjustmentSettings
	Audit            PickupPlanAudit
	Students         PickupAdjustmentStudents
	// Fingerprint returns the lowercase hexadecimal SHA-256 of the content.
	Fingerprint func([]byte) string
	Today       func() calendar.Date
}

// NewPickupAdjustments composes the pickup adjustment over the tenant
// runtime.
func NewPickupAdjustments(deps PickupAdjustmentDependencies) (careplan.PickupAdjustments, error) {
	return application.NewPickupAdjustments(application.PickupAdjustmentDependencies{
		PickupSchedules: deps.PickupSchedules, ArrivalSchedules: deps.ArrivalSchedules, Records: deps.Records,
		Baselines: deps.Baselines, Offerings: deps.Offerings, Settings: deps.Settings, Audit: deps.Audit,
		Students: deps.Students, UnitOfWork: tenantUnitOfWork{}, Fingerprint: deps.Fingerprint, Today: deps.Today,
	})
}

// tenantUnitOfWork runs a pickup adjustment in the tenant transaction of the
// request, joining the one the tenant middleware opened.
type tenantUnitOfWork struct{}

func (tenantUnitOfWork) TenantID(ctx context.Context) int64 { return tenant.FromContext(ctx) }

func (tenantUnitOfWork) RunInTenantTx(ctx context.Context, fn func(context.Context) error) error {
	return tenant.WithTenantTx(ctx, (*bun.DB)(nil), tenant.FromContext(ctx), func(txCtx context.Context, _ bun.Tx) error {
		return fn(txCtx)
	})
}

func (tenantUnitOfWork) MarkRollback(ctx context.Context) { tenant.MarkRollback(ctx) }
