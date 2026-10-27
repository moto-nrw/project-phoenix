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

// TestHolidayRegistrationDeletionLogIsClosedToTheRequestRole pins ADR 0118:
// only the administrative transaction of the hourly retention writes the
// deletion log; under the request role even the Träger's own transaction
// neither reads nor writes it. An entry counts something and goes with its
// Träger.
func TestHolidayRegistrationDeletionLogIsClosedToTheRequestRole(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := context.Background()
	slug := fmt.Sprintf("traeger-loeschprotokoll-%d", testpkg.UniqueTestTenantID(t))
	organization := testpkg.CreateTestOrganization(t, db, &testpkg.Organization{Name: "Träger Löschprotokoll", Slug: slug}).ID

	_, err := db.ExecContext(ctx, `INSERT INTO enrollment.holiday_registration_deletions
			(organization_id, holiday_period_id, period_ended_on, registrations, children, audit_entries)
		VALUES (?, 1, '2027-01-15', 0, 0, 0)`, organization)
	assert.ErrorContains(t, err, "chk_holiday_registration_deletions_counts", "an entry logs a deletion")
	_, err = db.ExecContext(ctx, `INSERT INTO enrollment.holiday_registration_deletions
			(organization_id, holiday_period_id, period_ended_on, registrations, children, audit_entries)
		VALUES (?, 1, '2027-01-15', 2, 3, 4)`, organization)
	require.NoError(t, err)

	asTraeger := func(fn func(bun.Tx) error) error {
		return db.RunInTx(ctx, &sql.TxOptions{}, func(ctx context.Context, tx bun.Tx) error {
			if _, err := tx.ExecContext(ctx, `SET LOCAL ROLE phoenix_tenant`); err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, `SELECT set_config('app.current_org_id', ?, true)`, fmt.Sprint(organization)); err != nil {
				return err
			}
			return fn(tx)
		})
	}
	err = asTraeger(func(tx bun.Tx) error {
		var rows int
		return tx.NewRaw(`SELECT COUNT(*) FROM enrollment.holiday_registration_deletions`).Scan(ctx, &rows)
	})
	assert.ErrorContains(t, err, "permission denied", "the request role does not read the log")
	err = asTraeger(func(tx bun.Tx) error {
		_, err := tx.ExecContext(ctx, `INSERT INTO enrollment.holiday_registration_deletions
				(organization_id, holiday_period_id, period_ended_on, registrations, children, audit_entries)
			VALUES (?, 1, '2027-01-15', 1, 1, 0)`, organization)
		return err
	})
	assert.ErrorContains(t, err, "permission denied", "the request role does not write the log")

	_, err = db.ExecContext(ctx, `DELETE FROM platform.organizations WHERE id = ?`, organization)
	require.NoError(t, err)
	var left int
	require.NoError(t, db.NewRaw(`SELECT COUNT(*) FROM enrollment.holiday_registration_deletions WHERE organization_id = ?`, organization).
		Scan(ctx, &left))
	assert.Zero(t, left, "the log goes with its Träger")
}
