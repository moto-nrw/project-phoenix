package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// TestCurrentTraegerNameNamesOnlyTheOwnTraeger pins 1.15.890 (#6437): the
// request role reads no row of platform.organizations, but in the Träger
// transaction it reads its own Träger's name, and only that one: another
// Träger's transaction reads its own name, a school transaction and a
// deleted Träger read none.
func TestCurrentTraegerNameNamesOnlyTheOwnTraeger(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := context.Background()
	newTraeger := func(name string) int64 {
		return testpkg.CreateTestOrganization(t, db, &testpkg.Organization{
			Name: name, Slug: fmt.Sprintf("traeger-name-%d", testpkg.UniqueTestTenantID(t)),
		}).ID
	}
	own, foreign, deleted := newTraeger("Träger Name Eigen"), newTraeger("Träger Name Fremd"), newTraeger("Träger Name Gelöscht")
	school := testpkg.CreateTestSchoolInOrganization(t, db, own, "Schule Träger Name")
	_, err := db.ExecContext(ctx, `UPDATE platform.organizations SET deleted_at = now() WHERE id = ?`, deleted)
	require.NoError(t, err)

	asRequestRole := func(setting string, value int64, fn func(bun.Tx) error) error {
		return db.RunInTx(ctx, &sql.TxOptions{}, func(ctx context.Context, tx bun.Tx) error {
			if _, err := tx.ExecContext(ctx, `SET LOCAL ROLE phoenix_tenant`); err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, `SELECT set_config(?, ?, true)`, setting, fmt.Sprint(value)); err != nil {
				return err
			}
			return fn(tx)
		})
	}
	nameIn := func(setting string, value int64) *string {
		t.Helper()
		var name *string
		require.NoError(t, asRequestRole(setting, value, func(tx bun.Tx) error {
			return tx.NewRaw(`SELECT platform.current_traeger_name()`).Scan(ctx, &name)
		}))
		return name
	}

	ownName := nameIn("app.current_org_id", own)
	require.NotNil(t, ownName)
	assert.Equal(t, "Träger Name Eigen", *ownName)
	foreignName := nameIn("app.current_org_id", foreign)
	require.NotNil(t, foreignName)
	assert.Equal(t, "Träger Name Fremd", *foreignName, "each Träger transaction reads its own name")
	assert.Nil(t, nameIn("app.current_tenant_id", school), "a school transaction reads no Träger name")
	assert.Nil(t, nameIn("app.current_org_id", deleted), "a deleted Träger has no name")

	assert.Error(t, asRequestRole("app.current_org_id", own, func(tx bun.Tx) error {
		var names []string
		return tx.NewRaw(`SELECT name FROM platform.organizations WHERE id = ?`, own).Scan(ctx, &names)
	}), "the request role still reads no row of the table")
}
