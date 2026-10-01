package testutil

import (
	"context"
	"testing"

	"github.com/moto-nrw/project-phoenix/services"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func NewEnrollmentOwner() services.EnrollmentBookingFixture {
	return services.NewEnrollmentBookingFixture()
}

// NewApprovedOfferingProjection wires the same owner projection used by the
// server while keeping composition out of individual API test packages.
func NewApprovedOfferingProjection(db *bun.DB, selections services.ApprovedSelectionTestReader) (*services.ApprovedOfferingTestProjection, error) {
	return services.NewApprovedOfferingTestProjection(db, selections)
}

// CareOfferingCatalogOption replaces one collaborator of the Care Plan
// care-offering catalog a suite drives.
type CareOfferingCatalogOption func(*services.CareOfferingCatalogTestOptions)

// WithCareOfferingSettings replaces the tenant settings the catalog reads the
// grade range from.
func WithCareOfferingSettings(settings interface {
	ResolveInt(context.Context, string) (int, error)
}) CareOfferingCatalogOption {
	return func(options *services.CareOfferingCatalogTestOptions) { options.Settings = settings }
}

// WithCareOfferingRosterResync binds the roster resync of the templates sourcing
// an edited or deleted offering (the decision service in production).
func WithCareOfferingRosterResync(resync interface {
	ResyncTemplatesSourcedFromOffering(context.Context, int64, calendar.Date) error
	DetachTemplatesSourcedFromOffering(context.Context, int64, calendar.Date) error
}) CareOfferingCatalogOption {
	return func(options *services.CareOfferingCatalogTestOptions) { options.SourcedTemplates = resync }
}

// WithCareOfferingPickupResync binds the pickup projection refresh after an
// offering edit.
func WithCareOfferingPickupResync(resync interface {
	ReconcileOfferingPickupForOffering(context.Context, int64) error
}) CareOfferingCatalogOption {
	return func(options *services.CareOfferingCatalogTestOptions) { options.Pickup = resync }
}

// WithCareOfferingRecurrenceLock binds the tenant recurrence gate.
func WithCareOfferingRecurrenceLock(lock func(context.Context) error) CareOfferingCatalogOption {
	return func(options *services.CareOfferingCatalogTestOptions) { options.LockRecurrence = lock }
}

// WithCareOfferingToday pins the catalog's calendar day.
func WithCareOfferingToday(today func() calendar.Date) CareOfferingCatalogOption {
	return func(options *services.CareOfferingCatalogTestOptions) { options.Today = today }
}

// NewCareOfferingCatalog composes the Care Plan care-offering catalog (#3559)
// over the test database with the owner bindings of the server.
func NewCareOfferingCatalog(tb testing.TB, db *bun.DB, options ...CareOfferingCatalogOption) services.CareOfferingCatalogTestModule {
	tb.Helper()
	var chosen services.CareOfferingCatalogTestOptions
	for _, option := range options {
		option(&chosen)
	}
	module, err := services.NewCareOfferingCatalogTestModule(db, testpkg.TenantRuntime(tb, db), chosen)
	require.NoError(tb, err)
	return module
}
