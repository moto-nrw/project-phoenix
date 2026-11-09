// No session survives the end of its school (#6305). Ending a school's
// sessions (auth.revoke_school_sessions, behind RevokeTokensByTenantID and
// the demo retirement) locks their accounts, then holds the school until
// its deletion commits; a sign-in or refresh locks the account, then reads
// the school under a share lock. Each test holds one side in an open
// transaction and lets the other run into it.
package behavior_test

import (
	"context"
	"database/sql"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// othersWaitForALock reports whether another session of the test's own
// database waits for a lock.
func othersWaitForALock(t *testing.T, db *bun.DB) bool {
	t.Helper()
	var waiting int
	err := db.NewRaw(`SELECT count(*) FROM pg_stat_activity
		WHERE datname = current_database() AND pid <> pg_backend_pid() AND wait_event_type = 'Lock'`).Scan(context.Background(), &waiting)
	return err == nil && waiting > 0
}

func sessionsAtSchool(t *testing.T, db *bun.DB, schoolID int64) int {
	t.Helper()
	var count int
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM auth.tokens WHERE tenant_id = ?`, schoolID).Scan(context.Background(), &count))
	return count
}

// runAgainst runs act while holder is open: once act waits for a lock, or
// has returned, the holder commits. It reports whether act waited for the
// holder, and act's error.
func runAgainst(t *testing.T, db *bun.DB, holder bun.Tx, act func() error) (bool, error) {
	t.Helper()
	done := make(chan error, 1)
	go func() { done <- act() }()
	var result error
	finished, waited := false, false
	for !finished && !waited {
		select {
		case result = <-done:
			finished = true
		default:
			waited = othersWaitForALock(t, db)
		}
	}
	require.NoError(t, holder.Commit())
	if !finished {
		result = <-done
	}
	return waited, result
}

// A refresh that rotates the session while the school's sessions end: the
// revocation waits for the account and then deletes the successor the
// refresh committed meanwhile, so no valid successor is left.
func TestRevokeTokensByTenantIDEndsARefreshCommittedWhileItWaits(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	service := setupAuthService(t, db)
	ctx := context.Background()
	schoolID, _ := testpkg.CreateTestTenant(t, db)
	account := testpkg.CreateTestAccount(t, db, "revoke-school-rotation")
	testpkg.MapAccountToTenant(t, db, account.ID, schoolID)
	testpkg.CreateTestSessionFamily(t, db, account.ID, schoolID, "")

	refresh, err := db.BeginTx(ctx, &sql.TxOptions{})
	require.NoError(t, err)
	defer func() { _ = refresh.Rollback() }()
	for _, statement := range []string{
		`SELECT id FROM auth.accounts WHERE id = ?0 FOR UPDATE`,
		`INSERT INTO auth.tokens (account_id, token, expiry, tenant_id, portal_scope, family_id)
			SELECT account_id, encode(sha256(convert_to(gen_random_uuid()::text, 'UTF8')), 'hex'), expiry, tenant_id, portal_scope, family_id
			FROM auth.tokens WHERE account_id = ?0 AND tenant_id = ?1 AND rotated_at IS NULL`,
		`UPDATE auth.tokens SET rotated_at = NOW(), replacement_token = encode(sha256(convert_to(gen_random_uuid()::text, 'UTF8')), 'hex')
			WHERE account_id = ?0 AND tenant_id = ?1 AND rotated_at IS NULL
				AND id < (SELECT max(id) FROM auth.tokens WHERE account_id = ?0 AND tenant_id = ?1)`,
	} {
		_, err := refresh.NewRaw(statement, account.ID, schoolID).Exec(ctx)
		require.NoError(t, err, statement)
	}

	waited, revokeErr := runAgainst(t, db, refresh, func() error {
		return testpkg.WithinAdminContext(t, ctx, db, func(adminCtx context.Context) error {
			_, err := service.RevokeTokensByTenantID(adminCtx, schoolID)
			return err
		})
	})

	require.NoError(t, revokeErr)
	require.True(t, waited, "the revocation waits for the refresh")
	assert.Zero(t, sessionsAtSchool(t, db, schoolID), "the successor the refresh committed is gone too")
}

// endSchool is the operator's soft delete as it runs: the sessions end, then
// the school is deleted, in one transaction that stays open.
func endSchool(t *testing.T, db *bun.DB, schoolID int64) bun.Tx {
	t.Helper()
	ctx := context.Background()
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	require.NoError(t, err)
	t.Cleanup(func() { _ = tx.Rollback() })
	_, err = tx.NewRaw(`SELECT count(*) FROM auth.revoke_school_sessions(?)`, schoolID).Exec(ctx)
	require.NoError(t, err)
	_, err = tx.NewRaw(`UPDATE platform.schools SET deleted_at = NOW() WHERE id = ?`, schoolID).Exec(ctx)
	require.NoError(t, err)
	return tx
}

// A staff sign-in while the school ends: it waits for the school and finds
// it gone, so no session of the deleted school is minted.
func TestLoginWhileItsSchoolEndsMintsNoSession(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	service := setupAuthService(t, db)
	ctx := context.Background()
	schoolID, _ := testpkg.CreateTestTenant(t, db)
	email, username := uniqueTestCredentials("login-school-ends")
	account, err := service.Register(testpkg.TenantContext(schoolID), email, username, testPassword, nil, 0)
	require.NoError(t, err)
	testpkg.MapAccountToTenant(t, db, account.ID, schoolID)

	_, loginErr := runAgainst(t, db, endSchool(t, db, schoolID), func() error {
		_, _, err := service.Login(ctx, email, testPassword)
		return err
	})

	assert.Error(t, loginErr, "the sign-in finds the school gone")
	assert.Zero(t, sessionsAtSchool(t, db, schoolID), "no session of the deleted school is left")
}

// A parent sign-in while the school ends: the session would be pinned to
// the school; it waits for it and finds it gone.
func TestParentLoginWhileItsSchoolEndsMintsNoSession(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	service := setupAuthService(t, db)
	ctx := context.Background()
	schoolID, _ := testpkg.CreateTestTenant(t, db)
	email, _ := newGuardianAccount(t, db, service, "parent-school-ends", schoolID)

	_, loginErr := runAgainst(t, db, endSchool(t, db, schoolID), func() error {
		_, _, err := service.LoginParent(ctx, email, testPassword)
		return err
	})

	assert.Error(t, loginErr, "the sign-in finds the school gone")
	assert.Zero(t, sessionsAtSchool(t, db, schoolID), "no session of the deleted school is left")
}
