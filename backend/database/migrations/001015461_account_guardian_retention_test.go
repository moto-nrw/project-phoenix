package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"testing"
	"time"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func TestGuardianChildlessSinceFollowsTheRelationships(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := context.Background()
	childlessSince := func(guardianID int64) *time.Time {
		var since *time.Time
		require.NoError(t, db.NewRaw(`SELECT childless_since FROM users.guardian_profiles WHERE id = ?`, guardianID).Scan(ctx, &since))
		return since
	}

	guardian := testpkg.CreateTestGuardianProfile(t, db, "childless-since")
	require.NotNil(t, childlessSince(guardian.ID), "a new contact has no child yet")

	first := testpkg.CreateTestStudent(t, db, "Erstes", "Kind", "1a")
	second := testpkg.CreateTestStudent(t, db, "Zweites", "Kind", "1a")
	firstLink := testpkg.CreateTestStudentGuardianLink(t, db, first.ID, guardian.ID, "primary_guardian")
	secondLink := testpkg.CreateTestStudentGuardianLink(t, db, second.ID, guardian.ID, "primary_guardian")
	assert.Nil(t, childlessSince(guardian.ID), "the first relationship clears the mark")

	_, err := db.ExecContext(ctx, `DELETE FROM users.student_guardian_relationships WHERE id = ?`, firstLink.ID)
	require.NoError(t, err)
	assert.Nil(t, childlessSince(guardian.ID), "a contact with a remaining child is not childless")

	_, err = db.ExecContext(ctx, `DELETE FROM users.student_guardian_relationships WHERE id = ?`, secondLink.ID)
	require.NoError(t, err)
	assert.NotNil(t, childlessSince(guardian.ID), "removing the last relationship marks the contact")
}

func TestEraseUnusedAccountsKeepsEveryAccountASchoolStillNeeds(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := context.Background()
	school := testpkg.Tenant(t)
	testpkg.EnsureTestTenant(t, db, school)
	exec := func(query string, args ...any) {
		t.Helper()
		_, err := db.ExecContext(ctx, query, args...)
		require.NoError(t, err)
	}

	// Erased: a parent whose contact is gone keeps only the guardian role.
	formerParent := testpkg.CreateTestAccount(t, db, "former-parent")
	exec(`INSERT INTO auth.account_roles (account_id, role_id, tenant_id)
		SELECT ?, id, ? FROM auth.roles WHERE name = 'guardian'`, formerParent.ID, school)
	exec(`UPDATE auth.accounts SET avatar = '/uploads/avatars/former.png', username = ?, calendar_feed_token = 'feed' WHERE id = ?`,
		fmt.Sprintf("former-%d", formerParent.ID), formerParent.ID)
	exec(`INSERT INTO users.profiles (tenant_id, account_id, avatar) VALUES (?, ?, '/uploads/avatars/former-profile.png')`, school, formerParent.ID)
	// Erased: an account no school maps any more.
	orphan := testpkg.CreateTestAccount(t, db, "orphan")
	testpkg.UnclaimTestAccount(t, db, orphan.ID)

	// Kept: the school still needs these accounts.
	parent := testpkg.CreateTestParentGuardianChain(t, db)
	_, staff := testpkg.CreateTestPersonWithAccount(t, db, "Konto", "Person")
	pending := testpkg.CreateTestAccount(t, db, "pending")
	exec(`UPDATE auth.account_tenants SET status = 'pending' WHERE account_id = ?`, pending.ID)
	teamRole := testpkg.CreateTestRoleForTenant(t, db, "Team", school)
	team := testpkg.CreateTestAccount(t, db, "team")
	exec(`INSERT INTO auth.account_roles (account_id, role_id, tenant_id) VALUES (?, ?, ?)`, team.ID, teamRole.ID, school)
	invited := testpkg.CreateTestAccount(t, db, "invited")
	testpkg.UnclaimTestAccount(t, db, invited.ID)
	exec(`INSERT INTO auth.invitation_tokens (tenant_id, email, token, role_id, created_by, expires_at)
		VALUES (?, UPPER(?), ?, ?, ?, NOW() + INTERVAL '1 day')`, school, invited.Email, fmt.Sprintf("token-%d", invited.ID), teamRole.ID, team.ID)

	relative := testpkg.CreateTestAccount(t, db, "relative")
	testpkg.UnclaimTestAccount(t, db, relative.ID)
	var invitedProfileID int64
	require.NoError(t, db.NewRaw(`INSERT INTO users.guardian_profiles (tenant_id, first_name, last_name, email)
		VALUES (?, 'Eingeladen', 'Verwandt', ?) RETURNING id`, school, relative.Email).Scan(ctx, &invitedProfileID))
	exec(`INSERT INTO auth.guardian_invitations (tenant_id, token, guardian_profile_id, created_by, expires_at)
		VALUES (?, ?, ?, ?, NOW() + INTERVAL '1 day')`, school, fmt.Sprintf("guardian-token-%d", relative.ID), invitedProfileID, team.ID)

	erased := eraseUnusedAccountsAsAdmin(t, db)
	assert.Equal(t, map[int64][]string{
		formerParent.ID: {"/uploads/avatars/former-profile.png", "/uploads/avatars/former.png"},
		orphan.ID:       nil,
	}, pick(erased, formerParent.ID, orphan.ID, parent.AccountID, staff.ID, pending.ID, team.ID, invited.ID, relative.ID),
		"only accounts no school needs are erased; their avatar files come back for removal")

	var tombstone struct {
		Email        string     `bun:"email"`
		Username     *string    `bun:"username"`
		Active       bool       `bun:"active"`
		PasswordHash *string    `bun:"password_hash"`
		Avatar       *string    `bun:"avatar"`
		FeedToken    *string    `bun:"calendar_feed_token"`
		ErasedAt     *time.Time `bun:"personal_data_erased_at"`
	}
	require.NoError(t, db.NewRaw(`SELECT email, username, active, password_hash, avatar, calendar_feed_token, personal_data_erased_at
		FROM auth.accounts WHERE id = ?`, formerParent.ID).Scan(ctx, &tombstone))
	assert.Equal(t, fmt.Sprintf("geloescht-%d@example.invalid", formerParent.ID), tombstone.Email)
	assert.Nil(t, tombstone.Username)
	assert.False(t, tombstone.Active)
	assert.Nil(t, tombstone.PasswordHash)
	assert.Nil(t, tombstone.Avatar)
	assert.Nil(t, tombstone.FeedToken)
	assert.NotNil(t, tombstone.ErasedAt)
	for _, table := range []string{"auth.account_tenants", "auth.account_roles", "users.profiles"} {
		var left int
		require.NoError(t, db.NewRaw(`SELECT COUNT(*) FROM `+table+` WHERE account_id = ?`, formerParent.ID).Scan(ctx, &left))
		assert.Zero(t, left, "%s of the erased account is deleted", table)
	}

	again := eraseUnusedAccountsAsAdmin(t, db)
	assert.Empty(t, pick(again, formerParent.ID, orphan.ID), "an erased account is not erased twice")
}

