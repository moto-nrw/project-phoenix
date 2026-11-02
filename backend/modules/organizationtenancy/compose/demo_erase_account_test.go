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

// The check "this account hangs only in demo schools" holds the accounts
// until it is done (#5540): a membership another transaction is writing
// while the erasure runs is either seen by the check, and the account stays,
// or comes too late and finds the account erased, never half of each.
func eraseWhileAMembershipIsWritten(t *testing.T, membership func(ctx context.Context, tx bun.Tx, accountID, realID int64) error) {
	t.Helper()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := t.Context()
	schoolID, _ := testpkg.CreateTestTenant(t, db)
	realID, _ := testpkg.CreateTestTenant(t, db)
	accounts := leaveVisitorTraces(t, db, schoolID)
	onlyInSchools(t, db, accounts.caregiver, schoolID)
	onlyInSchools(t, db, accounts.parent, schoolID)
	before := stateOfAccount(t, db, accounts.caregiver)
	queue, slug := finishedVisitorOrder(t, db, schoolID, accounts)

	writer, err := db.BeginTx(ctx, &sql.TxOptions{})
	require.NoError(t, err)
	defer func() { _ = writer.Rollback() }()
	require.NoError(t, membership(ctx, writer, accounts.caregiver, realID))

	type outcome struct {
		erased int
		err    error
	}
	done := make(chan outcome, 1)
	go func() {
		_, erased, retireErr := queue.RetireDemoSchools(ctx, []string{slug})
		done <- outcome{erased: erased, err: retireErr}
	}()
	select {
	case result := <-done:
		require.NoError(t, writer.Rollback())
		t.Fatalf("the erasure did not wait for the membership being written: erased %d, %v", result.erased, result.err)
	case <-time.After(500 * time.Millisecond):
	}
	require.NoError(t, writer.Commit())
	result := <-done
	require.NoError(t, result.err)
	require.Equal(t, 1, result.erased)

	assert.Equal(t, before, stateOfAccount(t, db, accounts.caregiver),
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

func TestDemoVisitorErasureWaitsForAParallelSchoolMembership(t *testing.T) {
	t.Parallel()
	eraseWhileAMembershipIsWritten(t, func(ctx context.Context, tx bun.Tx, accountID, realID int64) error {
		_, err := tx.ExecContext(ctx, `INSERT INTO auth.account_tenants (account_id, tenant_id, status) VALUES (?, ?, 'active')`, accountID, realID)
		return err
	})
}

func TestDemoVisitorErasureWaitsForAParallelTraegerMembership(t *testing.T) {
	t.Parallel()
	eraseWhileAMembershipIsWritten(t, func(ctx context.Context, tx bun.Tx, accountID, realID int64) error {
		_, err := tx.ExecContext(ctx, `INSERT INTO platform.organization_accounts (organization_id, account_id, status, activated_at)
			SELECT organization_id, ?, 'active', NOW() FROM platform.schools WHERE id = ?`, accountID, realID)
		return err
	})
}
