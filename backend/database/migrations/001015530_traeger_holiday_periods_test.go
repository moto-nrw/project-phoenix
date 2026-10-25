package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// TestHolidayPeriodsFollowTheTraegerPolicy pins the Träger policy of ADR 0105
// on the holiday period tables: under the request role a Träger transaction
// sees and writes only its own Träger's rows, a school transaction sees none,
// and only an undeleted school of the period's Träger hosts a week.
func TestHolidayPeriodsFollowTheTraegerPolicy(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := context.Background()
	newTraeger := func(name string) (int64, int64) {
		slug := fmt.Sprintf("traeger-ferien-%d", testpkg.UniqueTestTenantID(t))
		organization := testpkg.CreateTestOrganization(t, db, &testpkg.Organization{Name: name, Slug: slug}).ID
		testpkg.OwnTraegerHolidayPeriods(t, db, organization)
		return organization, testpkg.CreateTestSchoolInOrganization(t, db, organization, name+" OGS")
	}
	own, ownSchool := newTraeger("Träger Policy Eigen")
	foreign, foreignSchool := newTraeger("Träger Policy Fremd")

	var period int64
	require.NoError(t, db.NewRaw(`INSERT INTO enrollment.holiday_periods (organization_id, name, registration_opens_on, registration_closes_on)
		VALUES (?, 'Sommerferien', '2027-05-03', '2027-06-04') RETURNING id`, own).Scan(ctx, &period))
	_, err := db.ExecContext(ctx, `INSERT INTO enrollment.holiday_period_weeks (organization_id, holiday_period_id, starts_on, ends_on, host_school_id)
		VALUES (?, ?, '2027-07-05', '2027-07-09', ?)`, own, period, ownSchool)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `INSERT INTO enrollment.holiday_period_weeks (organization_id, holiday_period_id, starts_on, ends_on, host_school_id)
		VALUES (?, ?, '2027-07-12', '2027-07-16', ?)`, own, period, foreignSchool)
	assert.ErrorContains(t, err, "is not a school of Träger", "a foreign school hosts no week")
	_, err = db.ExecContext(ctx, `INSERT INTO enrollment.holiday_period_weeks (organization_id, holiday_period_id, starts_on, ends_on, host_school_id)
		VALUES (?, ?, '2027-07-12', '2027-07-16', ?)`, foreign, period, foreignSchool)
	assert.Error(t, err, "a week cannot join the period of another Träger")

	asRequestRole := func(setting, value string, fn func(bun.Tx) error) error {
		return db.RunInTx(ctx, &sql.TxOptions{}, func(ctx context.Context, tx bun.Tx) error {
			if _, err := tx.ExecContext(ctx, `SET LOCAL ROLE phoenix_tenant`); err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, `SELECT set_config(?, ?, true)`, setting, value); err != nil {
				return err
			}
			return fn(tx)
		})
	}
	count := func(setting, value string) int {
		var visible int
		require.NoError(t, asRequestRole(setting, value, func(tx bun.Tx) error {
			return tx.NewRaw(`SELECT
				(SELECT COUNT(*) FROM enrollment.holiday_periods WHERE organization_id = ?)
				+ (SELECT COUNT(*) FROM enrollment.holiday_period_weeks WHERE organization_id = ?)`, own, own).Scan(ctx, &visible)
		}))
		return visible
	}
	assert.Equal(t, 2, count("app.current_org_id", fmt.Sprint(own)), "the Träger sees its period and week")
	assert.Zero(t, count("app.current_org_id", fmt.Sprint(foreign)), "another Träger sees nothing")
	assert.Zero(t, count("app.current_tenant_id", fmt.Sprint(ownSchool)), "a school transaction sees nothing of its Träger's periods")

	err = asRequestRole("app.current_org_id", fmt.Sprint(foreign), func(tx bun.Tx) error {
		_, err := tx.ExecContext(ctx, `INSERT INTO enrollment.holiday_periods (organization_id, name, registration_opens_on, registration_closes_on)
			VALUES (?, 'Fremd', '2027-05-03', '2027-06-04')`, own)
		return err
	})
	assert.ErrorContains(t, err, "row-level security", "a Träger cannot write a period for another Träger")
}
