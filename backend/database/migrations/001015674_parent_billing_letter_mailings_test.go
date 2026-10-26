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

// TestLetterMailingsStayWithTheirTraeger pins ADR 0123 on the sends of the
// letters: under the request role a Träger reads and adds only sends of its
// own letters, a school transaction sees none, no send changes or goes, a
// send needs a stored letter and an address, and each outbox entry is one
// send.
func TestLetterMailingsStayWithTheirTraeger(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := context.Background()
	newTraeger := func(name string) (int64, int64) {
		slug := fmt.Sprintf("traeger-mail-%d", testpkg.UniqueTestTenantID(t))
		organization := testpkg.CreateTestOrganization(t, db, &testpkg.Organization{Name: name, Slug: slug}).ID
		testpkg.OwnTraegerHolidayPeriods(t, db, organization)
		testpkg.OwnTraegerContributionRules(t, db, organization)
		return organization, testpkg.CreateTestSchoolInOrganization(t, db, organization, name+" OGS")
	}
	own, ownSchool := newTraeger("Träger Mail Eigen")
	foreign, _ := newTraeger("Träger Mail Fremd")

	var run, document, unlettered int64
	require.NoError(t, db.NewRaw(`INSERT INTO parent_billing.runs (organization_id, service, holiday_period_id, period_name, service_from,
			service_until, status, released_at, issued_on, document_count, total_cents)
		VALUES (?, 'holiday', 8, 'Sommerferien', '2027-07-05', '2027-07-16', 'released', NOW(), '2027-06-01', 2, 9000) RETURNING id`, own).
		Scan(ctx, &run))
	insertDocument := func(number string, sequence int) int64 {
		var id int64
		require.NoError(t, db.NewRaw(`INSERT INTO parent_billing.documents (organization_id, run_id, service, number, number_year, number_sequence,
				issued_on, holiday_period_id, holiday_registration_id, recipient_first_name, recipient_last_name, recipient_email,
				service_from, service_until, total_cents)
			VALUES (?, ?, 'holiday', ?, 0, ?, '2027-06-01', 8, ?, 'Eva', 'Muster', 'eva@example.org', '2027-07-05', '2027-07-09', 4500)
			RETURNING id`, own, run, number, sequence, sequence+10).Scan(ctx, &id))
		return id
	}
	document, unlettered = insertDocument("FE-0001", 1), insertDocument("FE-0002", 2)
	_, err := db.ExecContext(ctx, `INSERT INTO parent_billing.run_letters (organization_id, run_id, payment_due_on) VALUES (?, ?, '2027-06-15')`, own, run)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `INSERT INTO parent_billing.document_letters (organization_id, document_id, run_id, filename, pdf)
		VALUES (?, ?, ?, 'bestaetigung-fe-0001.pdf', ?)`, own, document, run, []byte("%PDF-1.4 FE-0001"))
	require.NoError(t, err)

	asRequestRole := func(setting, value, statement string, args ...any) error {
		return db.RunInTx(ctx, &sql.TxOptions{}, func(ctx context.Context, tx bun.Tx) error {
			if _, err := tx.ExecContext(ctx, `SET LOCAL ROLE phoenix_tenant`); err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, `SELECT set_config(?, ?, true)`, setting, value); err != nil {
				return err
			}
			_, err := tx.ExecContext(ctx, statement, args...)
			return err
		})
	}
	const insertSend = `INSERT INTO parent_billing.document_mailings (organization_id, run_id, document_id, delivery_id, recipient_email)
		VALUES (?, ?, ?, ?, ?)`
	require.NoError(t, asRequestRole("app.current_org_id", fmt.Sprint(own), insertSend, own, run, document, 101, "eva@example.org"),
		"the request role adds a send of a letter of its Träger")
	require.NoError(t, asRequestRole("app.current_org_id", fmt.Sprint(own), insertSend, own, run, document, 102, "eva@example.org"),
		"a letter is sent again with a new outbox entry")

	assert.ErrorContains(t, asRequestRole("app.current_org_id", fmt.Sprint(own), insertSend, own, run, document, 101, "eva@example.org"),
		"uq_document_mailings_delivery", "an outbox entry is one send")
	assert.ErrorContains(t, asRequestRole("app.current_org_id", fmt.Sprint(own), insertSend, own, run, unlettered, 103, "eva@example.org"),
		"fk_document_mailings_letter", "a send needs a stored letter")
	assert.ErrorContains(t, asRequestRole("app.current_org_id", fmt.Sprint(own), insertSend, own, run, document, 104, " "),
		"chk_document_mailings_recipient", "a send needs an address")
	assert.ErrorContains(t, asRequestRole("app.current_org_id", fmt.Sprint(foreign), insertSend, own, run, document, 105, "eva@example.org"),
		"row-level security", "another Träger adds no send to this Träger")

	count := func(setting, value string) int {
		var visible int
		require.NoError(t, db.RunInTx(ctx, &sql.TxOptions{}, func(ctx context.Context, tx bun.Tx) error {
			if _, err := tx.ExecContext(ctx, `SET LOCAL ROLE phoenix_tenant`); err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, `SELECT set_config(?, ?, true)`, setting, value); err != nil {
				return err
			}
			return tx.NewRaw(`SELECT COUNT(*) FROM parent_billing.document_mailings WHERE organization_id = ?`, own).Scan(ctx, &visible)
		}))
		return visible
	}
	assert.Equal(t, 2, count("app.current_org_id", fmt.Sprint(own)))
	assert.Zero(t, count("app.current_org_id", fmt.Sprint(foreign)), "another Träger sees no send")
	assert.Zero(t, count("app.current_tenant_id", fmt.Sprint(ownSchool)), "a school transaction sees no send")

	for _, statement := range []string{
		`UPDATE parent_billing.document_mailings SET recipient_email = 'anders@example.org' WHERE organization_id = ?`,
		`DELETE FROM parent_billing.document_mailings WHERE organization_id = ?`,
	} {
		assert.ErrorContains(t, asRequestRole("app.current_org_id", fmt.Sprint(own), statement, own), "permission denied",
			"the request role neither changes nor deletes a send: %s", statement)
	}

	// The sends go with their letter, so with the invoice and the Träger.
	_, err = db.ExecContext(ctx, `DELETE FROM parent_billing.document_letters WHERE organization_id = ? AND document_id = ?`, own, document)
	require.NoError(t, err)
	var left int
	require.NoError(t, db.NewRaw(`SELECT COUNT(*) FROM parent_billing.document_mailings WHERE organization_id = ?`, own).Scan(ctx, &left))
	assert.Zero(t, left, "the sends go with their letter")
}
