package postgres_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-ogs/project-phoenix/modules/enrollment"
	ownerCompose "github.com/moto-ogs/project-phoenix/modules/enrollment/compose"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-ogs/project-phoenix/test"
)

// insertHostedPeriod writes a period of the Träger with one week hosted by
// school and returns both IDs; the period goes with the test.
func insertHostedPeriod(t *testing.T, ctx context.Context, tx bun.IDB, organization, school int64, startsOn string) (int64, int64) {
	t.Helper()
	t.Cleanup(func() {
		_, err := tx.ExecContext(context.Background(), `DELETE FROM enrollment.holiday_periods WHERE organization_id = ?`, organization)
		assert.NoError(t, err)
	})
	var period, week int64
	require.NoError(t, tx.NewRaw(`INSERT INTO enrollment.holiday_periods (organization_id, name, registration_opens_on, registration_closes_on)
		VALUES (?, 'Herbstferien', DATE '2029-09-01', DATE '2029-09-30') RETURNING id`, organization).Scan(ctx, &period))
	require.NoError(t, tx.NewRaw(`INSERT INTO enrollment.holiday_period_weeks (organization_id, holiday_period_id, starts_on, ends_on, host_school_id)
		VALUES (?, ?, ?::date, ?::date + 4, ?) RETURNING id`, organization, period, startsOn, startsOn, school).Scan(ctx, &week))
	return period, week
}

// The host schools of the weeks (#6624) below row level security (brief
// rule 25): the test runs without the request role, so only the Träger
// filter of the statement keeps the other Träger's period out.
func TestHolidayWeekHostsKeepTheTraegerBelowRLS(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	unique := testpkg.UniqueTestTenantID(t)
	own := testpkg.CreateTestOrganization(t, db, &testpkg.Organization{Name: "Träger Gastgeber A", Slug: fmt.Sprintf("hosts-a-%d", unique)}).ID
	other := testpkg.CreateTestOrganization(t, db, &testpkg.Organization{Name: "Träger Gastgeber B", Slug: fmt.Sprintf("hosts-b-%d", unique)}).ID
	ownSchool := testpkg.CreateTestSchoolInOrganization(t, db, own, "Schule A")
	otherSchool := testpkg.CreateTestSchoolInOrganization(t, db, other, "Schule B")
	period, week := insertHostedPeriod(t, context.Background(), db, own, ownSchool, "2029-10-01")
	foreign, _ := insertHostedPeriod(t, context.Background(), db, other, otherSchool, "2029-10-08")
	store := ownerCompose.NewHolidayWeekHosts()
	rollback := errors.New("leave the Träger transaction")
	ctx := testpkg.ContextWithTenantRuntime(context.Background(), testpkg.TenantRuntime(t, db))
	err := testpkg.WithinTraeger(ctx, own, func(ctx context.Context) error {
		require.NoError(t, testpkg.WithoutRequestRole(ctx))

		hosts, err := store.HolidayWeekHosts(ctx, own, period)
		require.NoError(t, err)
		assert.Equal(t, []enrollment.HolidayWeekHost{{WeekID: week, StartsOn: calendar.NewDate(2029, time.October, 1),
			EndsOn: calendar.NewDate(2029, time.October, 5), HostSchoolID: ownSchool}}, hosts)

		hosts, err = store.HolidayWeekHosts(ctx, own, foreign)
		require.NoError(t, err)
		assert.Empty(t, hosts, "a period of another Träger names no week, also without RLS")
		return rollback
	})
	require.ErrorIs(t, err, rollback)
}
