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

// TestParentBillingTablesFollowTheTraegerPolicy pins ADR 0115 on the first
// tables of the Elternabrechnung: under the request role a Träger reads and
// writes only its own rules, a school transaction sees none; rules of one
// kind never overlap; a frozen contribution can be read and inserted but
// never changed or deleted, and it goes with the child of its registration.
func TestParentBillingTablesFollowTheTraegerPolicy(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := context.Background()
	newTraeger := func(name string) (int64, int64) {
		slug := fmt.Sprintf("traeger-beitrag-%d", testpkg.UniqueTestTenantID(t))
		organization := testpkg.CreateTestOrganization(t, db, &testpkg.Organization{Name: name, Slug: slug}).ID
		testpkg.OwnTraegerHolidayPeriods(t, db, organization)
		testpkg.OwnTraegerContributionRules(t, db, organization)
		return organization, testpkg.CreateTestSchoolInOrganization(t, db, organization, name+" OGS")
	}
	own, ownSchool := newTraeger("Träger Beitrag Eigen")
	foreign, _ := newTraeger("Träger Beitrag Fremd")

	_, err := db.ExecContext(ctx, `INSERT INTO parent_billing.contribution_rules
			(organization_id, service, kind, valid_from, sibling_from_child, discount_basis_points)
		VALUES (?, 'holiday', 'sibling', '2027-01-01', 2, 5000)`, own)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `INSERT INTO parent_billing.contribution_rules
			(organization_id, service, kind, valid_from, valid_until, sibling_from_child, discount_cents)
		VALUES (?, 'holiday', 'sibling', '2026-01-01', '2027-01-01', 2, 1000)`, own)
	assert.ErrorContains(t, err, "excl_contribution_rules_overlap", "two sibling rules may not be valid on the same day")
	_, err = db.ExecContext(ctx, `INSERT INTO parent_billing.contribution_rules
			(organization_id, service, kind, valid_from, sibling_from_child, discount_basis_points, discount_cents)
		VALUES (?, 'holiday', 'sibling', '2028-01-01', 2, 5000, 1000)`, own)
	assert.ErrorContains(t, err, "chk_contribution_rules_sibling", "a sibling rule has a share or an amount")
	_, err = db.ExecContext(ctx, `INSERT INTO parent_billing.contribution_rules (organization_id, service, kind, valid_from)
		VALUES (?, 'holiday', 'benefits_exemption', '2027-01-01')`, own)
	assert.ErrorContains(t, err, "chk_contribution_rules_exemption", "an exemption carries its note")

	var period, registration, child int64
	require.NoError(t, db.NewRaw(`INSERT INTO enrollment.holiday_periods (organization_id, name, registration_opens_on, registration_closes_on)
		VALUES (?, 'Sommerferien', '2027-05-03', '2027-06-04') RETURNING id`, own).Scan(ctx, &period))
	require.NoError(t, db.NewRaw(`INSERT INTO enrollment.holiday_registrations (organization_id, holiday_period_id, guardian_first_name,
			guardian_last_name, guardian_email, guardian_phone, privacy_consent_at, benefits_declared)
		VALUES (?, ?, 'Eva', 'Muster', 'eva@example.org', '0123', NOW(), true) RETURNING id`, own, period).Scan(ctx, &registration))
	require.NoError(t, db.NewRaw(`INSERT INTO enrollment.holiday_registration_children (organization_id, holiday_period_id,
			holiday_registration_id, position, first_name, last_name, birth_date)
		VALUES (?, ?, ?, 1, 'Lia', 'Muster', '2018-03-01') RETURNING id`, own, period, registration).Scan(ctx, &child))

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
	var contribution int64
	require.NoError(t, asRequestRole("app.current_org_id", fmt.Sprint(own), func(tx bun.Tx) error {
		if err := tx.NewRaw(`INSERT INTO parent_billing.holiday_contributions (organization_id, holiday_period_id,
				holiday_registration_id, holiday_registration_child_id, total_cents)
			VALUES (?, ?, ?, ?, 4500) RETURNING id`, own, period, registration, child).Scan(ctx, &contribution); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, `INSERT INTO parent_billing.holiday_contribution_lines (organization_id, holiday_contribution_id,
				position, kind, amount_cents, week_starts_on, week_ends_on, time_model_name)
			VALUES (?, ?, 1, 'base', 4500, '2027-07-05', '2027-07-09', 'bis 13 Uhr')`, own, contribution)
		return err
	}), "the Träger freezes its own contribution")

	count := func(table, setting, value string) int {
		var visible int
		require.NoError(t, asRequestRole(setting, value, func(tx bun.Tx) error {
			return tx.NewRaw(`SELECT COUNT(*) FROM `+table+` WHERE organization_id = ?`, own).Scan(ctx, &visible)
		}))
		return visible
	}
	for _, table := range []string{"parent_billing.contribution_rules", "parent_billing.holiday_contributions", "parent_billing.holiday_contribution_lines"} {
		assert.Equal(t, 1, count(table, "app.current_org_id", fmt.Sprint(own)), table)
		assert.Zero(t, count(table, "app.current_org_id", fmt.Sprint(foreign)), "another Träger sees nothing of %s", table)
		assert.Zero(t, count(table, "app.current_tenant_id", fmt.Sprint(ownSchool)), "a school transaction sees nothing of %s", table)
	}

	for _, statement := range []string{
		`UPDATE parent_billing.holiday_contributions SET total_cents = 0 WHERE organization_id = ?`,
		`DELETE FROM parent_billing.holiday_contributions WHERE organization_id = ?`,
		`UPDATE parent_billing.holiday_contribution_lines SET amount_cents = 0 WHERE organization_id = ?`,
	} {
		err = asRequestRole("app.current_org_id", fmt.Sprint(own), func(tx bun.Tx) error {
			_, err := tx.ExecContext(ctx, statement, own)
			return err
		})
		assert.ErrorContains(t, err, "permission denied", "a frozen contribution never changes: %s", statement)
	}
	err = asRequestRole("app.current_org_id", fmt.Sprint(foreign), func(tx bun.Tx) error {
		_, err := tx.ExecContext(ctx, `INSERT INTO parent_billing.contribution_rules (organization_id, service, kind, valid_from, note)
			VALUES (?, 'holiday', 'benefits_exemption', '2027-01-01', 'Vermerk')`, own)
		return err
	})
	assert.ErrorContains(t, err, "row-level security", "a Träger cannot write a rule of another Träger")

	_, err = db.ExecContext(ctx, `DELETE FROM enrollment.holiday_registrations WHERE id = ?`, registration)
	require.NoError(t, err)
	var left int
	require.NoError(t, db.NewRaw(`SELECT COUNT(*) FROM parent_billing.holiday_contribution_lines WHERE organization_id = ?`, own).Scan(ctx, &left))
	assert.Zero(t, left, "the frozen contribution goes with the child of its registration")
}
