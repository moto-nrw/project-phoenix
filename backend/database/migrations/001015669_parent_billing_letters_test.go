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

// TestConfirmationLettersNeverChange pins ADR 0121 on the tables of the
// letters: under the request role a Träger reads only its own letters and a
// school transaction sees none; the database keeps the SHA-256 of every
// PDF; no role changes a letter or what the run wrote for it, and the
// request role deletes none; a run has its letters once; the trail knows
// the creation.
func TestConfirmationLettersNeverChange(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := context.Background()
	newTraeger := func(name string) (int64, int64) {
		slug := fmt.Sprintf("traeger-brief-%d", testpkg.UniqueTestTenantID(t))
		organization := testpkg.CreateTestOrganization(t, db, &testpkg.Organization{Name: name, Slug: slug}).ID
		testpkg.OwnTraegerHolidayPeriods(t, db, organization)
		testpkg.OwnTraegerContributionRules(t, db, organization)
		return organization, testpkg.CreateTestSchoolInOrganization(t, db, organization, name+" OGS")
	}
	own, ownSchool := newTraeger("Träger Brief Eigen")
	foreign, _ := newTraeger("Träger Brief Fremd")

	var run, document int64
	require.NoError(t, db.NewRaw(`INSERT INTO parent_billing.runs (organization_id, service, holiday_period_id, period_name, service_from,
			service_until, status, released_at, issued_on, document_count, total_cents)
		VALUES (?, 'holiday', 8, 'Sommerferien', '2027-07-05', '2027-07-16', 'released', NOW(), '2027-06-01', 1, 4500) RETURNING id`, own).
		Scan(ctx, &run))
	require.NoError(t, db.NewRaw(`INSERT INTO parent_billing.documents (organization_id, run_id, service, number, number_year, number_sequence,
			issued_on, holiday_period_id, holiday_registration_id, recipient_first_name, recipient_last_name, recipient_email,
			service_from, service_until, total_cents)
		VALUES (?, ?, 'holiday', 'FE-0001', 0, 1, '2027-06-01', 8, 9, 'Eva', 'Muster', 'eva@example.org', '2027-07-05', '2027-07-09', 4500)
		RETURNING id`, own, run).Scan(ctx, &document))

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
	pdf := []byte("%PDF-1.4 Bestätigung FE-0001")
	require.NoError(t, asRequestRole("app.current_org_id", fmt.Sprint(own), func(tx bun.Tx) error {
		if _, err := tx.ExecContext(ctx, `INSERT INTO parent_billing.run_letters (organization_id, run_id, payment_due_on, notes, emergency_contacts)
			VALUES (?, ?, '2027-06-15', 'Hausschuhe', '[{"name": "OGS", "phone": "0170 1"}]')`, own, run); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO parent_billing.document_letters (organization_id, document_id, run_id, filename, pdf)
			VALUES (?, ?, ?, 'bestaetigung-fe-0001.pdf', ?)`, own, document, run, pdf); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, `INSERT INTO parent_billing.run_events (organization_id, run_id, action) VALUES (?, ?, 'letters')`, own, run)
		return err
	}), "the request role adds the letters of its Träger and records them in the trail")

	var checksum string
	require.NoError(t, db.NewRaw(`SELECT sha256 FROM parent_billing.document_letters WHERE document_id = ?`, document).Scan(ctx, &checksum))
	// sha256sum of the bytes above.
	assert.Equal(t, "a952f05b50daaed0d6aefc67f71dd9ca4e1c409f27b6a369a95e02a48824a678", checksum,
		"the database keeps the SHA-256 of the PDF")

	_, err := db.ExecContext(ctx, `INSERT INTO parent_billing.run_letters (organization_id, run_id, payment_due_on) VALUES (?, ?, '2027-06-20')`, own, run)
	assert.ErrorContains(t, err, "run_letters_pkey", "a run has its letters once")

	count := func(table, setting, value string) int {
		var visible int
		require.NoError(t, asRequestRole(setting, value, func(tx bun.Tx) error {
			return tx.NewRaw(`SELECT COUNT(*) FROM `+table+` WHERE organization_id = ?`, own).Scan(ctx, &visible)
		}))
		return visible
	}
	for _, table := range []string{"parent_billing.run_letters", "parent_billing.document_letters"} {
		assert.Equal(t, 1, count(table, "app.current_org_id", fmt.Sprint(own)), table)
		assert.Zero(t, count(table, "app.current_org_id", fmt.Sprint(foreign)), "another Träger sees nothing of %s", table)
		assert.Zero(t, count(table, "app.current_tenant_id", fmt.Sprint(ownSchool)), "a school transaction sees nothing of %s", table)
	}

	for _, statement := range []string{
		`UPDATE parent_billing.run_letters SET notes = 'anders' WHERE organization_id = ?`,
		`UPDATE parent_billing.document_letters SET filename = 'anders.pdf' WHERE organization_id = ?`,
	} {
		err := asRequestRole("app.current_org_id", fmt.Sprint(own), func(tx bun.Tx) error {
			_, err := tx.ExecContext(ctx, statement, own)
			return err
		})
		require.Error(t, err, "the request role changes no letter: %s", statement)
		assert.Regexp(t, "permission denied|never changes", err.Error(), statement)
		_, err = db.ExecContext(ctx, statement, own)
		assert.ErrorContains(t, err, "never changes", "no role changes a letter: %s", statement)
	}
	for _, statement := range []string{
		`DELETE FROM parent_billing.run_letters WHERE organization_id = ?`,
		`DELETE FROM parent_billing.document_letters WHERE organization_id = ?`,
	} {
		err := asRequestRole("app.current_org_id", fmt.Sprint(own), func(tx bun.Tx) error {
			_, err := tx.ExecContext(ctx, statement, own)
			return err
		})
		assert.ErrorContains(t, err, "permission denied", "the request role deletes no letter: %s", statement)
	}
	err = asRequestRole("app.current_org_id", fmt.Sprint(foreign), func(tx bun.Tx) error {
		_, err := tx.ExecContext(ctx, `INSERT INTO parent_billing.document_letters (organization_id, document_id, run_id, filename, pdf)
			VALUES (?, ?, ?, 'x.pdf', ?)`, own, document, run, pdf)
		return err
	})
	assert.ErrorContains(t, err, "row-level security", "a Träger cannot add a letter to another Träger")
}
