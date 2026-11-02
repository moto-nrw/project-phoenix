package compose

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// onlyInSchools leaves the account mapped to exactly the given schools.
func onlyInSchools(t *testing.T, db *bun.DB, accountID int64, schoolIDs ...int64) {
	t.Helper()
	_, err := db.NewRaw(`DELETE FROM auth.account_tenants WHERE account_id = ?`, accountID).Exec(t.Context())
	require.NoError(t, err)
	for _, schoolID := range schoolIDs {
		testpkg.MapAccountToTenant(t, db, accountID, schoolID)
	}
}

// finishedVisitorOrder orders a demo school for the visitor accounts and
// finishes it, ready to be retired.
func finishedVisitorOrder(t *testing.T, db *bun.DB, schoolID int64, accounts visitorAccounts) (*organizationtenancy.DemoSchoolQueue, string) {
	t.Helper()
	ctx := t.Context()
	queue, err := NewDemoSchoolQueue(db)
	require.NoError(t, err)
	schools, err := NewDemoSchools(db)
	require.NoError(t, err)
	slug := orderDemoSchool(t, db, "OGS Nord")
	_, err = queue.ClaimDemoSchoolOrder(ctx)
	require.NoError(t, err)
	require.NoError(t, schools.RememberDemoSchool(ctx, slug, organizationtenancy.DemoSchoolState{
		SchoolID: schoolID, SeedJSON: []byte(`{}`),
		VisitorAccountID: accounts.caregiver, VisitorParentAccountID: accounts.parent,
	}))
	require.NoError(t, queue.FinishDemoSchoolOrder(ctx, slug, accounts.caregiver, accounts.parent))
	return queue, slug
}

func requireNothingLeftToErase(t *testing.T, queue *organizationtenancy.DemoSchoolQueue) {
	t.Helper()
	_, erased, err := queue.RetireDemoSchools(t.Context(), nil)
	require.NoError(t, err)
	assert.Zero(t, erased, "a second run finds nothing")
}

func pushSubscription(t *testing.T, db *bun.DB, schoolID, accountID int64, portal string) int64 {
	t.Helper()
	var id int64
	require.NoError(t, db.NewRaw(`INSERT INTO iot.push_subscriptions (tenant_id, account_id, portal, endpoint, p256dh, auth, user_agent)
		VALUES (?, ?, ?, 'https://push.example.org/' || ? || '/' || ?, 'p256dh-key', 'auth-key', 'Mozilla/5.0 (Besuchsgerät)')
		RETURNING id`, schoolID, accountID, portal, schoolID, accountID).Scan(t.Context(), &id))
	return id
}

func remainingIDs(t *testing.T, db *bun.DB, table string, ids ...int64) []int64 {
	t.Helper()
	remaining := []int64{}
	require.NoError(t, db.NewRaw(`SELECT id FROM `+table+` WHERE id IN (?) ORDER BY id`, bun.List(ids)).Scan(t.Context(), &remaining))
	return remaining
}

// The push subscriptions of a retired demo school hold the endpoint and the
// user agent of the visitor's devices (#5540). They go with the school, and
// an account the erasure anonymizes loses every subscription it has. A
// visitor account that also belongs to a real school keeps that school's
// subscription.
func TestDemoSchoolQueueErasesThePushSubscriptionsOfTheVisitor(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	schoolID, _ := testpkg.CreateTestTenant(t, db)
	realID, _ := testpkg.CreateTestTenant(t, db)
	accounts := leaveVisitorTraces(t, db, schoolID)
	onlyInSchools(t, db, accounts.caregiver, schoolID)
	onlyInSchools(t, db, accounts.parent, schoolID, realID)
	otherAccount := testpkg.CreateTestAccount(t, db, fmt.Sprintf("echte-schule-%d@example.org", realID))
	onlyInSchools(t, db, otherAccount.ID, realID)

	caregiverInDemo := pushSubscription(t, db, schoolID, accounts.caregiver, "staff")
	parentInDemo := pushSubscription(t, db, schoolID, accounts.parent, "parent")
	caregiverElsewhere := pushSubscription(t, db, realID, accounts.caregiver, "staff")
	parentInReal := pushSubscription(t, db, realID, accounts.parent, "parent")
	otherInReal := pushSubscription(t, db, realID, otherAccount.ID, "staff")

	queue, slug := finishedVisitorOrder(t, db, schoolID, accounts)
	_, erased, err := queue.RetireDemoSchools(t.Context(), []string{slug})
	require.NoError(t, err)
	require.Equal(t, 1, erased)

	ids := []int64{caregiverInDemo, parentInDemo, caregiverElsewhere, parentInReal, otherInReal}
	assert.Equal(t, []int64{parentInReal, otherInReal}, remainingIDs(t, db, "iot.push_subscriptions", ids...),
		"the retired school and the anonymized account keep no push subscription; the real school keeps its own")
	requireNothingLeftToErase(t, queue)
	assert.Equal(t, []int64{parentInReal, otherInReal}, remainingIDs(t, db, "iot.push_subscriptions", ids...))
}

