package integration

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/modules/enrollment/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"

	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// childDaysPeriod is a published period of a Träger with one week by days
// and registrations of Ole Tag, 4 March 2018, by the family
// eva.tag@example.org and by others.
type childDaysPeriod struct {
	organization, period, week, model int64
	// first is the child row of the first registration of Ole.
	first int64
}

// childDaysBooking is one registration with one child and one booking.
type childDaysBooking struct {
	email, firstName, lastName, birthDate, status, placement, days string
}

func newChildDaysPeriod(t *testing.T, db *bun.DB, name string) childDaysPeriod {
	t.Helper()
	ctx := context.Background()
	organization := testpkg.CreateTestOrganization(t, db, &testpkg.Organization{
		Name: name, Slug: fmt.Sprintf("traeger-kindtage-%d", testpkg.UniqueTestTenantID(t)), Active: true,
	}).ID
	testpkg.OwnTraegerHolidayPeriods(t, db, organization)
	school := testpkg.CreateTestSchoolInOrganization(t, db, organization, name+" OGS")
	p := childDaysPeriod{organization: organization}
	require.NoError(t, db.NewRaw(`INSERT INTO enrollment.holiday_periods (organization_id, name, registration_opens_on,
			registration_closes_on, status, published_at, max_days_per_child) VALUES (?, 'Sommerferien', '2027-01-01', '2027-06-01', 'published', NOW(), 3)
		RETURNING id`, organization).Scan(ctx, &p.period))
	require.NoError(t, db.NewRaw(`INSERT INTO enrollment.holiday_period_weeks (organization_id, holiday_period_id, starts_on, ends_on, host_school_id, booking_unit)
		VALUES (?, ?, '2027-07-05', '2027-07-09', ?, 'day') RETURNING id`, organization, p.period, school).Scan(ctx, &p.week))
	require.NoError(t, db.NewRaw(`INSERT INTO enrollment.holiday_week_time_models (organization_id, holiday_period_week_id, name, starts_at, ends_at, price_cents)
		VALUES (?, ?, 'Ganztag', '08:00', '16:00', 1500) RETURNING id`, organization, p.week).Scan(ctx, &p.model))
	for i, b := range []childDaysBooking{
		{"Eva.Tag@Example.org ", "Ole", "Tag", "2018-03-04", "received", "placed", "{2027-07-05,2027-07-06}"},
		{"eva.tag@example.org", "ole", "TAG", "2018-03-04", "confirmed", "pending", "{2027-07-07}"},
		{"eva.tag@example.org", "Ole", "Tag", "2018-03-04", "received", "waitlisted", "{2027-07-08}"},
		// Not the same child: another family, a cancelled registration,
		// another date of birth.
		{"ina.tag@example.org", "Ole", "Tag", "2018-03-04", "received", "placed", "{2027-07-09}"},
		{"eva.tag@example.org", "Ole", "Tag", "2018-03-04", "cancelled", "placed", "{2027-07-09}"},
		{"eva.tag@example.org", "Ole", "Tag", "2019-03-04", "received", "placed", "{2027-07-09}"},
	} {
		child := p.insert(t, db, b)
		if i == 0 {
			p.first = child
		}
	}
	return p
}

func (p childDaysPeriod) insert(t *testing.T, db *bun.DB, b childDaysBooking) int64 {
	t.Helper()
	ctx := context.Background()
	var registration, child int64
	require.NoError(t, db.NewRaw(`INSERT INTO enrollment.holiday_registrations (organization_id, holiday_period_id, guardian_first_name,
			guardian_last_name, guardian_email, guardian_phone, privacy_consent_at, status)
		VALUES (?, ?, 'Eva', 'Tag', ?, '0123', NOW(), ?) RETURNING id`, p.organization, p.period, b.email, b.status).Scan(ctx, &registration))
	require.NoError(t, db.NewRaw(`INSERT INTO enrollment.holiday_registration_children (organization_id, holiday_period_id,
			holiday_registration_id, position, first_name, last_name, birth_date)
		VALUES (?, ?, ?, 1, ?, ?, ?) RETURNING id`, p.organization, p.period, registration, b.firstName, b.lastName, b.birthDate).Scan(ctx, &child))
	_, err := db.ExecContext(ctx, `INSERT INTO enrollment.holiday_registration_bookings (organization_id, holiday_period_id,
			holiday_registration_child_id, holiday_period_week_id, holiday_week_time_model_id, placement, days) VALUES (?, ?, ?, ?, ?, ?, ?::date[])`,
		p.organization, p.period, child, p.week, p.model, b.placement, b.days)
	require.NoError(t, err)
	return child
}

var oleTag = enrollment.NewHolidayChildKey(" EVA.TAG@example.org", "Ole ", "Tag", calendar.NewDate(2018, 3, 4))

// TestHolidayChildBookingsCountTheSameChildOverItsRegistrations pins the
// child the bound of care days counts across registrations (#6446): the
// same family and child in any case, placed, waiting and waiting for the
// office alike; not another family with the same name and date of birth,
// not a cancelled registration, not another date of birth. The child row
// being changed is left out.
func TestHolidayChildBookingsCountTheSameChildOverItsRegistrations(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	p := newChildDaysPeriod(t, db, "Träger Kindtage")
	store := postgres.NewHolidayRegistrationStore()

	ctx := testpkg.ContextWithTenantRuntime(context.Background(), testpkg.TenantRuntime(t, db))
	require.NoError(t, testpkg.WithinTraeger(ctx, p.organization, func(ctx context.Context) error {
		all, err := store.ReadHolidayChildBookings(ctx, p.organization, p.period, oleTag, 0)
		require.NoError(t, err)
		assert.Equal(t, []enrollment.HolidayBookingInput{
			{WeekID: p.week, TimeModelID: p.model, Days: []calendar.Date{calendar.NewDate(2027, 7, 5), calendar.NewDate(2027, 7, 6)}},
			{WeekID: p.week, TimeModelID: p.model, Days: []calendar.Date{calendar.NewDate(2027, 7, 7)}},
			{WeekID: p.week, TimeModelID: p.model, Days: []calendar.Date{calendar.NewDate(2027, 7, 8)}},
		}, all, "placed, pending and waiting bookings of the same family and child count")

		others, err := store.ReadHolidayChildBookings(ctx, p.organization, p.period, oleTag, p.first)
		require.NoError(t, err)
		assert.Len(t, others, 2, "the child row being rebooked is left out")
		return nil
	}))
}

// TestHolidayChildBookingsKeepTheirTraegerBeneathRLS pins the Träger filter
// of the statement beneath RLS (Punkt 25): in A's Träger transaction, as the
// owner of the tables, A asks for B's period with B's child. It goes red when
// the statement drops its organization_id filter.
func TestHolidayChildBookingsKeepTheirTraegerBeneathRLS(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	own := newChildDaysPeriod(t, db, "Träger Kindtage A")
	foreign := newChildDaysPeriod(t, db, "Träger Kindtage B")
	store := postgres.NewHolidayRegistrationStore()

	ctx := testpkg.ContextWithTenantRuntime(context.Background(), testpkg.TenantRuntime(t, db))
	require.NoError(t, testpkg.WithinTraeger(ctx, own.organization, func(ctx context.Context) error {
		require.NoError(t, testpkg.WithoutRequestRole(ctx))
		read, err := store.ReadHolidayChildBookings(ctx, own.organization, foreign.period, oleTag, 0)
		require.NoError(t, err)
		assert.Empty(t, read, "A counts no booking of B")
		return nil
	}))
}
