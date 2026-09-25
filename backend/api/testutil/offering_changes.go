package testutil

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/services"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// OfferingChangeOption replaces one collaborator of Care Plan's
// offering-change review (#3561) a suite drives.
type OfferingChangeOption func(*services.OfferingChangeTestOptions)

// WithOfferingChangeCatalog intercepts the care-offering reads.
func WithOfferingChangeCatalog(catalog services.OfferingChangeTestCatalog) OfferingChangeOption {
	return func(options *services.OfferingChangeTestOptions) { options.Catalog = catalog }
}

// WithOfferingChangeToday pins the review's calendar day.
func WithOfferingChangeToday(today func() calendar.Date) OfferingChangeOption {
	return func(options *services.OfferingChangeTestOptions) { options.Today = today }
}

// NewOfferingChanges composes Care Plan's offering-change review over the
// test database with the owner bindings of the server. The settings answer
// the offering-change flags; approvals apply through bookings.
func NewOfferingChanges(
	tb testing.TB,
	db *bun.DB,
	settings interface {
		ResolveBool(context.Context, string) (bool, error)
		ResolveString(context.Context, string) (string, error)
	},
	bookings services.OfferingChangeTestBookings,
	options ...OfferingChangeOption,
) services.OfferingChangeTestCapability {
	tb.Helper()
	chosen := services.OfferingChangeTestOptions{Settings: settings, Bookings: bookings}
	for _, option := range options {
		option(&chosen)
	}
	changes, err := services.NewOfferingChangeTestModule(db, chosen)
	require.NoError(tb, err)
	return changes
}

// NewOfferingReviewQuery composes the staff review queue of offering
// changes (#3179) over the test database, reviewing the whole school on the
// given day.
func NewOfferingReviewQuery(tb testing.TB, db *bun.DB, today func() calendar.Date) services.OfferingReviewTestQuery {
	tb.Helper()
	query, err := services.NewOfferingReviewTestQuery(db, today)
	require.NoError(tb, err)
	return query
}
