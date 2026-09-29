package migrations

import (
	"context"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// personalPINColumns lists the columns 1.15.431 drops, as schema.table.column.
var personalPINColumns = [][3]string{
	{"auth", "accounts", "pin_hash"},
	{"auth", "accounts", "pin_attempts"},
	{"auth", "accounts", "pin_locked_until"},
	{"platform", "schools", "device_pin_hash"},
}

func existingPersonalPINColumns(t *testing.T, db *testpkg.DB) []string {
	t.Helper()
	var found []string
	for _, column := range personalPINColumns {
		var exists bool
		require.NoError(t, db.NewRaw(`
			SELECT EXISTS (
				SELECT 1 FROM information_schema.columns
				WHERE table_schema = ? AND table_name = ? AND column_name = ?
			)
		`, column[0], column[1], column[2]).Scan(context.Background(), &exists))
		if exists {
			found = append(found, column[0]+"."+column[1]+"."+column[2])
		}
	}
	return found
}

// The migrated schema has no PIN columns; Down restores them empty and Up
// drops them again, so both directions can run on an existing database.
func TestDropPersonalPINColumnsMigration(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := context.Background()
	account := testpkg.CreateTestAccount(t, db, "pin-columns")

	assert.Empty(t, existingPersonalPINColumns(t, db), "the migrated schema has no PIN columns")
	require.NoError(t, dropPersonalPINColumnsUp(ctx, db), "Up runs again on a migrated database")

	require.NoError(t, dropPersonalPINColumnsDown(ctx, db))
	assert.Len(t, existingPersonalPINColumns(t, db), len(personalPINColumns))
	var attempts int
	require.NoError(t, db.NewRaw(`SELECT pin_attempts FROM auth.accounts WHERE id = ?`, account.ID).Scan(ctx, &attempts))
	assert.Zero(t, attempts, "restored accounts start without failed PIN attempts")

	_, err := db.NewRaw(`UPDATE auth.accounts SET pin_hash = 'hash', pin_attempts = 3 WHERE id = ?`, account.ID).Exec(ctx)
	require.NoError(t, err)
	require.NoError(t, dropPersonalPINColumnsUp(ctx, db), "Up drops columns that still hold PIN data")
	assert.Empty(t, existingPersonalPINColumns(t, db))

	var email string
	require.NoError(t, db.NewRaw(`SELECT email FROM auth.accounts WHERE id = ?`, account.ID).Scan(ctx, &email))
	assert.Equal(t, account.Email, email, "the account itself survives the drop")
}
