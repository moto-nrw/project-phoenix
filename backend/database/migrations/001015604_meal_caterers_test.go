package migrations

import (
	"context"
	"fmt"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMealCaterersKeepCaterersAwayFromSchools pins ADR 0077: sessions may
// carry the caterer scope, auth.caterer_schools is the one record of a
// school's supply and holds one active supply per school, and a school may
// neither write nor rewrite a caterer or its account link directly.
func TestMealCaterersKeepCaterersAwayFromSchools(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupIsolatedTestDB(t)
	require.NoError(t, retireDirectPermissionsDown(t.Context(), db))
	ctx := context.Background()
	require.NoError(t, mealCaterersDown(ctx, db))
	require.NoError(t, mealCaterersUp(ctx, db))

	var allowsCaterer bool
	require.NoError(t, db.NewRaw(`SELECT pg_get_constraintdef(oid) LIKE '%caterer%' FROM pg_constraint WHERE conname = 'chk_tokens_portal_scope'`).Scan(ctx, &allowsCaterer))
	assert.True(t, allowsCaterer)

	var duplicate bool
	require.NoError(t, db.NewRaw(`SELECT to_regclass('schedule.meal_caterer_supplies') IS NOT NULL`).Scan(ctx, &duplicate))
	assert.False(t, duplicate, "the supply has one record, auth.caterer_schools")

	tenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, tenantID)
	var catererID int64
	require.NoError(t, db.NewRaw(`INSERT INTO auth.caterers (name) VALUES ('Küche am Markt') RETURNING id`).Scan(ctx, &catererID))
	_, err := db.NewRaw(`INSERT INTO auth.caterer_schools (caterer_id, tenant_id, delivery_address) VALUES (?, ?, 'Hof')`, catererID, tenantID).Exec(ctx)
	require.NoError(t, err)
	_, err = db.NewRaw(`INSERT INTO auth.caterer_schools (caterer_id, tenant_id, started_at) VALUES (?, ?, NOW() + INTERVAL '1 second')`, catererID, tenantID).Exec(ctx)
	require.Error(t, err, "a second active supply of the same school")
	_, err = db.NewRaw(`UPDATE auth.caterer_schools SET ended_at = NOW() + INTERVAL '1 second' WHERE tenant_id = ?`, tenantID).Exec(ctx)
	require.NoError(t, err)
	_, err = db.NewRaw(`INSERT INTO auth.caterer_schools (caterer_id, tenant_id, started_at) VALUES (?, ?, NOW() + INTERVAL '2 seconds')`, catererID, tenantID).Exec(ctx)
	require.NoError(t, err, "after the end a new supply may start")

	for _, table := range []string{"auth.caterers", "auth.caterer_accounts"} {
		var writable bool
		require.NoError(t, db.NewRaw(`SELECT has_table_privilege('phoenix_tenant', ?, 'INSERT') OR has_table_privilege('phoenix_tenant', ?, 'UPDATE') OR has_table_privilege('phoenix_tenant', ?, 'DELETE')`, table, table, table).Scan(ctx, &writable))
		assert.False(t, writable, table)
	}
	var deletable bool
	require.NoError(t, db.NewRaw(`SELECT has_table_privilege('phoenix_tenant', 'auth.caterer_schools', 'DELETE')`).Scan(ctx, &deletable))
	assert.False(t, deletable, "an ended supply stays as the record of it")

	require.NoError(t, mealCaterersDown(ctx, db))
	require.NoError(t, mealCaterersUp(ctx, db))
}

// TestCatererCreationNeedsAnOpenInvitationOfTheSchool pins the only way a
// school's role creates a caterer: auth.create_invited_caterer, for an open
// invitation of the transaction's school and the account of its address.
func TestCatererCreationNeedsAnOpenInvitationOfTheSchool(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := t.Context()
	school := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, school)
	other := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, other)
	invited := testpkg.CreateTestAccount(t, db, "caterer-function-invited")
	stranger := testpkg.CreateTestAccount(t, db, "caterer-function-stranger")
	invite := func(tenantID int64, revoked bool) int64 {
		var id int64
		require.NoError(t, db.NewRaw(`INSERT INTO auth.caterer_invitations (tenant_id, token_hash, email, caterer_name, expires_at, revoked_at)
			VALUES (?, md5(random()::text), UPPER(?), 'Küche am Markt', NOW() + INTERVAL '1 day', CASE WHEN ? THEN NOW() END) RETURNING id`,
			tenantID, invited.Email, revoked).Scan(ctx, &id))
		return id
	}
	asSchool := func(tenantID int64, statement string, args ...any) (int64, error) {
		tx, err := db.BeginTx(ctx, nil)
		require.NoError(t, err)
		defer func() { _ = tx.Rollback() }()
		_, err = tx.ExecContext(ctx, `SET LOCAL ROLE phoenix_tenant`)
		require.NoError(t, err)
		_, err = tx.ExecContext(ctx, `SELECT set_config('app.current_tenant_id', ?, true)`, fmt.Sprint(tenantID))
		require.NoError(t, err)
		var id int64
		if err := tx.NewRaw(statement, args...).Scan(ctx, &id); err != nil {
			return 0, err
		}
		return id, tx.Commit()
	}

	_, err := asSchool(school, `INSERT INTO auth.caterers (name) VALUES ('Küche') RETURNING id`)
	require.Error(t, err, "the school's role inserts no caterer directly")
	_, err = asSchool(school, `SELECT auth.create_invited_caterer(?, ?)`, invite(other, false), invited.ID)
	require.Error(t, err, "another school's invitation creates nothing")
	_, err = asSchool(school, `SELECT auth.create_invited_caterer(?, ?)`, invite(school, true), invited.ID)
	require.Error(t, err, "a revoked invitation creates nothing")
	open := invite(school, false)
	_, err = asSchool(school, `SELECT auth.create_invited_caterer(?, ?)`, open, stranger.ID)
	require.Error(t, err, "only the account of the invited address becomes the caterer")

	catererID, err := asSchool(school, `SELECT auth.create_invited_caterer(?, ?)`, open, invited.ID)
	require.NoError(t, err)
	var linked int64
	require.NoError(t, db.NewRaw(`SELECT caterer_id FROM auth.caterer_accounts WHERE account_id = ?`, invited.ID).Scan(ctx, &linked))
	assert.Equal(t, catererID, linked)
	_, err = asSchool(school, `SELECT auth.create_invited_caterer(?, ?)`, open, invited.ID)
	require.Error(t, err, "an account belongs to one caterer")
}

// The hourly account erasure of 1.15.464 keeps a caterer's login while the
// caterer supplies a school, and takes it once the last supply ended.
func TestAccountErasureKeepsAWorkingCaterer(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	require.NoError(t, retireDirectPermissionsDown(t.Context(), db))
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
