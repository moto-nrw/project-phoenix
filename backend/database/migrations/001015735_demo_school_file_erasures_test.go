package migrations

import (
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The file erasure marker of retired demo schools (#5544) belongs to the demo
// process alone: it may read and add a marker, never change or remove one,
// and no serving role sees the table.
func TestDemoSchoolFileErasuresBelongToTheDemoRoleOnly(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := t.Context()
	const table = "platform.demo_school_file_erasures"
	grants := map[string]map[string]bool{
		"phoenix_demo":   {"SELECT": true, "INSERT": true},
		"phoenix_auth":   {},
		"phoenix_tenant": {},
		"phoenix_admin":  {},
	}
	for role, allowed := range grants {
		for _, privilege := range []string{"SELECT", "INSERT", "UPDATE", "DELETE", "TRUNCATE", "REFERENCES", "TRIGGER"} {
			var granted bool
			require.NoError(t, db.NewRaw(`SELECT has_table_privilege(?, ?, ?)`, role, table, privilege).Scan(ctx, &granted))
			assert.Equal(t, allowed[privilege], granted, "%s %s on %s", role, privilege, table)
		}
	}

	// No incoming foreign key on platform.schools: the school erasure would
	// refuse to run on a changed foreign key fingerprint.
	var foreignKeys int
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM pg_constraint WHERE conrelid = ?::regclass AND contype = 'f'`, table).Scan(ctx, &foreignKeys))
	assert.Zero(t, foreignKeys)

	// The demo role adds a marker with counts; no other shape is accepted.
	// The rollback keeps the shared database as it was.
	schoolID, _ := testpkg.CreateTestTenant(t, db)
	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = tx.Rollback() })
	_, err = tx.ExecContext(ctx, `SET LOCAL ROLE phoenix_demo`)
	require.NoError(t, err)
	_, err = tx.ExecContext(ctx, `SAVEPOINT shape`)
	require.NoError(t, err)
	_, err = tx.ExecContext(ctx, `INSERT INTO platform.demo_school_file_erasures (school_id, file_counts) VALUES (?, '[]'::jsonb)`, schoolID)
	require.Error(t, err, "counts are an object per namespace")
	_, err = tx.ExecContext(ctx, `ROLLBACK TO SAVEPOINT shape`)
	require.NoError(t, err)
	_, err = tx.ExecContext(ctx, `INSERT INTO platform.demo_school_file_erasures (school_id, file_counts) VALUES (?, '{"files": 2}'::jsonb)`, schoolID)
	require.NoError(t, err)
	var counts string
	require.NoError(t, tx.NewRaw(`SELECT file_counts::text FROM platform.demo_school_file_erasures WHERE school_id = ?`, schoolID).Scan(ctx, &counts))
	assert.JSONEq(t, `{"files": 2}`, counts)
}
