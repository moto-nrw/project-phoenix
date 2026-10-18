package application

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/ports"
)

type bookingRostersStub struct{ ports.BookingRosters }
type bookingTemplatesStub struct{ ports.BookingTemplates }
type bookingPeriodsStub struct{ ports.BookingPeriods }
type bookingEnrollmentStub struct{ ports.BookingEnrollment }
type bookingStudentsStub struct{ ports.BookingStudents }
type bookingSettingsStub struct{ ports.BookingSettings }
type bookingCommandsStub struct{ ports.BookingCommands }
type adjustmentAuditStub struct{ ports.AdjustmentAudit }
type bookingPickupStub struct{ ports.BookingPickup }

func bookingDependencies() BookingMaterializationDependencies {
	return BookingMaterializationDependencies{
		Rosters: bookingRostersStub{}, Templates: bookingTemplatesStub{}, Periods: bookingPeriodsStub{},
		Enrollment: bookingEnrollmentStub{}, Students: bookingStudentsStub{}, Settings: bookingSettingsStub{},
		Bookings: bookingCommandsStub{}, Audit: adjustmentAuditStub{}, Pickup: bookingPickupStub{},
		RunInTx: func(ctx context.Context, fn func(context.Context) error) error { return fn(ctx) },
	}
}

// The catalog and the booking materialization composed together resync
// through each other: the catalog never resolves a resyncer late, so an
// offering edit cannot save while its sourced rosters go stale.
func TestNewCareOfferingCatalogWithBookings_BindsTheResyncers(t *testing.T) {
	t.Parallel()

	catalog, bookings, err := NewCareOfferingCatalogWithBookings(catalogDependencies(), bookingDependencies())
	require.NoError(t, err)
	require.Same(t, catalog, bookings.catalog)
	require.Same(t, bookings, catalog.sourcedTemplateResyncer())
	require.Same(t, bookings, catalog.pickupResyncer())
}

// A missing dependency of either half is a construction error, and the
// late resolvers and the catalog the constructor binds itself are refused.
func TestNewCareOfferingCatalogWithBookings_RequiresItsDependencies(t *testing.T) {
	t.Parallel()

	withoutPhases := catalogDependencies()
	withoutPhases.Phases = nil
	_, _, err := NewCareOfferingCatalogWithBookings(withoutPhases, bookingDependencies())
	require.ErrorContains(t, err, "phases")

	withoutPickup := bookingDependencies()
	withoutPickup.Pickup = nil
	_, _, err = NewCareOfferingCatalogWithBookings(catalogDependencies(), withoutPickup)
	require.ErrorContains(t, err, "pickup")

	lateSourced := catalogDependencies()
	lateSourced.SourcedTemplates = func() ports.SourcedTemplateResyncer { return nil }
	_, _, err = NewCareOfferingCatalogWithBookings(lateSourced, bookingDependencies())
	require.ErrorContains(t, err, "must be unset")

	latePickup := catalogDependencies()
	latePickup.Pickup = func() ports.PickupResyncer { return nil }
	_, _, err = NewCareOfferingCatalogWithBookings(latePickup, bookingDependencies())
	require.ErrorContains(t, err, "must be unset")

	presetCatalog := bookingDependencies()
	presetCatalog.Catalog = newTestCatalog(t, catalogDependencies())
	_, _, err = NewCareOfferingCatalogWithBookings(catalogDependencies(), presetCatalog)
	require.ErrorContains(t, err, "must be unset")
}
