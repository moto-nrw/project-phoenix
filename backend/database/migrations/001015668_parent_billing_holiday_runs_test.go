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

// TestIssuedInvoicesNeverChange pins ADR 0119 on the tables of the billing
// run: under the request role a Träger reads only its own runs and invoices
// and a school transaction sees none; an issued invoice, its positions and
// a released run refuse every update, also for the migration role, and the
// request role may delete none of them; a period keeps one calculated run.
func TestIssuedInvoicesNeverChange(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := context.Background()
	newTraeger := func(name string) (int64, int64) {
		slug := fmt.Sprintf("traeger-lauf-%d", testpkg.UniqueTestTenantID(t))
		organization := testpkg.CreateTestOrganization(t, db, &testpkg.Organization{Name: name, Slug: slug}).ID
		testpkg.OwnTraegerHolidayPeriods(t, db, organization)
		testpkg.OwnTraegerContributionRules(t, db, organization)
		return organization, testpkg.CreateTestSchoolInOrganization(t, db, organization, name+" OGS")
	}
	own, ownSchool := newTraeger("Träger Lauf Eigen")
	foreign, _ := newTraeger("Träger Lauf Fremd")

	var run, released, document int64
	require.NoError(t, db.NewRaw(`INSERT INTO parent_billing.runs (organization_id, service, holiday_period_id, period_name, service_from, service_until)
		VALUES (?, 'holiday', 7, 'Winterferien', '2027-01-04', '2027-01-15') RETURNING id`, own).Scan(ctx, &run))
	_, err := db.ExecContext(ctx, `INSERT INTO parent_billing.runs (organization_id, service, holiday_period_id, period_name, service_from, service_until)
		VALUES (?, 'holiday', 7, 'Winterferien', '2027-01-04', '2027-01-15')`, own)
	assert.ErrorContains(t, err, "uq_runs_calculated_period", "a period keeps one calculated run")
	require.NoError(t, db.NewRaw(`INSERT INTO parent_billing.runs (organization_id, service, holiday_period_id, period_name, service_from,
			service_until, status, released_at, issued_on, document_count, total_cents)
		VALUES (?, 'holiday', 8, 'Sommerferien', '2027-07-05', '2027-07-16', 'released', NOW(), '2027-06-01', 1, 4500) RETURNING id`, own).
		Scan(ctx, &released))
	require.NoError(t, db.NewRaw(`INSERT INTO parent_billing.documents (organization_id, run_id, service, number, number_year, number_sequence,
			issued_on, holiday_period_id, holiday_registration_id, recipient_first_name, recipient_last_name, recipient_email,
			service_from, service_until, total_cents)
		VALUES (?, ?, 'holiday', 'FE-0001', 0, 1, '2027-06-01', 8, 9, 'Eva', 'Muster', 'eva@example.org', '2027-07-05', '2027-07-09', 4500)
		RETURNING id`, own, released).Scan(ctx, &document))
	_, err = db.ExecContext(ctx, `INSERT INTO parent_billing.document_positions (organization_id, document_id, position,
			holiday_registration_child_id, child_first_name, child_last_name, kind, amount_cents, net_cents, week_starts_on, week_ends_on, time_model_name)
		VALUES (?, ?, 1, 10, 'Lia', 'Muster', 'base', 4500, 4500, '2027-07-05', '2027-07-09', 'bis 13 Uhr')`, own, document)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `INSERT INTO parent_billing.document_positions (organization_id, document_id, position,
			holiday_registration_child_id, child_first_name, child_last_name, kind, amount_cents, net_cents)
		VALUES (?, ?, 2, 10, 'Lia', 'Muster', 'correction', -500, -500)`, own, document)
	assert.ErrorContains(t, err, "chk_document_positions_correction", "a correction carries its reason")

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
	count := func(table, setting, value string) int {
		var visible int
		require.NoError(t, asRequestRole(setting, value, func(tx bun.Tx) error {
			return tx.NewRaw(`SELECT COUNT(*) FROM `+table+` WHERE organization_id = ?`, own).Scan(ctx, &visible)
		}))
		return visible
	}
	for _, table := range []string{"parent_billing.runs", "parent_billing.documents", "parent_billing.document_positions"} {
		assert.NotZero(t, count(table, "app.current_org_id", fmt.Sprint(own)), table)
		assert.Zero(t, count(table, "app.current_org_id", fmt.Sprint(foreign)), "another Träger sees nothing of %s", table)
		assert.Zero(t, count(table, "app.current_tenant_id", fmt.Sprint(ownSchool)), "a school transaction sees nothing of %s", table)
	}

	for _, statement := range []string{
		`UPDATE parent_billing.documents SET total_cents = 0 WHERE organization_id = ?`,
		`UPDATE parent_billing.document_positions SET amount_cents = 0, net_cents = 0 WHERE organization_id = ?`,
		`UPDATE parent_billing.runs SET total_cents = 0 WHERE status = 'released' AND organization_id = ?`,
	} {
		err := asRequestRole("app.current_org_id", fmt.Sprint(own), func(tx bun.Tx) error {
			_, err := tx.ExecContext(ctx, statement, own)
			return err
		})
		require.Error(t, err, "the request role changes no issued invoice: %s", statement)
		assert.Regexp(t, "permission denied|never changes", err.Error(), statement)
		_, err = db.ExecContext(ctx, statement, own)
		assert.ErrorContains(t, err, "never changes", "no role changes an issued invoice: %s", statement)
	}
	for _, statement := range []string{
		`DELETE FROM parent_billing.documents WHERE organization_id = ?`,
		`DELETE FROM parent_billing.document_positions WHERE organization_id = ?`,
		`DELETE FROM parent_billing.runs WHERE organization_id = ?`,
		`DELETE FROM parent_billing.run_events WHERE organization_id = ?`,
		`DELETE FROM parent_billing.run_corrections WHERE organization_id = ?`,
	} {
		err := asRequestRole("app.current_org_id", fmt.Sprint(own), func(tx bun.Tx) error {
			_, err := tx.ExecContext(ctx, statement, own)
			return err
		})
		assert.ErrorContains(t, err, "permission denied", "the request role deletes nothing of a run: %s", statement)
	}
	require.NoError(t, asRequestRole("app.current_org_id", fmt.Sprint(own), func(tx bun.Tx) error {
		_, err := tx.ExecContext(ctx, `UPDATE parent_billing.runs SET period_name = 'Winterferien 2027' WHERE id = ?`, run)
		return err
	}), "a calculated run may still change")
	err = asRequestRole("app.current_org_id", fmt.Sprint(foreign), func(tx bun.Tx) error {
		_, err := tx.ExecContext(ctx, `INSERT INTO parent_billing.number_ranges (organization_id, service, pattern) VALUES (?, 'holiday', 'FE-{NNNN}')`, own)
		return err
	})
	assert.ErrorContains(t, err, "row-level security", "a Träger cannot set the number range of another Träger")
}