// An anonymized visitor account loses what the general account erasure
// removes (#5540): its profile, its settings, and its avatar files, which go
// to the durable file cleanup before their references disappear (#4756). A
// visitor account that also belongs to a real school keeps all of it.
func TestDemoSchoolQueueErasesTheProfileAndAvatarFilesOfAnAnonymizedVisitor(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := t.Context()
	schoolID, _ := testpkg.CreateTestTenant(t, db)
	realID, _ := testpkg.CreateTestTenant(t, db)
	accounts := leaveVisitorTraces(t, db, schoolID)
	onlyInSchools(t, db, accounts.caregiver, schoolID)
	onlyInSchools(t, db, accounts.parent, schoolID, realID)
	avatar := func(kind string, accountID int64) string {
		return fmt.Sprintf("/uploads/avatars/demo-%s-%d.png", kind, accountID)
	}
	for _, accountID := range []int64{accounts.caregiver, accounts.parent} {
		_, err := db.NewRaw(`UPDATE auth.accounts SET avatar = ? WHERE id = ?`, avatar("account", accountID), accountID).Exec(ctx)
		require.NoError(t, err)
		_, err = db.NewRaw(`INSERT INTO users.profiles (tenant_id, account_id, avatar, bio) VALUES (?, ?, ?, 'Ich bin Anna Lena')`,
			schoolID, accountID, avatar("profile", accountID)).Exec(ctx)
		require.NoError(t, err)
		_, err = db.NewRaw(`INSERT INTO iot.pwa_standalone_usage (tenant_id, account_id, portal)
			VALUES (?, ?, 'staff')`, schoolID, accountID).Exec(ctx)
		require.NoError(t, err)
	}

	queue, slug := finishedVisitorOrder(t, db, schoolID, accounts)
	_, erased, err := queue.RetireDemoSchools(ctx, []string{slug})
	require.NoError(t, err)
	require.Equal(t, 1, erased)

	var queued []string
	require.NoError(t, db.NewRaw(`SELECT avatar FROM auth.account_avatar_file_cleanup WHERE avatar IN (?) ORDER BY avatar`,
		bun.List([]string{avatar("account", accounts.caregiver), avatar("profile", accounts.caregiver),
			avatar("account", accounts.parent), avatar("profile", accounts.parent)})).Scan(ctx, &queued))
	assert.Equal(t, []string{avatar("account", accounts.caregiver), avatar("profile", accounts.caregiver)}, queued,
		"the anonymized account's avatar files are queued for removal; the shared account keeps its own")
	for _, table := range []string{"users.profiles", "iot.pwa_standalone_usage"} {
		var caregiver, parent int
		require.NoError(t, db.NewRaw(`SELECT count(*) FROM `+table+` WHERE account_id = ?`, accounts.caregiver).Scan(ctx, &caregiver), table)
		require.NoError(t, db.NewRaw(`SELECT count(*) FROM `+table+` WHERE account_id = ?`, accounts.parent).Scan(ctx, &parent), table)
		assert.Zero(t, caregiver, "%s of the anonymized account is deleted", table)
		assert.Equal(t, 1, parent, "%s of the shared account stays", table)
	}
	var kept string
	require.NoError(t, db.NewRaw(`SELECT avatar FROM auth.accounts WHERE id = ?`, accounts.parent).Scan(ctx, &kept))
	assert.Equal(t, avatar("account", accounts.parent), kept)

	requireNothingLeftToErase(t, queue)
	var stillQueued int
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM auth.account_avatar_file_cleanup WHERE avatar IN (?)`,
		bun.List([]string{avatar("account", accounts.caregiver), avatar("profile", accounts.caregiver)})).Scan(ctx, &stillQueued))
	assert.Equal(t, 2, stillQueued, "a second run queues nothing twice")
}

// accountState is what tells an anonymized account from a kept one.
type accountState struct {
	Email  string     `bun:"email"`
	Active bool       `bun:"active"`
	Erased *time.Time `bun:"personal_data_erased_at"`
}

func stateOfAccount(t *testing.T, db *bun.DB, accountID int64) accountState {
	t.Helper()
	var state accountState
	require.NoError(t, db.NewRaw(`SELECT email, active, personal_data_erased_at FROM auth.accounts WHERE id = ?`, accountID).
		Scan(t.Context(), &state))
	return state
}

// retireNow runs the erasure and fails the test when it waits: the erasure
// never waits for an account that another transaction holds.
func retireNow(t *testing.T, queue *organizationtenancy.DemoSchoolQueue, slug string) int {
	t.Helper()
	type outcome struct {
		erased int
		err    error
	}
	done := make(chan outcome, 1)
	go func() {
		_, erased, err := queue.RetireDemoSchools(t.Context(), []string{slug})
		done <- outcome{erased: erased, err: err}
	}()
	select {
	case result := <-done:
		require.NoError(t, result.err)
		return result.erased
	case <-time.After(5 * time.Second):
		t.Fatal("the erasure waited for an account another transaction holds")
		return 0
	}
}

// The check "this account hangs only in demo schools" must see every
// membership (#5540). The erasure locks the accounts without waiting: while
// another transaction holds an account, writing a school or Träger
// membership, the erasure leaves the whole order for its next run and
// changes nothing. The next run sees the membership and keeps the account.
func eraseWhileAMembershipIsWritten(t *testing.T, membership func(ctx context.Context, tx bun.Tx, accountID, realID int64) error) {
	t.Helper()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := t.Context()
	schoolID, _ := testpkg.CreateTestTenant(t, db)
	realID, _ := testpkg.CreateTestTenant(t, db)
	accounts := leaveVisitorTraces(t, db, schoolID)
	onlyInSchools(t, db, accounts.caregiver, schoolID)
	onlyInSchools(t, db, accounts.parent, schoolID)
	caregiverBefore, parentBefore := stateOfAccount(t, db, accounts.caregiver), stateOfAccount(t, db, accounts.parent)
	queue, slug := finishedVisitorOrder(t, db, schoolID, accounts)

	writer, err := db.BeginTx(ctx, &sql.TxOptions{})
	require.NoError(t, err)
	defer func() { _ = writer.Rollback() }()
	require.NoError(t, membership(ctx, writer, accounts.caregiver, realID))

	assert.Zero(t, retireNow(t, queue, slug), "the order waits for the next run")
	assert.Equal(t, everyTrace(1), visitorTraces(t, db, schoolID), "a deferred order changes nothing")
	assert.Equal(t, parentBefore, stateOfAccount(t, db, accounts.parent))
	require.NoError(t, writer.Commit())

	assert.Equal(t, 1, retireNow(t, queue, slug))
	assert.Equal(t, caregiverBefore, stateOfAccount(t, db, accounts.caregiver),
		"an account that got a real membership while the erasure ran keeps its address and access")
	parent := stateOfAccount(t, db, accounts.parent)
	assert.Equal(t, fmt.Sprintf("demo-besuch-%d@example.invalid", accounts.parent), parent.Email)
	assert.False(t, parent.Active)
	assert.NotNil(t, parent.Erased, "the demo-only account is erased")

	// A school that tries to take on the erased account afterwards is
	// refused, as for every erased account (#4043).
	_, err = db.NewRaw(`INSERT INTO auth.account_tenants (account_id, tenant_id, status) VALUES (?, ?, 'active')`,
		accounts.parent, realID).Exec(ctx)
	require.ErrorContains(t, err, "cannot reference an erased account")
	requireNothingLeftToErase(t, queue)
}

func TestDemoVisitorErasureDefersWhileASchoolMembershipIsWritten(t *testing.T) {
	t.Parallel()
	eraseWhileAMembershipIsWritten(t, func(ctx context.Context, tx bun.Tx, accountID, realID int64) error {
		_, err := tx.ExecContext(ctx, `INSERT INTO auth.account_tenants (account_id, tenant_id, status) VALUES (?, ?, 'active')`, accountID, realID)
		return err
	})
}

func TestDemoVisitorErasureDefersWhileATraegerMembershipIsWritten(t *testing.T) {
	t.Parallel()
	eraseWhileAMembershipIsWritten(t, func(ctx context.Context, tx bun.Tx, accountID, realID int64) error {
		_, err := tx.ExecContext(ctx, `INSERT INTO platform.organization_accounts (organization_id, account_id, status, activated_at)
			SELECT organization_id, ?, 'active', NOW() FROM platform.schools WHERE id = ?`, accountID, realID)
		return err
	})
}

// lockWaits reports whether another session runs a statement containing
// marker and waits for a lock.
func lockWaits(t *testing.T, db *bun.DB, marker string) bool {
	t.Helper()
	var waiting int
	err := db.NewRaw(`SELECT count(*) FROM pg_stat_activity
		WHERE datname = current_database() AND pid <> pg_backend_pid()
			AND wait_event_type = 'Lock' AND strpos(query, ?) > 0`, marker).Scan(t.Context(), &waiting)
	return err == nil && waiting > 0
}

// An UPDATE of a membership locks the row first and its account second, in
// auth.lock_live_account_reference. The erasure never waits for a lock
// (#5540): stopped at its first write, it gives up after lock_timeout and
// defers the order. Here the staff calendar feed writes its token into the
// visitor's membership meanwhile; neither fails, and the next run erases.
func TestDemoVisitorErasureLocksTheMembershipBeforeTheAccount(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := t.Context()
	schoolID, _ := testpkg.CreateTestTenant(t, db)
	accounts := leaveVisitorTraces(t, db, schoolID)
	onlyInSchools(t, db, accounts.caregiver, schoolID)
	onlyInSchools(t, db, accounts.parent, schoolID)
	queue, slug := finishedVisitorOrder(t, db, schoolID, accounts)

	// Barrier: the visitor's person row, the first row the erasure writes.
	barrier, err := db.BeginTx(ctx, &sql.TxOptions{})
	require.NoError(t, err)
	defer func() { _ = barrier.Rollback() }()
	_, err = barrier.ExecContext(ctx, `SELECT id FROM users.persons WHERE account_id = ? FOR UPDATE`, accounts.caregiver)
	require.NoError(t, err)

	assert.Zero(t, retireNow(t, queue, slug), "the order waits for the next run")
	// EnsureStaffCalendarFeedToken: it locks the membership row, then its
	// trigger locks the account.
	_, err = db.ExecContext(ctx, `UPDATE auth.account_tenants SET staff_calendar_feed_token = 'feed-token-hash'
		WHERE account_id = ? AND tenant_id = ? AND status = 'active'
		AND (staff_calendar_feed_token IS NULL OR staff_calendar_feed_token = '')`, accounts.caregiver, schoolID)
	require.NoError(t, err, "the membership update must not fail beside the erasure")
	require.NoError(t, barrier.Rollback())

	assert.Equal(t, 1, retireNow(t, queue, slug))
	assert.Equal(t, fmt.Sprintf("demo-besuch-%d@example.invalid", accounts.caregiver), stateOfAccount(t, db, accounts.caregiver).Email)
	requireNothingLeftToErase(t, queue)
}

// An upsert locks the account first, in auth.lock_live_account_reference,
// and then the conflicting row. While a school writes the visitor's
// membership or role again, the erasure gives up on the account or the row
// after lock_timeout (#5540): it leaves the order for its next run, the
// writer finishes, and the next run erases the order.
func eraseWhileARowIsWrittenAgain(t *testing.T, setup func(ctx context.Context, db *bun.DB, accountID, schoolID int64), upsert string) {
	t.Helper()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := t.Context()
	schoolID, _ := testpkg.CreateTestTenant(t, db)
	accounts := leaveVisitorTraces(t, db, schoolID)
	onlyInSchools(t, db, accounts.caregiver, schoolID)
	onlyInSchools(t, db, accounts.parent, schoolID)
	setup(ctx, db, accounts.caregiver, schoolID)
	queue, slug := finishedVisitorOrder(t, db, schoolID, accounts)

	writer, err := db.BeginTx(ctx, &sql.TxOptions{})
	require.NoError(t, err)
	defer func() { _ = writer.Rollback() }()
	_, err = writer.ExecContext(ctx, upsert, accounts.caregiver, schoolID)
	require.NoError(t, err)

	assert.Zero(t, retireNow(t, queue, slug), "the order waits for the next run")
	require.NoError(t, writer.Commit(), "the writer must not fail beside the erasure")

	assert.Equal(t, 1, retireNow(t, queue, slug))
	assert.NotNil(t, stateOfAccount(t, db, accounts.caregiver).Erased, "the next run erases the demo-only account")
	requireNothingLeftToErase(t, queue)
}

// EnsureActiveTenantMapping writes a membership with an upsert.
func TestDemoVisitorErasureDefersForARewrittenMembership(t *testing.T) {
	t.Parallel()
	eraseWhileARowIsWrittenAgain(t, func(context.Context, *bun.DB, int64, int64) {},
		`INSERT INTO auth.account_tenants (account_id, tenant_id, status) VALUES (?, ?, 'active')
		ON CONFLICT (account_id, tenant_id) DO UPDATE SET status = 'active'`)
}

// AssignAccountRole writes a role with an upsert.
func TestDemoVisitorErasureDefersForARewrittenRole(t *testing.T) {
	t.Parallel()
	eraseWhileARowIsWrittenAgain(t, func(ctx context.Context, db *bun.DB, accountID, schoolID int64) {
		_, err := db.NewRaw(`INSERT INTO auth.account_roles (account_id, role_id, tenant_id)
			SELECT ?, id, ? FROM auth.roles ORDER BY id LIMIT 1`, accountID, schoolID).Exec(ctx)
		require.NoError(t, err)
	}, `INSERT INTO auth.account_roles (account_id, role_id, tenant_id)
		SELECT ?0, id, ?1 FROM auth.roles ORDER BY id LIMIT 1
		ON CONFLICT DO NOTHING`)
}

// RemoveRoleFromAccount locks the account, deletes the role and then every
// session of the account. The erasure deletes the sessions of the visitor
// schools and then their roles. Before #5540 each waited for the other's
// rows: a deadlock that aborted the whole erasure run. Now the erasure gives
// up on the role after lock_timeout and rolls its sessions back; the role
// removal finishes and the next run erases the order.
func TestDemoVisitorErasureDefersWhileARoleIsRemoved(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := t.Context()
	schoolID, _ := testpkg.CreateTestTenant(t, db)
	accounts := leaveVisitorTraces(t, db, schoolID)
	onlyInSchools(t, db, accounts.caregiver, schoolID)
	onlyInSchools(t, db, accounts.parent, schoolID)
	var roleID int64
	require.NoError(t, db.NewRaw(`SELECT id FROM auth.roles ORDER BY id LIMIT 1`).Scan(ctx, &roleID))
	_, err := db.NewRaw(`INSERT INTO auth.account_roles (account_id, role_id, tenant_id) VALUES (?, ?, ?)`, accounts.caregiver, roleID, schoolID).Exec(ctx)
	require.NoError(t, err)
	_, err = db.NewRaw(`INSERT INTO auth.tokens (account_id, token, expiry, tenant_id, portal_scope)
		VALUES (?, encode(sha256(convert_to('role-removal-' || ?::text, 'UTF8')), 'hex'), NOW() + INTERVAL '1 day', ?, 'tenant')`,
		accounts.caregiver, accounts.caregiver, schoolID).Exec(ctx)
	require.NoError(t, err)
	queue, slug := finishedVisitorOrder(t, db, schoolID, accounts)

	// RemoveRoleFromAccount: lockManageableAccount, DeleteAccountRole ...
	removal, err := db.BeginTx(ctx, &sql.TxOptions{})
	require.NoError(t, err)
	defer func() { _ = removal.Rollback() }()
	_, err = removal.ExecContext(ctx, `SELECT id FROM auth.accounts WHERE id = ? FOR UPDATE`, accounts.caregiver)
	require.NoError(t, err)
	_, err = removal.ExecContext(ctx, `DELETE FROM auth.account_roles WHERE account_id = ? AND role_id = ?`, accounts.caregiver, roleID)
	require.NoError(t, err)

	type outcome struct {
		erased int
		err    error
	}
	erasure := make(chan outcome, 1)
	go func() {
		_, erased, retireErr := queue.RetireDemoSchools(ctx, []string{slug})
		erasure <- outcome{erased: erased, err: retireErr}
	}()
	// The erasure holds the visitor school's sessions and waits for the role,
	// or has already given up.
	var result outcome
	finished := false
	require.Eventually(t, func() bool {
		select {
		case result = <-erasure:
			finished = true
			return true
		default:
			return lockWaits(t, db, "retire_expired_demo_schools")
		}
	}, 10*time.Second, 10*time.Millisecond, "the erasure either waits for the role or gives up")

	// ... then DeleteAccountSessionsWithAudit: every session of the account.
	_, err = removal.ExecContext(ctx, `DELETE FROM auth.tokens WHERE account_id = ?`, accounts.caregiver)
	require.NoError(t, err, "the role removal must not deadlock with the erasure")
	require.NoError(t, removal.Commit())
	if !finished {
		result = <-erasure
	}
	require.NoError(t, result.err, "the erasure run must not deadlock with the role removal")
	assert.Zero(t, result.erased, "the order waits for the next run")

	assert.Equal(t, 1, retireNow(t, queue, slug))
	assert.NotNil(t, stateOfAccount(t, db, accounts.caregiver).Erased)
	requireNothingLeftToErase(t, queue)
}

// An order the erasure defers is counted in its state row; a long run of
// deferrals becomes a warning (#5540).
func TestDemoVisitorErasureCountsItsDeferrals(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := t.Context()
	schoolID, _ := testpkg.CreateTestTenant(t, db)
	accounts := leaveVisitorTraces(t, db, schoolID)
	onlyInSchools(t, db, accounts.caregiver, schoolID)
	onlyInSchools(t, db, accounts.parent, schoolID)
	queue, slug := finishedVisitorOrder(t, db, schoolID, accounts)

	holder, err := db.BeginTx(ctx, &sql.TxOptions{})
	require.NoError(t, err)
	defer func() { _ = holder.Rollback() }()
	_, err = holder.ExecContext(ctx, `SELECT id FROM auth.accounts WHERE id = ? FOR UPDATE`, accounts.caregiver)
	require.NoError(t, err)
	for range 2 {
		assert.Zero(t, retireNow(t, queue, slug))
	}
	var deferrals int
	require.NoError(t, db.NewRaw(`SELECT erasure_deferrals FROM platform.demo_school_states WHERE name = ?`, slug).Scan(ctx, &deferrals))
	assert.Equal(t, 2, deferrals)
	require.NoError(t, holder.Rollback())

	assert.Equal(t, 1, retireNow(t, queue, slug))
	assert.Empty(t, demoOrderNames(t, db), "an erased order leaves no state row, counter included")
}

// A profile change holds the account first (UpdateProfile sets the user name,
// then upserts the bio). Whether it changes the profile of a real school or
// of the demo school, the erasure leaves the order for its next run instead
// of waiting for the account (#5540); neither ends in a deadlock.
func eraseWhileAProfileChanges(t *testing.T, inDemoSchool bool) {
	t.Helper()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := t.Context()
	schoolID, _ := testpkg.CreateTestTenant(t, db)
	realID, _ := testpkg.CreateTestTenant(t, db)
	accounts := leaveVisitorTraces(t, db, schoolID)
	onlyInSchools(t, db, accounts.caregiver, schoolID)
	accountID, profileSchool := accounts.parent, realID
	onlyInSchools(t, db, accounts.parent, schoolID, realID)
	if inDemoSchool {
		onlyInSchools(t, db, accounts.parent, schoolID)
		profileSchool = schoolID
	}
	for _, tenantID := range []int64{schoolID, realID} {
		_, err := db.NewRaw(`INSERT INTO users.profiles (tenant_id, account_id, bio) VALUES (?, ?, 'vorher')`, tenantID, accountID).Exec(ctx)
		require.NoError(t, err)
	}
	queue, slug := finishedVisitorOrder(t, db, schoolID, accounts)

	profileChange, err := db.BeginTx(ctx, &sql.TxOptions{})
	require.NoError(t, err)
	defer func() { _ = profileChange.Rollback() }()
	_, err = profileChange.ExecContext(ctx, `UPDATE auth.accounts SET username = 'elternteil-' || id WHERE id = ?`, accountID)
	require.NoError(t, err)

	assert.Zero(t, retireNow(t, queue, slug), "the order waits for the next run")
	_, err = profileChange.ExecContext(ctx, `INSERT INTO users.profiles (account_id, tenant_id, bio, settings)
		VALUES (?, ?, 'nachher', '{}'::jsonb)
		ON CONFLICT (tenant_id, account_id) DO UPDATE SET bio = EXCLUDED.bio`, accountID, profileSchool)
	require.NoError(t, err, "the profile change must not deadlock with the erasure")
	require.NoError(t, profileChange.Commit())

	assert.Equal(t, 1, retireNow(t, queue, slug))
	var profiles int
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM users.profiles WHERE account_id = ?`, accountID).Scan(ctx, &profiles))
	if inDemoSchool {
		assert.Zero(t, profiles, "the demo-only account loses its profiles in the next run")
		assert.NotNil(t, stateOfAccount(t, db, accountID).Erased)
	} else {
		var bio string
		require.NoError(t, db.NewRaw(`SELECT bio FROM users.profiles WHERE account_id = ? AND tenant_id = ?`, accountID, realID).Scan(ctx, &bio))
		assert.Equal(t, "nachher", bio)
		assert.Nil(t, stateOfAccount(t, db, accountID).Erased, "the shared account stays")
	}
	requireNothingLeftToErase(t, queue)
}

