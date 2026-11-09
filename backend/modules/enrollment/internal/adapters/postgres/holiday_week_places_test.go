package postgres_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/modules/enrollment/internal/adapters/postgres"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// placesWeek is a published period of organization with one week of capacity
// one: a placed booking holds the place, a waitlisted one waits for it.
type placesWeek struct {
	organization, period, week, waiting int64
}

func newPlacesWeek(t *testing.T, db *bun.DB, name string) placesWeek {
	t.Helper()
	ctx := context.Background()
	organization := testpkg.CreateTestOrganization(t, db, &testpkg.Organization{
		Name: name, Slug: fmt.Sprintf("traeger-plaetze-%d", testpkg.UniqueTestTenantID(t)), Active: true,
	}).ID
	testpkg.OwnTraegerHolidayPeriods(t, db, organization)
	school := testpkg.CreateTestSchoolInOrganization(t, db, organization, name+" OGS")
	w := placesWeek{organization: organization}
	require.NoError(t, db.NewRaw(`INSERT INTO enrollment.holiday_periods (organization_id, name, registration_opens_on,
			registration_closes_on, status, published_at) VALUES (?, 'Sommerferien', '2027-01-01', '2027-06-01', 'published', NOW())
		RETURNING id`, organization).Scan(ctx, &w.period))
	require.NoError(t, db.NewRaw(`INSERT INTO enrollment.holiday_period_weeks (organization_id, holiday_period_id, starts_on, ends_on, host_school_id, capacity)
		VALUES (?, ?, '2027-07-05', '2027-07-09', ?, 1) RETURNING id`, organization, w.period, school).Scan(ctx, &w.week))
	var model int64
	require.NoError(t, db.NewRaw(`INSERT INTO enrollment.holiday_week_time_models (organization_id, holiday_period_week_id, name, starts_at, ends_at, price_cents)
		VALUES (?, ?, 'Ganztag', '08:00', '16:00', 4500) RETURNING id`, organization, w.week).Scan(ctx, &model))
	for i, placement := range []string{"placed", "waitlisted"} {
		var registration, child int64
		require.NoError(t, db.NewRaw(`INSERT INTO enrollment.holiday_registrations (organization_id, holiday_period_id, guardian_first_name,
				guardian_last_name, guardian_email, guardian_phone, privacy_consent_at)
			VALUES (?, ?, 'Eva', 'Platz', 'eva.platz@example.org', '0123', NOW()) RETURNING id`, organization, w.period).Scan(ctx, &registration))
		require.NoError(t, db.NewRaw(`INSERT INTO enrollment.holiday_registration_children (organization_id, holiday_period_id,
				holiday_registration_id, position, first_name, last_name, birth_date)
			VALUES (?, ?, ?, 1, ?, 'Platz', '2018-03-04') RETURNING id`, organization, w.period, registration, fmt.Sprintf("Kind%d", i)).Scan(ctx, &child))
		_, err := db.ExecContext(ctx, `INSERT INTO enrollment.holiday_registration_bookings (organization_id, holiday_period_id,
				holiday_registration_child_id, holiday_period_week_id, holiday_week_time_model_id, placement) VALUES (?, ?, ?, ?, ?, ?)`,
			organization, w.period, child, w.week, model, placement)
		require.NoError(t, err)
		w.waiting = registration
	}
	return w
}

// TestHolidayWeekPlacesKeepTheirTraegerBeneathRLS pins the Träger filter of
// every statement of the places (#4699) beneath RLS, as #5952 and #6001 do:
// in the Träger transaction of A, as the owner of the tables, each statement
// is asked for B's period and week. Each assertion goes red when its
// statement drops its organization_id filter.
func TestHolidayWeekPlacesKeepTheirTraegerBeneathRLS(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	own := newPlacesWeek(t, db, "Träger Plätze A")
	foreign := newPlacesWeek(t, db, "Träger Plätze B")
	store := postgres.NewHolidayRegistrationStore()
	nine := 9
	// B's week is unlimited: without its filter the placement would give
	// B's waiting booking a place.
	_, err := db.ExecContext(context.Background(), `UPDATE enrollment.holiday_period_weeks SET capacity = NULL WHERE id = ?`, foreign.week)
	require.NoError(t, err)

	ctx := testpkg.ContextWithTenantRuntime(context.Background(), testpkg.TenantRuntime(t, db))
	require.NoError(t, testpkg.WithinTraeger(ctx, own.organization, func(ctx context.Context) error {
		require.NoError(t, testpkg.WithoutRequestRole(ctx))
		places, err := store.ReadHolidayWeekPlaces(ctx, own.organization, foreign.period)
		require.NoError(t, err)
		assert.Empty(t, places, "A reads no week of B")

		locked, err := store.LockHolidayWeeks(ctx, own.organization, foreign.period, []int64{foreign.week})
		require.NoError(t, err)
		assert.Empty(t, locked, "A locks no week of B")

		require.NoError(t, store.SetHolidayWeekCapacity(ctx, own.organization, foreign.period, foreign.week, &nine))
		placed, err := store.PlaceHolidayWaitlist(ctx, own.organization, foreign.week, 0)
		require.NoError(t, err)
		assert.Empty(t, placed, "A moves no booking of B up")

		read, _, err := store.ReadHolidayReview(ctx, own.organization, foreign.period, foreign.waiting)
		require.NoError(t, err)
		assert.Empty(t, read, "A reads no registration of B with its place")
		return nil
	}))

	var capacity *int
	var waiting string
	require.NoError(t, db.NewRaw(`SELECT capacity FROM enrollment.holiday_period_weeks WHERE id = ?`, foreign.week).Scan(context.Background(), &capacity))
	assert.Nil(t, capacity, "A changes no capacity of B")
	require.NoError(t, db.NewRaw(`SELECT booking.placement FROM enrollment.holiday_registration_bookings AS booking
		JOIN enrollment.holiday_registration_children AS child ON child.id = booking.holiday_registration_child_id
		WHERE child.holiday_registration_id = ?`, foreign.waiting).Scan(context.Background(), &waiting))
	assert.Equal(t, "waitlisted", waiting)
}
