package integration

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	enrollmentCompose "github.com/moto-nrw/project-phoenix/modules/enrollment/compose"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// ownChildIdentities names school A's own child as its registration does.
type ownChildIdentities map[int64]enrollment.HolidayStudentIdentity

func (o ownChildIdentities) HolidayStudentIdentities(context.Context, []int64) (map[int64]enrollment.HolidayStudentIdentity, error) {
	return o, nil
}

func (o ownChildIdentities) UniqueHolidayStudents(_ context.Context, identities []enrollment.HolidayStudentIdentity) (map[int]int64, error) {
	result := map[int]int64{}
	for i, identity := range identities {
		for id, candidate := range o {
			if candidate == identity {
				result[i] = id
			}
		}
	}
	return result, nil
}

// TestHolidayBookingsOfAnotherSchoolStayOutBeneathRLS pins the school filter
// of the holiday bookings the Soll of Soll und Ist reads (#5994) beneath RLS,
// as #5952 and #6001 do for the Träger: as the owner of the tables, another
// school of the same Träger asks for the guest and the own child of school A
// and gets no booked day. School A gets both. The guest goes red when the
// statement drops `guest.tenant_id = ?`, the own child when the function
// drops its host school filter.
func TestHolidayBookingsOfAnotherSchoolStayOutBeneathRLS(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := tenant.WithUnitOfWork(context.Background(), testpkg.TenantRuntime(t, db))
	organization := testpkg.CreateTestOrganization(t, db, &testpkg.Organization{
		Name: "Träger Ferienfilter", Slug: fmt.Sprintf("traeger-ferienfilter-%d", testpkg.UniqueTestTenantID(t)), Active: true,
	}).ID
	testpkg.OwnTraegerHolidayPeriods(t, db, organization)
	schoolA := testpkg.Tenant(t)
	joinOrganization(t, db, organization, schoolA)
	schoolB := testpkg.CreateTestSchoolInOrganization(t, db, organization, "OGS Ferienfilter B")

	monday := calendar.NewDate(2026, 6, 8)
	guest := testpkg.CreateTestStudent(t, db, "Gina", "Gast", "Feriengast")
	_, err := db.ExecContext(context.Background(), `INSERT INTO enrollment.holiday_guests
		(tenant_id, organization_id, holiday_period_week_id, holiday_registration_child_id, student_id,
			starts_on, ends_on, time_model_name, time_model_starts_at, time_model_ends_at)
		VALUES (?, ?, 1, ?, ?, ?, ?, 'Ganztag', '08:00', '16:00')`,
		schoolA, organization, guest.ID, guest.ID, monday, monday.AddDays(4))
	require.NoError(t, err)
	hostOwnChild(t, db, organization, schoolA, monday)
	own := testpkg.CreateTestStudent(t, db, "Ole", "Eigen", "2a")
	care := enrollmentCompose.NewHolidayGuestBookedCare(ownChildIdentities{
		own.ID: {FirstName: "Ole", LastName: "Eigen", BirthDate: calendar.NewDate(2018, 3, 4)},
	})

	read := func(school int64, beneathRLS bool) (guestDays, ownDays int) {
		t.Helper()
		id, err := tenant.NewTenantID(school)
		require.NoError(t, err)
		require.NoError(t, tenant.WithinTenant(ctx, id, func(txCtx context.Context) error {
			if beneathRLS {
				if err := testpkg.WithoutRequestRole(txCtx); err != nil {
					return err
				}
			}
			days, err := care.HolidayGuestBookedDays(txCtx, []int64{guest.ID, own.ID}, monday, monday.AddDays(6))
			guestDays, ownDays = len(days[guest.ID]), len(days[own.ID])
			return err
		}))
		return guestDays, ownDays
	}

	guestDays, ownDays := read(schoolA, false)
	assert.Equal(t, 5, guestDays, "school A reads the week of its guest")
	assert.Equal(t, 5, ownDays, "school A reads the week of its own child")
	guestDays, ownDays = read(schoolB, true)
	assert.Zero(t, guestDays, "beneath RLS the statement's school filter keeps A's guest from B")
	assert.Zero(t, ownDays, "beneath RLS the function's host filter keeps A's own child from B")
}

// hostOwnChild lets school host a week of a published period of organization
// with one confirmed child whose home school it is.
func hostOwnChild(t *testing.T, db *bun.DB, organization, school int64, monday calendar.Date) {
	t.Helper()
	ctx := context.Background()
	var period, week, model, registration, child int64
	require.NoError(t, db.NewRaw(`INSERT INTO enrollment.holiday_periods (organization_id, name, registration_opens_on,
			registration_closes_on, status, published_at) VALUES (?, 'Sommerferien', '2026-01-01', '2026-06-01', 'published', NOW())
		RETURNING id`, organization).Scan(ctx, &period))
	require.NoError(t, db.NewRaw(`INSERT INTO enrollment.holiday_period_weeks (organization_id, holiday_period_id, starts_on, ends_on, host_school_id)
		VALUES (?, ?, ?, ?, ?) RETURNING id`, organization, period, monday, monday.AddDays(4), school).Scan(ctx, &week))
	require.NoError(t, db.NewRaw(`INSERT INTO enrollment.holiday_week_time_models (organization_id, holiday_period_week_id, name, starts_at, ends_at, price_cents)
		VALUES (?, ?, 'Ganztag', '08:00', '16:00', 4500) RETURNING id`, organization, week).Scan(ctx, &model))
	require.NoError(t, db.NewRaw(`INSERT INTO enrollment.holiday_registrations (organization_id, holiday_period_id, guardian_first_name,
			guardian_last_name, guardian_email, guardian_phone, privacy_consent_at, status)
		VALUES (?, ?, 'Eva', 'Eigen', 'eva@example.org', '0123', NOW(), 'confirmed') RETURNING id`, organization, period).Scan(ctx, &registration))
	require.NoError(t, db.NewRaw(`INSERT INTO enrollment.holiday_registration_children (organization_id, holiday_period_id,
			holiday_registration_id, position, first_name, last_name, birth_date, home_school_id)
		VALUES (?, ?, ?, 1, 'Ole', 'Eigen', '2018-03-04', ?) RETURNING id`, organization, period, registration, school).Scan(ctx, &child))
	_, err := db.ExecContext(ctx, `INSERT INTO enrollment.holiday_registration_bookings (organization_id, holiday_period_id,
			holiday_registration_child_id, holiday_period_week_id, holiday_week_time_model_id) VALUES (?, ?, ?, ?, ?)`,
		organization, period, child, week, model)
	require.NoError(t, err)
}

// joinOrganization moves the per-test school into organization for the
// test; the function joins the host school to the Träger of the week.
func joinOrganization(t *testing.T, db *bun.DB, organization, school int64) {
	t.Helper()
	var previous *int64
	require.NoError(t, db.NewRaw(`SELECT organization_id FROM platform.schools WHERE id = ?`, school).Scan(context.Background(), &previous))
	_, err := db.ExecContext(context.Background(), `UPDATE platform.schools SET organization_id = ? WHERE id = ?`, organization, school)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, err := db.ExecContext(context.Background(), `UPDATE platform.schools SET organization_id = ? WHERE id = ?`, previous, school)
		require.NoError(t, err)
	})
}
