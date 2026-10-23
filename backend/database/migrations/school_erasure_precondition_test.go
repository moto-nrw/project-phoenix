package migrations

import (
	"bytes"
	"os"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

func TestSchoolErasurePreconditionRefusesOrphansBeforeMigration(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	require.NoError(t, schoolErasureDown(t.Context(), db))
	account := testpkg.CreateTestAccount(t, db, "nullable-mfa-fixture")
	erasureExec(t, db, `INSERT INTO auth.mfa_email_challenges(account_id,tenant_id,scope,code_hash,expires_at)
 VALUES (?,NULL,'','test-placeholder',NOW()+interval '1 hour')`, account.ID)
	erasureExec(t, db, `INSERT INTO audit.room_color_migration_backup(room_id,tenant_id,name,color)
 VALUES (1,-384637,'Fixture room','#000000')`)
	check := MigrationRegistry[schoolErasureVersion].Precondition
	require.NotNil(t, check, "deployment preflight must detect the refusal before stopping the app")
	output := &bytes.Buffer{}
	err := reportPreflightChecks(t.Context(), output, []preconditionCheck{{version: schoolErasureVersion, migration: MigrationRegistry[schoolErasureVersion]}}, db)
	require.Error(t, err)
	require.Contains(t, err.Error(), "audit.room_color_migration_backup.tenant_id: 1 orphaned rows")
	require.NotContains(t, err.Error(), "-384637", "report counts, never row identifiers")
	require.ErrorContains(t, schoolErasureUp(t.Context(), db), "1 orphaned rows")
	var installed bool
	require.NoError(t, db.NewRaw(`SELECT to_regclass('platform.school_erasures') IS NOT NULL`).Scan(t.Context(), &installed))
	require.False(t, installed, "refusal must not install any erasure objects")
	inventory, err := os.ReadFile("../../../docs/operations/sql/mandantenloeschung-vorpruefung-3837.sql")
	require.NoError(t, err)
	var rows []struct {
		Source                   string
		OrphanedRows             int64
		AllSchoolReferencesValid bool
	}
	require.NoError(t, db.NewRaw(string(inventory)).Scan(t.Context(), &rows))
	require.Len(t, rows, 5)
	for _, row := range rows {
		require.False(t, row.AllSchoolReferencesValid)
		if row.Source == "audit.room_color_migration_backup.tenant_id" {
			require.EqualValues(t, 1, row.OrphanedRows)
		} else {
			require.Zero(t, row.OrphanedRows)
		}
	}
	erasureExec(t, db, `DELETE FROM audit.room_color_migration_backup WHERE tenant_id=-384637`)
	require.NoError(t, check(t.Context(), db))
	require.NoError(t, schoolErasureUp(t.Context(), db))
}
