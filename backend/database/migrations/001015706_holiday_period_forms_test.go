package migrations

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// TestHolidayPeriodFormsGiveEveryPeriodItsVersionOne pins #5198 on the
// schema: the migration gives a period of before version 1 without
// questions and its registrations that version; every later period gets
// version 1 at commit unless its writer brought one; a version never
// changes under the request role; only the Träger sees its versions, a
// school transaction none.
func TestHolidayPeriodFormsGiveEveryPeriodItsVersionOne(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := context.Background()
	slug := fmt.Sprintf("traeger-formular-%d", testpkg.UniqueTestTenantID(t))
	own := testpkg.CreateTestOrganization(t, db, &testpkg.Organization{Name: "Träger Formular", Slug: slug}).ID
	testpkg.OwnTraegerHolidayPeriods(t, db, own)
	school := testpkg.CreateTestSchoolInOrganization(t, db, own, "Träger Formular OGS")

	require.NoError(t, holidayPeriodFormsDown(ctx, db))
	var before, registration int64
	require.NoError(t, db.NewRaw(`INSERT INTO enrollment.holiday_periods (organization_id, name, registration_opens_on, registration_closes_on, status, published_at)
		VALUES (?, 'Sommerferien', '2020-01-01', '2099-06-30', 'published', NOW()) RETURNING id`, own).Scan(ctx, &before))
	require.NoError(t, db.NewRaw(`INSERT INTO enrollment.holiday_registrations (organization_id, holiday_period_id, guardian_first_name,
			guardian_last_name, guardian_email, guardian_phone, privacy_consent_at)
		VALUES (?, ?, 'Eva', 'Muster', 'eva@example.org', '0123', NOW()) RETURNING id`, own, before).Scan(ctx, &registration))
	_, err := db.ExecContext(ctx, `INSERT INTO enrollment.holiday_registration_children (organization_id, holiday_period_id,
			holiday_registration_id, position, first_name, last_name, birth_date, home_school_id)
		VALUES (?, ?, ?, 1, 'Ben', 'Muster', '2016-09-12', ?)`, own, before, registration, school)
	require.NoError(t, err)
	require.NoError(t, holidayPeriodFormsUp(ctx, db))

	type version struct {
		Version int             `bun:"version"`
		Fields  json.RawMessage `bun:"fields,type:jsonb"`
	}
	versions := func(period int64) []version {
		var rows []version
		require.NoError(t, db.NewRaw(`SELECT version, fields FROM enrollment.holiday_period_forms
			WHERE holiday_period_id = ? ORDER BY version`, period).Scan(ctx, &rows))
		return rows
	}
	migrated := versions(before)
	require.Len(t, migrated, 1)
	assert.Equal(t, 1, migrated[0].Version)
	assert.JSONEq(t, `[]`, string(migrated[0].Fields), "a period of before keeps its form without questions")
	var kept struct {
		FormVersion int             `bun:"form_version"`
		CustomData  json.RawMessage `bun:"custom_data,type:jsonb"`
		Child       json.RawMessage `bun:"child,type:jsonb"`
	}
	require.NoError(t, db.NewRaw(`SELECT registration.form_version, registration.custom_data,
			(SELECT child.custom_data FROM enrollment.holiday_registration_children AS child
				WHERE child.holiday_registration_id = registration.id) AS child
		FROM enrollment.holiday_registrations AS registration WHERE registration.id = ?`, registration).Scan(ctx, &kept))
	assert.Equal(t, 1, kept.FormVersion, "a registration of before keeps version 1")
	assert.JSONEq(t, `{}`, string(kept.CustomData))
	assert.JSONEq(t, `{}`, string(kept.Child))

	var later int64
	require.NoError(t, db.NewRaw(`INSERT INTO enrollment.holiday_periods (organization_id, name, registration_opens_on, registration_closes_on)
		VALUES (?, 'Herbstferien', '2027-08-02', '2027-09-03') RETURNING id`, own).Scan(ctx, &later))
	assert.Len(t, versions(later), 1, "a period written without a version gets version 1 at commit")

	var brought int64
	require.NoError(t, db.RunInTx(ctx, &sql.TxOptions{}, func(ctx context.Context, tx bun.Tx) error {
		if err := tx.NewRaw(`INSERT INTO enrollment.holiday_periods (organization_id, name, registration_opens_on, registration_closes_on)
			VALUES (?, 'Winterferien', '2027-11-02', '2027-12-03') RETURNING id`, own).Scan(ctx, &brought); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, `INSERT INTO enrollment.holiday_period_forms (organization_id, holiday_period_id, version, fields)
			VALUES (?, ?, 1, '[{"key": "frage"}]'::jsonb)`, own, brought)
		return err
	}))
	broughtVersions := versions(brought)
	require.Len(t, broughtVersions, 1, "the trigger adds nothing to a period that brought its version")
	assert.JSONEq(t, `[{"key": "frage"}]`, string(broughtVersions[0].Fields))

	_, err = db.ExecContext(ctx, `DELETE FROM enrollment.holiday_period_forms WHERE holiday_period_id = ?`, before)
	assert.Error(t, err, "a version a registration keeps cannot go on its own")

	asRequestRole := func(setting, value string, fn func(context.Context, bun.Tx) error) error {
		return db.RunInTx(ctx, &sql.TxOptions{}, func(ctx context.Context, tx bun.Tx) error {
			if _, err := tx.ExecContext(ctx, `SET LOCAL ROLE phoenix_tenant`); err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, `SELECT set_config(?, ?, true)`, setting, value); err != nil {
				return err
			}
			return fn(ctx, tx)
		})
	}
	visible := func(setting, value string) int {
		var count int
		require.NoError(t, asRequestRole(setting, value, func(ctx context.Context, tx bun.Tx) error {
			return tx.NewRaw(`SELECT COUNT(*) FROM enrollment.holiday_period_forms WHERE organization_id = ?`, own).Scan(ctx, &count)
		}))
		return count
	}
	assert.Equal(t, 3, visible("app.current_org_id", fmt.Sprint(own)), "the Träger sees the versions of its periods")
	assert.Zero(t, visible("app.current_tenant_id", fmt.Sprint(school)), "a school transaction sees no version")
	for _, change := range []string{
		`UPDATE enrollment.holiday_period_forms SET fields = '[]'::jsonb WHERE organization_id = ?`,
		`DELETE FROM enrollment.holiday_period_forms WHERE organization_id = ?`,
	} {
		err := asRequestRole("app.current_org_id", fmt.Sprint(own), func(ctx context.Context, tx bun.Tx) error {
			_, err := tx.ExecContext(ctx, change, own)
			return err
		})
		assert.ErrorContains(t, err, "permission denied", "a version never changes: %s", change)
	}
	require.NoError(t, asRequestRole("app.current_org_id", fmt.Sprint(own), func(ctx context.Context, tx bun.Tx) error {
		var period int64
		if err := tx.NewRaw(`INSERT INTO enrollment.holiday_periods (organization_id, name, registration_opens_on, registration_closes_on)
			VALUES (?, 'Osterferien', '2028-02-01', '2028-03-01') RETURNING id`, own).Scan(ctx, &period); err != nil {
			return err
		}
		return nil
	}), "the trigger writes version 1 under the request role of the Träger")
	assert.Equal(t, 4, visible("app.current_org_id", fmt.Sprint(own)))

	_, err = db.ExecContext(ctx, `DELETE FROM enrollment.holiday_periods WHERE id = ?`, before)
	require.NoError(t, err, "a deleted period takes its versions and registrations along")
	assert.Empty(t, versions(before))
}
