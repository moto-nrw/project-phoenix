package compose_test

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/compose"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func TestAccountRetentionCommitsAvatarCleanupWithErasure(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	testpkg.EnsureTestTenant(t, db, testpkg.Tenant(t))
	account := testpkg.CreateTestAccount(t, db, "avatar-retention")
	testpkg.UnclaimTestAccount(t, db, account.ID)
	external := testpkg.CreateTestAccount(t, db, "external-avatar")
	testpkg.UnclaimTestAccount(t, db, external.ID)
	_, err := db.ExecContext(t.Context(), `UPDATE auth.accounts SET avatar = 'https://example.invalid/avatar.png' WHERE id = ?`, external.ID)
	require.NoError(t, err)
	avatar := fmt.Sprintf("/uploads/avatars/global/retention-%d.png", account.ID)
	_, err = db.ExecContext(t.Context(), `UPDATE auth.accounts SET avatar = ? WHERE id = ?`, avatar, account.ID)
	require.NoError(t, err)
	owner, err := compose.New(compose.Dependencies{DB: db, Observe: func(compose.Observation) {}})
	require.NoError(t, err)
	base := testpkg.WithTenantRuntime(t, t.Context(), db)
	require.NoError(t, testpkg.WithinAdminTransaction(base, func(ctx context.Context) error {
		_, err := owner.EraseUnusedAccounts(ctx)
		return err
	}))
	var pending int
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM auth.account_avatar_file_cleanup WHERE avatar = ?`, avatar).Scan(t.Context(), &pending))
	require.Equal(t, 1, pending, "a process stopping after commit must leave a durable deletion intent")
	// A fresh composition has no in-memory callback from the erasure.
	restarted, err := compose.New(compose.Dependencies{DB: db, Observe: func(compose.Observation) {}, RemoveErasedAvatarFile: func(path string) error {
		require.Equal(t, avatar, path)
		return fs.ErrNotExist
	}})
	require.NoError(t, err)
	removed, err := restarted.CleanupErasedAccountAvatars(base)
	require.NoError(t, err)
	require.Equal(t, 1, removed)
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM auth.account_avatar_file_cleanup`).Scan(t.Context(), &pending))
	require.Zero(t, pending)
}

func TestAccountRetentionRetriesCommittedAvatarFailures(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	testpkg.EnsureTestTenant(t, db, testpkg.Tenant(t))
	account := testpkg.CreateTestAccount(t, db, "avatar-retry")
	testpkg.UnclaimTestAccount(t, db, account.ID)
	current := fmt.Sprintf("/uploads/avatars/global/retry-%d.png", account.ID)
	legacy := fmt.Sprintf("/uploads/avatars/%d/profile-%d.png", testpkg.Tenant(t), account.ID)
	_, err := db.ExecContext(t.Context(), `UPDATE auth.accounts SET avatar = ? WHERE id = ?`, current, account.ID)
	require.NoError(t, err)
	_, err = db.ExecContext(t.Context(), `INSERT INTO users.profiles (tenant_id, account_id, avatar) VALUES (?, ?, ?)`, testpkg.Tenant(t), account.ID, legacy)
	require.NoError(t, err)
	owner, err := compose.New(compose.Dependencies{DB: db, Observe: func(compose.Observation) {}, RemoveErasedAvatarFile: func(path string) error {
		if path == current {
			return fs.ErrPermission
		}
		require.Equal(t, legacy, path)
		return nil
	}})
	require.NoError(t, err)
	base := testpkg.WithTenantRuntime(t, t.Context(), db)
	require.NoError(t, testpkg.WithinAdminTransaction(base, func(ctx context.Context) error {
		_, err := owner.EraseUnusedAccounts(ctx)
		return err
	}))
	removed, err := owner.CleanupErasedAccountAvatars(base)
	require.Error(t, err, "a partial filesystem failure must not report success")
	require.Equal(t, 1, removed)
	var pending []string
	require.NoError(t, db.NewRaw(`SELECT avatar FROM auth.account_avatar_file_cleanup`).Scan(t.Context(), &pending))
	require.Equal(t, []string{current}, pending, "successful file deletion must commit despite a different file failing")
	restarted, err := compose.New(compose.Dependencies{DB: db, Observe: func(compose.Observation) {}, RemoveErasedAvatarFile: func(path string) error {
		require.Equal(t, current, path)
		return nil
	}})
	require.NoError(t, err)
	removed, err = restarted.CleanupErasedAccountAvatars(base)
	require.NoError(t, err)
	require.Equal(t, 1, removed)
	removed, err = restarted.CleanupErasedAccountAvatars(base)
	require.NoError(t, err)
	require.Zero(t, removed)
}

