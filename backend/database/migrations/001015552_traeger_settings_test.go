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

// asRole runs fn in a rolled-back transaction under the tenant role with the
// given session settings, the way the Träger and school transactions run.
func asRole(t *testing.T, db *bun.DB, setting string, id int64, fn func(tx bun.Tx)) {
	t.Helper()
	ctx := context.Background()
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	require.NoError(t, err)
	defer func() { _ = tx.Rollback() }()
	_, err = tx.ExecContext(ctx, "SET LOCAL ROLE phoenix_tenant")
	require.NoError(t, err)
	_, err = tx.ExecContext(ctx, "SELECT set_config(?, ?, true)", setting, fmt.Sprint(id))
	require.NoError(t, err)
	fn(tx)
}

func traegerValueKeys(t *testing.T, tx bun.Tx) []string {
	t.Helper()
	var keys []string
	require.NoError(t, tx.NewRaw(`SELECT setting_key FROM config.traeger_setting_values ORDER BY setting_key`).Scan(context.Background(), &keys))
	return keys
}

// TestTraegerSettingValuesFollowTheTraegerAndItsSchools pins the policies of
// ADR 0086 section 5: the Träger transaction reads and writes only its own
// rows; a school reads the rows of its own Träger and writes none; another
// Träger and its schools see nothing.
func TestTraegerSettingValuesFollowTheTraegerAndItsSchools(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := context.Background()
	suffix := testpkg.UniqueTestTenantID(t)
	own := testpkg.CreateTestOrganization(t, db, &testpkg.Organization{Name: "Träger Eigen", Slug: fmt.Sprintf("traeger-eigen-%d", suffix), Active: true}).ID
	other := testpkg.CreateTestOrganization(t, db, &testpkg.Organization{Name: "Träger Fremd", Slug: fmt.Sprintf("traeger-fremd-%d", suffix), Active: true}).ID
	ownSchool := testpkg.CreateTestSchoolInOrganization(t, db, own, "Eigene Schule")
	otherSchool := testpkg.CreateTestSchoolInOrganization(t, db, other, "Fremde Schule")
	_, err := db.ExecContext(ctx, `INSERT INTO config.traeger_setting_values (organization_id, setting_key, value) VALUES (?, 'own.key', 'true'), (?, 'other.key', 'false')`, own, other)
	require.NoError(t, err)

	asRole(t, db, "app.current_org_id", own, func(tx bun.Tx) {
		assert.Equal(t, []string{"own.key"}, traegerValueKeys(t, tx), "the Träger reads only its own values")
		_, err := tx.ExecContext(ctx, `INSERT INTO config.traeger_setting_values (organization_id, setting_key, value) VALUES (?, 'own.second', '1')`, own)
		assert.NoError(t, err, "the Träger writes its own value")
		_, err = tx.ExecContext(ctx, `SAVEPOINT foreign_write`)
		require.NoError(t, err)
		_, err = tx.ExecContext(ctx, `INSERT INTO config.traeger_setting_values (organization_id, setting_key, value) VALUES (?, 'foreign.key', '1')`, other)
		assert.ErrorContains(t, err, "row-level security", "the Träger writes no value of another Träger")
		_, err = tx.ExecContext(ctx, `ROLLBACK TO SAVEPOINT foreign_write`)
		require.NoError(t, err)
		result, err := tx.ExecContext(ctx, `UPDATE config.traeger_setting_values SET value = 'true' WHERE organization_id = ?`, other)
		require.NoError(t, err)
		changed, _ := result.RowsAffected()
		assert.Zero(t, changed, "the Träger changes no value of another Träger")
	})

	asRole(t, db, "app.current_tenant_id", ownSchool, func(tx bun.Tx) {
		assert.Equal(t, []string{"own.key"}, traegerValueKeys(t, tx), "a school reads the values of its own Träger")
		result, err := tx.ExecContext(ctx, `UPDATE config.traeger_setting_values SET value = 'false' WHERE organization_id = ?`, own)
		require.NoError(t, err)
		changed, _ := result.RowsAffected()
		assert.Zero(t, changed, "a school changes no value of its Träger")
		result, err = tx.ExecContext(ctx, `DELETE FROM config.traeger_setting_values WHERE organization_id = ?`, own)
		require.NoError(t, err)
		deleted, _ := result.RowsAffected()
		assert.Zero(t, deleted, "a school deletes no value of its Träger")
		_, err = tx.ExecContext(ctx, `INSERT INTO config.traeger_setting_values (organization_id, setting_key, value) VALUES (?, 'school.key', '1')`, own)
		assert.ErrorContains(t, err, "row-level security", "a school writes no Träger value")
	})

	asRole(t, db, "app.current_tenant_id", otherSchool, func(tx bun.Tx) {
		assert.Equal(t, []string{"other.key"}, traegerValueKeys(t, tx), "a school of another Träger sees only its own Träger")
	})
}

// TestTraegerSettingAuditIsAppendOnlyForTheTraeger pins section 7: the
// Träger appends to its own change log and changes no entry; schools do not
// read it.
func TestTraegerSettingAuditIsAppendOnlyForTheTraeger(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := context.Background()
	suffix := testpkg.UniqueTestTenantID(t)
	own := testpkg.CreateTestOrganization(t, db, &testpkg.Organization{Name: "Träger Protokoll", Slug: fmt.Sprintf("traeger-protokoll-%d", suffix), Active: true}).ID
	school := testpkg.CreateTestSchoolInOrganization(t, db, own, "Protokollschule")
	_, err := db.ExecContext(ctx, `INSERT INTO config.traeger_setting_audit (organization_id, setting_key, new_value, action) VALUES (?, 'own.key', 'true', 'set')`, own)
	require.NoError(t, err)

	asRole(t, db, "app.current_org_id", own, func(tx bun.Tx) {
		_, err := tx.ExecContext(ctx, `INSERT INTO config.traeger_setting_audit (organization_id, setting_key, new_value, action) VALUES (?, 'own.key', 'false', 'set')`, own)
		assert.NoError(t, err, "the Träger appends to its change log")
		_, err = tx.ExecContext(ctx, `UPDATE config.traeger_setting_audit SET action = 'reset' WHERE organization_id = ?`, own)
		assert.ErrorContains(t, err, "permission denied", "the change log is append-only")
	})
	asRole(t, db, "app.current_tenant_id", school, func(tx bun.Tx) {
		var entries int
		require.NoError(t, tx.NewRaw(`SELECT COUNT(*) FROM config.traeger_setting_audit`).Scan(ctx, &entries))
		assert.Zero(t, entries, "a school does not read the Träger's change log")
	})
}
