package migrations

import (
	"context"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMealCaterersKeepCaterersAwayFromSchools pins ADR 0074: sessions may
// carry the caterer scope, a school has one active supply and one active
// caterer link at most, and a school may create but never rewrite a caterer.
func TestMealCaterersKeepCaterersAwayFromSchools(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupIsolatedTestDB(t)
	ctx := context.Background()
	require.NoError(t, mealCaterersDown(ctx, db))
	require.NoError(t, mealCaterersUp(ctx, db))

	var allowsCaterer bool
	require.NoError(t, db.NewRaw(`SELECT pg_get_constraintdef(oid) LIKE '%caterer%' FROM pg_constraint WHERE conname = 'chk_tokens_portal_scope'`).Scan(ctx, &allowsCaterer))
	assert.True(t, allowsCaterer)

	tenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, tenantID)
	var catererID int64
	require.NoError(t, db.NewRaw(`INSERT INTO auth.caterers (name) VALUES ('Küche am Markt') RETURNING id`).Scan(ctx, &catererID))
	_, err := db.NewRaw(`INSERT INTO schedule.meal_caterer_supplies (tenant_id, caterer_id) VALUES (?, ?)`, tenantID, catererID).Exec(ctx)
	require.NoError(t, err)
	_, err = db.NewRaw(`INSERT INTO schedule.meal_caterer_supplies (tenant_id, caterer_id) VALUES (?, ?)`, tenantID, catererID).Exec(ctx)
	require.Error(t, err, "a second active supply of the same school")
	_, err = db.NewRaw(`UPDATE schedule.meal_caterer_supplies SET ended_at = NOW() WHERE tenant_id = ?`, tenantID).Exec(ctx)
	require.NoError(t, err)
	_, err = db.NewRaw(`INSERT INTO schedule.meal_caterer_supplies (tenant_id, caterer_id) VALUES (?, ?)`, tenantID, catererID).Exec(ctx)
	require.NoError(t, err, "after the end a new supply may start")

	for _, table := range []string{"auth.caterers", "auth.caterer_accounts"} {
		var rewritable bool
		require.NoError(t, db.NewRaw(`SELECT has_table_privilege('phoenix_tenant', ?, 'UPDATE') OR has_table_privilege('phoenix_tenant', ?, 'DELETE')`, table, table).Scan(ctx, &rewritable))
		assert.False(t, rewritable, table)
	}
	_, err = db.NewRaw(`INSERT INTO auth.caterer_schools (caterer_id, tenant_id) VALUES (?, ?)`, catererID, tenantID).Exec(ctx)
	require.NoError(t, err)
	_, err = db.NewRaw(`INSERT INTO auth.caterer_schools (caterer_id, tenant_id, started_at) VALUES (?, ?, NOW() + INTERVAL '1 second')`, catererID, tenantID).Exec(ctx)
	require.Error(t, err, "a second active caterer of the same school")
	var deletable bool
	require.NoError(t, db.NewRaw(`SELECT has_table_privilege('phoenix_tenant', 'schedule.meal_caterer_supplies', 'DELETE')`).Scan(ctx, &deletable))
	assert.False(t, deletable, "an ended supply stays as the record of it")

	require.NoError(t, mealCaterersDown(ctx, db))
	require.NoError(t, mealCaterersUp(ctx, db))
}

// The hourly account erasure of 1.15.464 keeps a caterer's login while the
// caterer supplies a school, and takes it once the last supply ended.
func TestAccountErasureKeepsAWorkingCaterer(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := context.Background()
	tenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, tenantID)
	account := testpkg.CreateTestAccount(t, db, "caterer-erasure")
	testpkg.UnclaimTestAccount(t, db, account.ID)
	invited := testpkg.CreateTestAccount(t, db, "caterer-invited")
	testpkg.UnclaimTestAccount(t, db, invited.ID)
	_, err := db.NewRaw(`INSERT INTO auth.caterer_invitations (tenant_id, token_hash, email, caterer_name, expires_at)
		VALUES (?, ?, UPPER(?), 'Neue Küche', NOW() + INTERVAL '1 day')`, tenantID, invited.Email, invited.Email).Exec(ctx)
	require.NoError(t, err)
	var catererID int64
	require.NoError(t, db.NewRaw(`INSERT INTO auth.caterers (name) VALUES ('Küche am Markt') RETURNING id`).Scan(ctx, &catererID))
	_, err = db.NewRaw(`INSERT INTO auth.caterer_accounts (account_id, caterer_id) VALUES (?, ?)`, account.ID, catererID).Exec(ctx)
	require.NoError(t, err)
	_, err = db.NewRaw(`INSERT INTO auth.caterer_schools (caterer_id, tenant_id) VALUES (?, ?)`, catererID, tenantID).Exec(ctx)
	require.NoError(t, err)

	erased := eraseUnusedAccountsAsAdmin(t, db)
	assert.NotContains(t, erased, account.ID, "a caterer that supplies a school keeps its login")
	assert.NotContains(t, erased, invited.ID, "an open caterer invitation keeps the account of its address")

	_, err = db.NewRaw(`UPDATE auth.caterer_schools SET ended_at = NOW() WHERE caterer_id = ?`, catererID).Exec(ctx)
	require.NoError(t, err)
	assert.Contains(t, eraseUnusedAccountsAsAdmin(t, db), account.ID, "without a school the login has no purpose")
}
