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

func TestNewGuardianStartsWithoutAChild(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	guardian := testpkg.CreateTestGuardianProfile(t, db, "childless-since")
	var since *time.Time
	require.NoError(t, db.NewRaw(`SELECT childless_since FROM users.guardian_profiles WHERE id = ?`, guardian.ID).Scan(context.Background(), &since))
	assert.NotNil(t, since, "a new contact counts from its creation until the nightly run sees a child")
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
	roleless := testpkg.CreateTestAccount(t, db, "roleless")
	parent := testpkg.CreateTestParentGuardianChain(t, db)
	_, staff := testpkg.CreateTestPersonWithAccount(t, db, "Konto", "Person")
	pending := testpkg.CreateTestAccount(t, db, "pending")
	exec(`UPDATE auth.account_tenants SET status = 'pending' WHERE account_id = ?`, pending.ID)
	teamRole := testpkg.CreateTestRoleForTenant(t, db, "Team", school)
	team := testpkg.CreateTestAccount(t, db, "team")
	exec(`INSERT INTO auth.account_roles (account_id, role_id, tenant_id) VALUES (?, ?, ?)`, team.ID, teamRole.ID, school)
	directPermission := testpkg.CreateTestAccount(t, db, "direct-permission")
	exec(`INSERT INTO auth.account_roles (account_id, role_id, tenant_id)
		SELECT ?, id, ? FROM auth.roles WHERE name = 'guardian'`, directPermission.ID, school)
	permission := testpkg.CreateTestPermission(t, db, "direct-retention", "direct-retention", "read")
	exec(`INSERT INTO auth.account_permissions (account_id, permission_id, tenant_id, granted)
		VALUES (?, ?, ?, TRUE)`, directPermission.ID, permission.ID, school)
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
	}, pick(erased, formerParent.ID, orphan.ID, roleless.ID, parent.AccountID, staff.ID, pending.ID, team.ID, directPermission.ID, invited.ID, relative.ID),
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

// Every table the guard classifies as going with its account
// (parent("auth.accounts")) goes with the tombstone too: the account row is
// never deleted, so its cascade never fires.
func TestEraseUnusedAccountsDeletesEveryTableThatGoesWithTheAccount(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	var definition string
	require.NoError(t, db.NewRaw(`SELECT pg_get_functiondef('auth.erase_unused_accounts()'::regprocedure)`).
		Scan(context.Background(), &definition))
	for table, retention := range tableRetentions {
		if retention == parent("auth.accounts") {
			assert.Contains(t, definition, "DELETE FROM "+table+" ", "%s goes with the account", table)
		}
	}
}

