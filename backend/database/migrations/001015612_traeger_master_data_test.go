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

// TestTraegerMasterDataFollowsTheTraegerPolicy pins the Träger policy of ADR
// 0105 on the master data tables (#4653): under the request role a Träger
// transaction sees and writes only its own Träger's rows, a school
// transaction sees none, the log takes no update, and one Träger has at most
// one default bank account.
func TestTraegerMasterDataFollowsTheTraegerPolicy(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := context.Background()
	newTraeger := func(name string) (int64, int64) {
		slug := fmt.Sprintf("traeger-stammdaten-%d", testpkg.UniqueTestTenantID(t))
		organization := testpkg.CreateTestOrganization(t, db, &testpkg.Organization{Name: name, Slug: slug}).ID
		testpkg.OwnTraegerMasterData(t, db, organization)
		return organization, testpkg.CreateTestSchoolInOrganization(t, db, organization, name+" OGS")
	}
	own, ownSchool := newTraeger("Träger Stammdaten Eigen")
	foreign, _ := newTraeger("Träger Stammdaten Fremd")

	_, err := db.ExecContext(ctx, `INSERT INTO platform.traeger_profiles (organization_id, contact_name) VALUES (?, 'Frau Beispiel')`, own)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `INSERT INTO platform.traeger_bank_accounts (organization_id, label, account_holder, iban, is_default)
		VALUES (?, 'Elternbeiträge', 'Träger e.V.', 'DE89370400440532013000', true)`, own)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `INSERT INTO platform.traeger_bank_accounts (organization_id, label, account_holder, iban, is_default)
		VALUES (?, 'Ferien', 'Träger e.V.', 'DE89370400440532013000', true)`, own)
	assert.ErrorContains(t, err, "uq_traeger_bank_accounts_default", "a Träger has one default bank account")
	_, err = db.ExecContext(ctx, `INSERT INTO platform.traeger_master_data_audit (organization_id, subject, action) VALUES (?, 'master_data', 'read')`, own)
	require.NoError(t, err)

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
				(SELECT COUNT(*) FROM platform.traeger_profiles WHERE organization_id = ?)
				+ (SELECT COUNT(*) FROM platform.traeger_bank_accounts WHERE organization_id = ?)
				+ (SELECT COUNT(*) FROM platform.traeger_master_data_audit WHERE organization_id = ?)`, own, own, own).Scan(ctx, &visible)
		}))
		return visible
	}
	assert.Equal(t, 3, count("app.current_org_id", fmt.Sprint(own)), "the Träger sees its profile, bank account and log")
	assert.Zero(t, count("app.current_org_id", fmt.Sprint(foreign)), "another Träger sees nothing")
	assert.Zero(t, count("app.current_tenant_id", fmt.Sprint(ownSchool)), "a school transaction sees nothing of its Träger's master data")

	err = asRequestRole("app.current_org_id", fmt.Sprint(foreign), func(tx bun.Tx) error {
		_, err := tx.ExecContext(ctx, `INSERT INTO platform.traeger_bank_accounts (organization_id, label, account_holder, iban)
			VALUES (?, 'Fremd', 'Fremd e.V.', 'DE89370400440532013000')`, own)
		return err
	})
	assert.ErrorContains(t, err, "row-level security", "a Träger cannot write a bank account for another Träger")
	err = asRequestRole("app.current_org_id", fmt.Sprint(own), func(tx bun.Tx) error {
		_, err := tx.ExecContext(ctx, `UPDATE platform.traeger_master_data_audit SET action = 'set' WHERE organization_id = ?`, own)
		return err
	})
	assert.ErrorContains(t, err, "permission denied", "the log takes no update")
}
