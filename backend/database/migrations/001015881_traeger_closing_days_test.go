package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"testing"

	testpkg "github.com/moto-ogs/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// TestTraegerClosingDaysFollowTheTraegerPolicy pins the row security of the
// Träger's closing days (#4702): under the request role a Träger transaction
// sees and writes only its own Träger's rows, a school transaction reads only
// the rows naming that school and writes none, and only undeleted schools of
// the row's Träger can be named.
func TestTraegerClosingDaysFollowTheTraegerPolicy(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := context.Background()
	newTraeger := func(name string) (int64, int64, int64) {
		slug := fmt.Sprintf("traeger-schliesstage-%d", testpkg.UniqueTestTenantID(t))
		organization := testpkg.CreateTestOrganization(t, db, &testpkg.Organization{Name: name, Slug: slug}).ID
		testpkg.OwnTraegerClosingDays(t, db, organization)
		return organization, testpkg.CreateTestSchoolInOrganization(t, db, organization, name+" OGS A"),
			testpkg.CreateTestSchoolInOrganization(t, db, organization, name+" OGS B")
	}
	own, named, unnamed := newTraeger("Träger Schließtag Eigen")
	foreign, foreignSchool, _ := newTraeger("Träger Schließtag Fremd")

	var closingDay int64
	require.NoError(t, db.NewRaw(`INSERT INTO schedule.traeger_closing_days (organization_id, start_date, end_date, reason, school_ids)
		VALUES (?, '2027-05-14', '2027-05-14', 'Brückentag', ARRAY[?]::bigint[]) RETURNING id`, own, named).Scan(ctx, &closingDay))
	_, err := db.ExecContext(ctx, `INSERT INTO schedule.traeger_closing_days (organization_id, start_date, end_date, reason, school_ids)
		VALUES (?, '2027-05-14', '2027-05-14', 'Fremd', ARRAY[?]::bigint[])`, own, foreignSchool)
	assert.ErrorContains(t, err, "is not a school of Träger", "a foreign school cannot be named")
	_, err = db.ExecContext(ctx, `INSERT INTO schedule.traeger_closing_days (organization_id, start_date, end_date, reason, school_ids)
		VALUES (?, '2027-05-14', '2027-05-14', 'Doppelt', ARRAY[?, ?]::bigint[])`, own, named, named)
	assert.ErrorContains(t, err, "named twice", "a school is named once")
	_, err = db.ExecContext(ctx, `UPDATE schedule.traeger_closing_days SET school_ids = ARRAY[?]::bigint[] WHERE id = ?`, foreignSchool, closingDay)
	assert.ErrorContains(t, err, "is not a school of Träger", "an update cannot name a foreign school either")

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
			return tx.NewRaw(`SELECT COUNT(*) FROM schedule.traeger_closing_days WHERE organization_id = ?`, own).Scan(ctx, &visible)
		}))
		return visible
	}
	assert.Equal(t, 1, count("app.current_org_id", fmt.Sprint(own)), "the Träger sees its closing day")
	assert.Zero(t, count("app.current_org_id", fmt.Sprint(foreign)), "another Träger sees nothing")
	assert.Equal(t, 1, count("app.current_tenant_id", fmt.Sprint(named)), "the named school reads the closing day")
	assert.Zero(t, count("app.current_tenant_id", fmt.Sprint(unnamed)), "a school of the Träger that is not named reads nothing")
	assert.Zero(t, count("app.current_tenant_id", fmt.Sprint(foreignSchool)), "a school of another Träger reads nothing")

	err = asRequestRole("app.current_tenant_id", fmt.Sprint(named), func(tx bun.Tx) error {
		result, err := tx.ExecContext(ctx, `UPDATE schedule.traeger_closing_days SET reason = 'Schule' WHERE id = ?`, closingDay)
		if err != nil {
			return err
		}
		rows, err := result.RowsAffected()
		if err == nil && rows != 0 {
			return fmt.Errorf("the school changed %d rows", rows)
		}
		return err
	})
	assert.NoError(t, err, "a named school changes no row of its Träger's closing days")
	err = asRequestRole("app.current_tenant_id", fmt.Sprint(named), func(tx bun.Tx) error {
		_, err := tx.ExecContext(ctx, `INSERT INTO schedule.traeger_closing_days (organization_id, start_date, end_date, reason, school_ids)
			VALUES (?, '2027-06-01', '2027-06-01', 'Schule', ARRAY[?]::bigint[])`, own, named)
		return err
	})
	assert.Equal(t, "42501", testpkg.PostgresSQLState(err), "a school cannot write a closing day of its Träger")
	err = asRequestRole("app.current_org_id", fmt.Sprint(foreign), func(tx bun.Tx) error {
		_, err := tx.ExecContext(ctx, `INSERT INTO schedule.traeger_closing_days (organization_id, start_date, end_date, reason, school_ids)
			VALUES (?, '2027-06-01', '2027-06-01', 'Fremd', ARRAY[?]::bigint[])`, own, named)
		return err
	})
	assert.Equal(t, "42501", testpkg.PostgresSQLState(err), "a Träger cannot write a closing day for another Träger")
	_, err = db.ExecContext(ctx, `UPDATE platform.schools SET organization_id = ? WHERE id = ?`, foreign, named)
	require.NoError(t, err)
	assert.Zero(t, count("app.current_tenant_id", fmt.Sprint(named)), "a transferred school no longer reads the former Träger's closing days")
	assert.Equal(t, 1, count("app.current_org_id", fmt.Sprint(own)), "the former Träger keeps its own closing-day record")

}