func TestAccountReferencesWaitForErasureAndRejectTheTombstone(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := context.Background()
	tenantID := testpkg.Tenant(t)
	testpkg.EnsureTestTenant(t, db, tenantID)
	account := testpkg.CreateTestAccount(t, db, "account-reference-lock")
	testpkg.UnclaimTestAccount(t, db, account.ID)
	person := testpkg.CreateTestPerson(t, db, "Person", "Link")
	guardian := testpkg.CreateTestGuardianProfileForTenant(t, db, tenantID, "Guardian", "Link", "account-reference-guardian")
	student := testpkg.CreateTestStudentForTenant(t, db, tenantID, "Student", "Link", "1a")
	guardianAccess := testpkg.CreateTestStudentGuardianLinkForTenant(t, db, tenantID, student.ID, guardian.ID, "parent")
	phase := testpkg.CreateTestEnrollmentPhase(t, db)
	request := testpkg.CreateTestEnrollmentRequestAwaitingGuardian(t, db, phase.ID, "account-reference-request@example.invalid", fmt.Sprintf("account-reference-request-%d", account.ID))
	var demoAccessID int64
	require.NoError(t, db.NewRaw(`INSERT INTO auth.demo_accesses
		(email, first_name, last_name, school_name, token_hash, expires_at, school_slug)
		VALUES (?, 'Demo', 'Referenz', 'OGS Beispiel', ?, NOW() + INTERVAL '1 day', ?)
		RETURNING id`,
		fmt.Sprintf("account-reference-demo-%d@example.invalid", account.ID),
		fmt.Sprintf("account-reference-demo-%d", account.ID),
		fmt.Sprintf("account-reference-demo-%d", account.ID)).Scan(ctx, &demoAccessID))
	role := testpkg.CreateTestRoleForTenant(t, db, "account-reference-role", tenantID)
	permission := testpkg.CreateTestPermission(t, db, "account-reference", "account-reference", "read")

	erasure, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer func() { _ = erasure.Rollback() }()
	_, err = erasure.ExecContext(ctx, `SELECT id FROM auth.accounts WHERE id = ? FOR UPDATE`, account.ID)
	require.NoError(t, err)

	linked := make(chan error, 1)
	go func() {
		_, linkErr := db.ExecContext(ctx, `UPDATE users.persons SET account_id = ? WHERE id = ?`, account.ID, person.ID)
		linked <- linkErr
	}()
	select {
	case linkErr := <-linked:
		t.Fatalf("person-account link did not wait for the erasure lock: %v", linkErr)
	case <-time.After(500 * time.Millisecond):
	}
	_, err = erasure.ExecContext(ctx, `UPDATE auth.accounts SET personal_data_erased_at = NOW() WHERE id = ?`, account.ID)
	require.NoError(t, err)
	require.NoError(t, erasure.Commit())
	require.ErrorContains(t, <-linked, "cannot reference an erased account")

	var linkedAccountID *int64
	require.NoError(t, db.NewRaw(`SELECT account_id FROM users.persons WHERE id = ?`, person.ID).Scan(ctx, &linkedAccountID))
	assert.Nil(t, linkedAccountID)

	for name, write := range map[string]struct {
		query string
		args  []any
	}{
		"role": {
			query: `INSERT INTO auth.account_roles (account_id, role_id, tenant_id) VALUES (?, ?, ?)`,
			args:  []any{account.ID, role.ID, tenantID},
		},
		"permission": {
			query: `INSERT INTO auth.account_permissions (account_id, permission_id, granted, tenant_id) VALUES (?, ?, TRUE, ?)`,
			args:  []any{account.ID, permission.ID, tenantID},
		},
		"tenant mapping": {
			query: `INSERT INTO auth.account_tenants (account_id, tenant_id, status, activated_at, created_at, updated_at)
				VALUES (?, ?, 'active', NOW(), NOW(), NOW())`,
			args: []any{account.ID, tenantID},
		},
		"guardian profile": {
			query: `UPDATE users.guardian_profiles SET account_id = ?, has_account = TRUE WHERE id = ?`,
			args:  []any{account.ID, guardian.ID},
		},
		"profile": {
			query: `INSERT INTO users.profiles (account_id, tenant_id, bio, settings) VALUES (?, ?, 'erased profile', '{}'::jsonb)`,
			args:  []any{account.ID, tenantID},
		},
		"guardian access": {
			query: `UPDATE auth.guardian_student_access SET account_id = ? WHERE tenant_id = ? AND relationship_id = ?`,
			args:  []any{account.ID, tenantID, guardianAccess.ID},
		},
		"enrollment request": {
			query: `UPDATE enrollment.requests SET guardian_account_id = ? WHERE id = ?`,
			args:  []any{account.ID, request.ID},
		},
		"demo access": {
			query: `UPDATE auth.demo_accesses SET account_id = ? WHERE id = ?`,
			args:  []any{account.ID, demoAccessID},
		},
		"demo parent access": {
			query: `UPDATE auth.demo_accesses SET parent_account_id = ? WHERE id = ?`,
			args:  []any{account.ID, demoAccessID},
		},
	} {
		t.Run(name, func(t *testing.T) {
			_, writeErr := db.ExecContext(ctx, write.query, write.args...)
			require.ErrorContains(t, writeErr, "cannot reference an erased account")
		})
	}

	var guardianAccountID, accessAccountID, requestAccountID, demoAccountID, demoParentAccountID *int64
	var profileCount int
	require.NoError(t, db.NewRaw(`SELECT account_id FROM users.guardian_profiles WHERE id = ?`, guardian.ID).Scan(ctx, &guardianAccountID))
	require.NoError(t, db.NewRaw(`SELECT COUNT(*) FROM users.profiles WHERE account_id = ?`, account.ID).Scan(ctx, &profileCount))
	require.NoError(t, db.NewRaw(`SELECT account_id FROM auth.guardian_student_access WHERE relationship_id = ?`, guardianAccess.ID).Scan(ctx, &accessAccountID))
	require.NoError(t, db.NewRaw(`SELECT guardian_account_id FROM enrollment.requests WHERE id = ?`, request.ID).Scan(ctx, &requestAccountID))
	require.NoError(t, db.NewRaw(`SELECT account_id, parent_account_id FROM auth.demo_accesses WHERE id = ?`, demoAccessID).Scan(ctx, &demoAccountID, &demoParentAccountID))
	assert.Nil(t, guardianAccountID)
	assert.Zero(t, profileCount)
	assert.Nil(t, accessAccountID)
	assert.Nil(t, requestAccountID)
	assert.Nil(t, demoAccountID)
	assert.Nil(t, demoParentAccountID)
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

	// The caterer tables of 1.15.479 fence their account references with
	// this migration's trigger function; they go first and come back last.
	require.NoError(t, mealCaterersDown(ctx, db))
	t.Cleanup(func() { require.NoError(t, mealCaterersUp(ctx, db)) })
	require.NoError(t, accountGuardianRetentionDown(ctx, db))
	require.ErrorContains(t, logRow(), "chk_data_deletions_subject", "before 1.15.464 the data kind needs a subject")
	_, err := db.ExecContext(ctx, `INSERT INTO audit.data_deletions (tenant_id, deletion_type, records_deleted, deleted_by)
		VALUES (?, 'attendance_retention', 1, 'system')`, school)
	require.NoError(t, err, "the rollback keeps the data kinds of 1.15.460")
	var functions int
	require.NoError(t, db.NewRaw(`SELECT COUNT(*) FROM pg_proc WHERE proname IN ('erase_unused_accounts', 'account_is_unused')`).Scan(ctx, &functions))
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