func TestOnlyTheAdminRoleMayEraseUnusedAccounts(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := context.Background()
	for function, admin := range map[string]bool{"auth.erase_unused_accounts()": true, "auth.account_is_unused(bigint, text)": false} {
		for role, allowed := range map[string]bool{"phoenix_admin": admin, "phoenix_tenant": false, "phoenix_auth": false, "phoenix_demo": false} {
			var canCall bool
			require.NoError(t, db.NewRaw(`SELECT has_function_privilege(?, ?, 'EXECUTE')`, role, function).Scan(ctx, &canCall))
			assert.Equal(t, allowed, canCall, function+" / "+role)
		}
	}
}

func TestDeletionLogAcceptsGuardianRetentionRowsWithoutASubject(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	school := testpkg.Tenant(t)

	_, err := db.ExecContext(context.Background(), `
		INSERT INTO audit.data_deletions (tenant_id, deletion_type, records_deleted, deleted_by)
		VALUES (?, ?, 1, 'system')`, school, guardianRetentionKind)
	assert.NoError(t, err, "the guardian retention logs a data kind without a subject")
}

func TestAccountGuardianRetentionRollsBack(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := context.Background()
	school := testpkg.Tenant(t)
	testpkg.EnsureTestTenant(t, db, school)
	logRow := func() error {
		_, err := db.ExecContext(ctx, `INSERT INTO audit.data_deletions (tenant_id, deletion_type, records_deleted, deleted_by)
			VALUES (?, ?, 1, 'system')`, school, guardianRetentionKind)
		return err
	}

	require.NoError(t, accountGuardianRetentionDown(ctx, db))
	require.ErrorContains(t, logRow(), "chk_data_deletions_subject", "before 1.15.461 the data kind needs a subject")
	_, err := db.ExecContext(ctx, `INSERT INTO audit.data_deletions (tenant_id, deletion_type, records_deleted, deleted_by)
		VALUES (?, 'attendance_retention', 1, 'system')`, school)
	require.NoError(t, err, "the rollback keeps the data kinds of 1.15.460")
	var functions int
	require.NoError(t, db.NewRaw(`SELECT COUNT(*) FROM pg_proc WHERE proname IN ('erase_unused_accounts', 'track_guardian_childless_since')`).Scan(ctx, &functions))
	assert.Zero(t, functions, "the rollback removes the functions")

	require.NoError(t, accountGuardianRetentionUp(ctx, db))
	require.NoError(t, logRow())
	assert.ErrorContains(t, accountGuardianRetentionDown(ctx, db), "guardian retention rows exist",
		"the subject check cannot be restored over logged retention rows")
}

// eraseUnusedAccountsAsAdmin runs the hourly erasure the way the token
// cleanup does and returns each erased account with its avatar files.
func eraseUnusedAccountsAsAdmin(t *testing.T, db *bun.DB) map[int64][]string {
	t.Helper()
	erased := map[int64][]string{}
	err := db.RunInTx(context.Background(), nil, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.ExecContext(ctx, `SET LOCAL ROLE phoenix_admin`); err != nil {
			return err
		}
		var rows []struct {
			AccountID int64          `bun:"account_id"`
			Avatar    sql.NullString `bun:"avatar"`
		}
		if err := tx.NewRaw(`SELECT account_id, avatar FROM auth.erase_unused_accounts()`).Scan(ctx, &rows); err != nil {
			return err
		}
		for _, row := range rows {
			files := erased[row.AccountID]
			if row.Avatar.Valid {
				files = append(files, row.Avatar.String)
			}
			erased[row.AccountID] = files
		}
		return nil
	})
	require.NoError(t, err)
	for _, files := range erased {
		sort.Strings(files)
	}
	return erased
}

// pick keeps the entries of the given accounts; an erased account without an
// avatar file shows up with a nil list.
func pick(erased map[int64][]string, accountIDs ...int64) map[int64][]string {
	picked := map[int64][]string{}
	for _, id := range accountIDs {
		if files, ok := erased[id]; ok {
			picked[id] = files
		}
	}
	return picked
}