func TestDemoVisitorErasureDefersWhileARealSchoolProfileChanges(t *testing.T) {
	t.Parallel()
	eraseWhileAProfileChanges(t, false)
}

func TestDemoVisitorErasureDefersWhileADemoSchoolProfileChanges(t *testing.T) {
	t.Parallel()
	eraseWhileAProfileChanges(t, true)
}

// A role in a real school is an assignment there, even without a membership
// row: the account is no demo-only account and stays (#5540).
func TestDemoVisitorErasureKeepsAnAccountWithARoleInARealSchool(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := t.Context()
	schoolID, _ := testpkg.CreateTestTenant(t, db)
	realID, _ := testpkg.CreateTestTenant(t, db)
	accounts := leaveVisitorTraces(t, db, schoolID)
	onlyInSchools(t, db, accounts.caregiver, schoolID)
	onlyInSchools(t, db, accounts.parent, schoolID)
	var roleID int64
	require.NoError(t, db.NewRaw(`SELECT id FROM auth.roles ORDER BY id LIMIT 1`).Scan(ctx, &roleID))
	_, err := db.NewRaw(`INSERT INTO auth.account_roles (account_id, role_id, tenant_id) VALUES (?, ?, ?)`, accounts.caregiver, roleID, realID).Exec(ctx)
	require.NoError(t, err)
	before := stateOfAccount(t, db, accounts.caregiver)

	queue, slug := finishedVisitorOrder(t, db, schoolID, accounts)
	_, erased, err := queue.RetireDemoSchools(ctx, []string{slug})
	require.NoError(t, err)
	require.Equal(t, 1, erased)

	assert.Equal(t, before, stateOfAccount(t, db, accounts.caregiver), "the account with a role in a real school keeps its address and access")
	var roles int
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM auth.account_roles WHERE account_id = ? AND tenant_id = ?`, accounts.caregiver, realID).Scan(ctx, &roles))
	assert.Equal(t, 1, roles)
	assert.NotNil(t, stateOfAccount(t, db, accounts.parent).Erased, "the demo-only account is erased")
	requireNothingLeftToErase(t, queue)
}
