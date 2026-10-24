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
