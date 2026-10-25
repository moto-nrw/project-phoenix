package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// TestHolidayRegistrationAuditFollowsTheTraegerPolicy pins ADR 0114 on the
// audit trail of the holiday registrations: under the request role a Träger
// transaction reads and appends only its own entries, cannot change one, a
// school transaction sees none; a list names no registration, a change
// names one; the new states and freeze overrides are the only ones allowed.
func TestHolidayRegistrationAuditFollowsTheTraegerPolicy(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := context.Background()
	newTraeger := func(name string) (int64, int64) {
		slug := fmt.Sprintf("traeger-pruefung-%d", testpkg.UniqueTestTenantID(t))
		organization := testpkg.CreateTestOrganization(t, db, &testpkg.Organization{Name: name, Slug: slug}).ID
		testpkg.OwnTraegerHolidayPeriods(t, db, organization)
		return organization, testpkg.CreateTestSchoolInOrganization(t, db, organization, name+" OGS")
	}
	own, ownSchool := newTraeger("Träger Prüfung Eigen")
	foreign, _ := newTraeger("Träger Prüfung Fremd")

	var period, registration int64
	require.NoError(t, db.NewRaw(`INSERT INTO enrollment.holiday_periods (organization_id, name, registration_opens_on, registration_closes_on)
		VALUES (?, 'Herbstferien', '2027-08-02', '2027-09-03') RETURNING id`, own).Scan(ctx, &period))
	require.NoError(t, db.NewRaw(`INSERT INTO enrollment.holiday_registrations (organization_id, holiday_period_id, guardian_first_name,
			guardian_last_name, guardian_email, guardian_phone, privacy_consent_at)
		VALUES (?, ?, 'Eva', 'Muster', 'eva@example.org', '0123', NOW()) RETURNING id`, own, period).Scan(ctx, &registration))
	for _, status := range []string{"confirmed", "cancelled", "received"} {
		_, err := db.ExecContext(ctx, `UPDATE enrollment.holiday_registrations SET status = ? WHERE id = ?`, status, registration)
		require.NoError(t, err, status)
	}
	_, err := db.ExecContext(ctx, `UPDATE enrollment.holiday_registrations SET status = 'waiting' WHERE id = ?`, registration)
	assert.ErrorContains(t, err, "chk_holiday_registrations_status")
	for _, override := range []string{"frozen", "reopened"} {
		_, err = db.ExecContext(ctx, `UPDATE enrollment.holiday_registrations SET freeze_override = ? WHERE id = ?`, override, registration)
		require.NoError(t, err, override)
	}
	_, err = db.ExecContext(ctx, `UPDATE enrollment.holiday_registrations SET freeze_override = 'open' WHERE id = ?`, registration)
	assert.ErrorContains(t, err, "chk_holiday_registrations_freeze_override")

	_, err = db.ExecContext(ctx, `INSERT INTO enrollment.holiday_registration_audit (organization_id, holiday_period_id, action)
		VALUES (?, ?, 'confirm')`, own, period)
	assert.ErrorContains(t, err, "chk_holiday_registration_audit_subject", "a change names its registration")
	_, err = db.ExecContext(ctx, `INSERT INTO enrollment.holiday_registration_audit (organization_id, holiday_period_id, holiday_registration_id, action)
		VALUES (?, ?, ?, 'list')`, own, period, registration)
	assert.ErrorContains(t, err, "chk_holiday_registration_audit_subject", "a list names no registration")

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
	require.NoError(t, asRequestRole("app.current_org_id", fmt.Sprint(own), func(tx bun.Tx) error {
		if _, err := tx.ExecContext(ctx, `INSERT INTO enrollment.holiday_registration_audit
				(organization_id, holiday_period_id, holiday_registration_id, action) VALUES (?, ?, ?, 'confirm')`, own, period, registration); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, `INSERT INTO enrollment.holiday_registration_audit
				(organization_id, holiday_period_id, action) VALUES (?, ?, 'list')`, own, period)
		return err
	}), "the Träger appends to its own trail")
	count := func(setting, value string) int {
		var visible int
		require.NoError(t, asRequestRole(setting, value, func(tx bun.Tx) error {
			return tx.NewRaw(`SELECT COUNT(*) FROM enrollment.holiday_registration_audit WHERE organization_id = ?`, own).Scan(ctx, &visible)
		}))
		return visible
	}
	assert.Equal(t, 2, count("app.current_org_id", fmt.Sprint(own)))
	assert.Zero(t, count("app.current_org_id", fmt.Sprint(foreign)), "another Träger sees nothing")
	assert.Zero(t, count("app.current_tenant_id", fmt.Sprint(ownSchool)), "a school transaction sees nothing")

	err = asRequestRole("app.current_org_id", fmt.Sprint(own), func(tx bun.Tx) error {
		_, err := tx.ExecContext(ctx, `UPDATE enrollment.holiday_registration_audit SET details = '{}'::jsonb WHERE organization_id = ?`, own)
		return err
	})
	assert.ErrorContains(t, err, "permission denied", "the trail is append-only for the request role")
	err = asRequestRole("app.current_org_id", fmt.Sprint(foreign), func(tx bun.Tx) error {
		_, err := tx.ExecContext(ctx, `INSERT INTO enrollment.holiday_registration_audit (organization_id, holiday_period_id, action)
			VALUES (?, ?, 'list')`, own, period)
		return err
	})
	assert.ErrorContains(t, err, "row-level security", "a Träger cannot write into another Träger's trail")

	_, err = db.ExecContext(ctx, `DELETE FROM enrollment.holiday_registrations WHERE id = ?`, registration)
	require.NoError(t, err)
	var left int
	require.NoError(t, db.NewRaw(`SELECT COUNT(*) FROM enrollment.holiday_registration_audit WHERE organization_id = ?`, own).Scan(ctx, &left))
	assert.Equal(t, 1, left, "the history goes with its registration, the list entry stays with the period")
}

// TestHolidayRegistrationDuplicateLookupUsesItsIndex pins the index behind the
// duplicate hint (#4663): the lookup of a period's children by name and date
// of birth is an index scan, not a scan of every child of the period.
func TestHolidayRegistrationDuplicateLookupUsesItsIndex(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := context.Background()
	var plan []string
	require.NoError(t, db.RunInTx(ctx, &sql.TxOptions{}, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.ExecContext(ctx, `SET LOCAL enable_seqscan = off`); err != nil {
			return err
		}
		return tx.NewRaw(`EXPLAIN (GENERIC_PLAN) SELECT holiday_registration_id FROM enrollment.holiday_registration_children
			WHERE organization_id = $1 AND holiday_period_id = $2
				AND lower(first_name) = $3 AND lower(last_name) = $4 AND birth_date = $5`).Scan(ctx, &plan)
	}))
	assert.Contains(t, strings.Join(plan, "\n"), "idx_holiday_registration_children_duplicate")
}
