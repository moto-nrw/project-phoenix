package testutil

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/services"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// BookingMaterializationOption replaces one collaborator of Care Plan's
// booking materialization (#3560) a suite drives.
type BookingMaterializationOption func(*services.BookingMaterializationTestOptions)

// WithBookingSettings replaces the tenant settings the materialization reads
// the care-offerings and authoritative-bookings flags from.
func WithBookingSettings(settings interface {
	ResolveBool(context.Context, string) (bool, error)
}) BookingMaterializationOption {
	return func(options *services.BookingMaterializationTestOptions) { options.Settings = settings }
}

// WithBookingCatalog applies the suite's care-offering catalog instead of a
// freshly composed one.
func WithBookingCatalog(catalog services.BookingTestCatalog) BookingMaterializationOption {
	return func(options *services.BookingMaterializationTestOptions) { options.Catalog = catalog }
}

// WithBookingRecurrenceLock binds the tenant recurrence gate.
func WithBookingRecurrenceLock(lock func(context.Context) error) BookingMaterializationOption {
	return func(options *services.BookingMaterializationTestOptions) { options.LockRecurrence = lock }
}

// WithBookingWithdrawals binds the complete-withdrawal follow-up of an
// authoritative booking change.
func WithBookingWithdrawals(withdrawals services.BookingTestWithdrawals) BookingMaterializationOption {
	return func(options *services.BookingMaterializationTestOptions) { options.Withdrawals = withdrawals }
}

// WithBookingPickupWeekdayExtension binds the close of a manual weekly
// pickup's Timetable task.
func WithBookingPickupWeekdayExtension(clear func(context.Context, int64, int) error) BookingMaterializationOption {
	return func(options *services.BookingMaterializationTestOptions) { options.ClearPickupWeekdayExtension = clear }
}

// WithBookingToday pins the materialization's calendar day.
func WithBookingToday(today func() calendar.Date) BookingMaterializationOption {
	return func(options *services.BookingMaterializationTestOptions) { options.Today = today }
}

// NewBookingMaterialization composes Care Plan's booking materialization
// (#3560) over the test database with the owner bindings of the server.
func NewBookingMaterialization(tb testing.TB, db *bun.DB, options ...BookingMaterializationOption) services.BookingMaterializationTestModule {
	tb.Helper()
	var chosen services.BookingMaterializationTestOptions
	for _, option := range options {
		option(&chosen)
	}
	module, err := services.NewBookingMaterializationTestModule(db, testpkg.TenantRuntime(tb, db), chosen)
	require.NoError(tb, err)
	return module
}