func TestAccountRetentionRollbackAndOtherSchoolProtection(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	school := testpkg.Tenant(t)
	testpkg.EnsureTestTenant(t, db, school)
	orphan := testpkg.CreateTestAccount(t, db, "avatar-rollback")
	testpkg.UnclaimTestAccount(t, db, orphan.ID)
	otherSchool := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, otherSchool)
	shared := testpkg.CreateTestAccount(t, db, "avatar-other-school")
	testpkg.UnclaimTestAccount(t, db, shared.ID)
	testpkg.MapAccountToTenant(t, db, shared.ID, otherSchool)
	for _, id := range []int64{orphan.ID, shared.ID} {
		_, err := db.ExecContext(t.Context(), `UPDATE auth.accounts SET avatar = ? WHERE id = ?`, fmt.Sprintf("/uploads/avatars/global/keep-%d.png", id), id)
		require.NoError(t, err)
	}
	owner, err := compose.New(compose.Dependencies{DB: db, Observe: func(compose.Observation) {}, RemoveErasedAvatarFile: func(string) error {
		t.Fatal("anonymization must not delete a file before commit")
		return nil
	}})
	require.NoError(t, err)
	base := testpkg.WithTenantRuntime(t, t.Context(), db)
	rollback := errors.New("abort outer transaction")
	require.ErrorIs(t, testpkg.WithinAdminTransaction(base, func(ctx context.Context) error {
		_, err := owner.EraseUnusedAccounts(ctx)
		require.NoError(t, err)
		_, err = owner.CleanupErasedAccountAvatars(ctx)
		require.ErrorContains(t, err, "ambient transaction")
		return rollback
	}), rollback)
	var pending int
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM auth.account_avatar_file_cleanup`).Scan(t.Context(), &pending))
	require.Zero(t, pending)
	require.NoError(t, testpkg.WithinAdminTransaction(base, func(ctx context.Context) error {
		_, err := owner.EraseUnusedAccounts(ctx)
		return err
	}))
	var avatar string
	require.NoError(t, db.NewRaw(`SELECT avatar FROM auth.accounts WHERE id = ?`, shared.ID).Scan(t.Context(), &avatar))
	require.Equal(t, fmt.Sprintf("/uploads/avatars/global/keep-%d.png", shared.ID), avatar)
	var queued []string
	require.NoError(t, db.NewRaw(`SELECT avatar FROM auth.account_avatar_file_cleanup`).Scan(t.Context(), &queued))
	require.Equal(t, []string{fmt.Sprintf("/uploads/avatars/global/keep-%d.png", orphan.ID)}, queued)
}

// ADR 0077: the hourly cleanup deletes a caterer business once nobody needs
// it: no school's supply names it, running or ended, none of its accounts is
// left unerased, and no invitation to one of their addresses is open.
func TestAccountRetentionDeletesCaterersNobodyNeeds(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	school := testpkg.Tenant(t)
	testpkg.EnsureTestTenant(t, db, school)
	exec := func(query string, args ...any) {
		t.Helper()
		_, err := db.ExecContext(t.Context(), query, args...)
		require.NoError(t, err)
	}
	caterer := func(name string, accountNames ...string) int64 {
		t.Helper()
		var id int64
		require.NoError(t, db.NewRaw(`INSERT INTO auth.caterers (name) VALUES (?) RETURNING id`, name).Scan(t.Context(), &id))
		for _, accountName := range accountNames {
			account := testpkg.CreateTestAccount(t, db, accountName)
			exec(`INSERT INTO auth.caterer_accounts (account_id, caterer_id) VALUES (?, ?)`, account.ID, id)
			if accountName != "caterer-live" {
				exec(`UPDATE auth.accounts SET personal_data_erased_at = NOW() WHERE id = ?`, account.ID)
			}
		}
		return id
	}
	unused := caterer("Ohne Schule", "caterer-erased")
	withoutAccounts := caterer("Ohne Konto")
	ended := caterer("Beendete Belieferung", "caterer-ended")
	exec(`INSERT INTO auth.caterer_schools (caterer_id, tenant_id, started_at, ended_at) VALUES (?, ?, NOW() - INTERVAL '2 days', NOW() - INTERVAL '1 day')`, ended, school)
	running := caterer("Laufende Belieferung", "caterer-running")
	exec(`INSERT INTO auth.caterer_schools (caterer_id, tenant_id) VALUES (?, ?)`, running, school)
	live := caterer("Konto in Gebrauch", "caterer-live")
	invited := caterer("Offene Einladung", "caterer-invited")
	exec(`INSERT INTO auth.caterer_invitations (tenant_id, token_hash, email, caterer_name, expires_at)
		SELECT ?, 'caterer-retention-' || account.id, account.email, 'Offene Einladung', NOW() + INTERVAL '7 days'
		FROM auth.caterer_accounts AS link JOIN auth.accounts AS account ON account.id = link.account_id
		WHERE link.caterer_id = ?`, school, invited)

	owner, err := compose.New(compose.Dependencies{DB: db, Observe: func(compose.Observation) {}})
	require.NoError(t, err)
	base := testpkg.WithTenantRuntime(t, t.Context(), db)
	var deleted int
	require.NoError(t, testpkg.WithinAdminTransaction(base, func(ctx context.Context) error {
		deleted, err = owner.DeleteUnusedCaterers(ctx)
		return err
	}))
	require.Equal(t, 2, deleted)

	var remaining []int64
	require.NoError(t, db.NewRaw(`SELECT id FROM auth.caterers WHERE id IN (?) ORDER BY id`, bun.List([]int64{unused, withoutAccounts, ended, running, live, invited})).Scan(t.Context(), &remaining))
	require.Equal(t, []int64{ended, running, live, invited}, remaining, "an ended supply stays the school's record, a live account or an open invitation still needs its business")
	var links int
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM auth.caterer_accounts WHERE caterer_id = ?`, unused).Scan(t.Context(), &links))
	require.Zero(t, links, "the account links go with the business")
	var supplies int
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM auth.caterer_schools WHERE caterer_id = ?`, ended).Scan(t.Context(), &supplies))
	require.Equal(t, 1, supplies)
}
