package behavior_test

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/api"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// Hold the first acceptance after account creation but before commit. The
// second school must reuse that account once the first transaction commits.
func TestGuardianInvitationService_Accept_ConcurrentFirstAccountAcrossSchools(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	gate := &guardianAcceptanceGate{entered: make(chan int64, 1), release: make(chan struct{})}
	defer gate.unblock()
	service, err := api.NewGuardianInvitationServiceForTests(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db), api.GuardianInvitationTestConfig{
		Outbox: api.NewCapturingOutbox(), Enrollments: gate,
	})
	require.NoError(t, err)
	_, creator := testpkg.CreateTestPersonWithAccount(t, db, "Inviter", "Concurrent")
	otherTenant, _ := testpkg.CreateTestTenant(t, db)
	tenants := []int64{testpkg.Tenant(t), otherTenant}
	firstProfile := testpkg.CreateTestGuardianProfile(t, db, "concurrent-first-accept")
	secondProfile := testpkg.CreateTestGuardianProfileWithExactEmail(t, db, otherTenant, testpkg.GuardianProfileWithEmail{
		FirstName: "Parent", LastName: "Concurrent", Email: strings.ToUpper(*firstProfile.Email),
		PreferredContactMethod: "email", LanguagePreference: "de",
	})
	profiles := []int64{firstProfile.ID, secondProfile.ID}
	tokens := make([]string, 2)
	invitationIDs := make([]int64, 2)
	for i, tenantID := range tenants {
		invitation, createErr := service.CreateGuardianInvitation(testpkg.TenantContext(tenantID), identityaccess.GuardianInvitationRequest{
			GuardianProfileID: profiles[i], CreatedBy: creator.ID,
		})
		require.NoError(t, createErr)
		tokens[i] = invitation.Token
		invitationIDs[i] = invitation.ID
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	type acceptance struct {
		account identityaccess.Account
		err     error
	}
	results := make(chan acceptance, 2)
	var workers sync.WaitGroup
	defer func() {
		cancel()
		gate.unblock()
		workers.Wait()
	}()
	accept := func(token, password string, callCtx context.Context) {
		defer workers.Done()
		account, acceptErr := service.AcceptGuardianInvitation(callCtx, token, identityaccess.GuardianRegistration{Password: password, ConfirmPassword: password})
		results <- acceptance{account: account, err: acceptErr}
	}
	workers.Add(1)
	go accept(tokens[0], strongTestPassword, ctx)
	var accountID int64
	select {
	case accountID = <-gate.entered:
	case result := <-results:
		t.Fatalf("first acceptance did not reach commit gate: %v", result.err)
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	counter := testpkg.CaptureQueriesForContext(t, db)
	workers.Add(1)
	go accept(tokens[1], testpkg.NewTestPassword(), counter.Context(ctx))
	require.Eventually(t, func() bool {
		return len(counter.Matching(func(query string) bool {
			return strings.Contains(query, "pg_advisory_xact_lock") || strings.Contains(query, "insert into auth.accounts")
		})) > 0
	}, 10*time.Second, 10*time.Millisecond, "second acceptance must reach account provisioning while the first is uncommitted")
	gate.unblock()
	for range 2 {
		select {
		case result := <-results:
			require.NoError(t, result.err)
			assert.Equal(t, accountID, result.account.ID)
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		}
	}
	testpkg.OwnTestAccount(t, db, accountID)
	stored, err := testpkg.ReadAccountState(ctx, db, accountID)
	require.NoError(t, err)
	require.NotNil(t, stored.PasswordHash)
	assert.Equal(t, gate.initialHash, *stored.PasswordHash, "the second registration must not replace the first password hash")
	for i, tenantID := range tenants {
		assert.NotNil(t, testpkg.GuardianInvitationByID(t, db, invitationIDs[i]).AcceptedAt)
		active, mappingErr := testpkg.ActiveAccountTenantExists(ctx, db, accountID, tenantID)
		require.NoError(t, mappingErr)
		assert.True(t, active)
		var linkedAccount int64
		require.NoError(t, db.NewRaw(`SELECT account_id FROM users.guardian_profiles WHERE id = ? AND tenant_id = ?`, profiles[i], tenantID).Scan(ctx, &linkedAccount))
		assert.Equal(t, accountID, linkedAccount)
	}
}

type guardianAcceptanceGate struct {
	once        sync.Once
	unlock      sync.Once
	entered     chan int64
	release     chan struct{}
	initialHash string
}

func (g *guardianAcceptanceGate) unblock() { g.unlock.Do(func() { close(g.release) }) }

func (g *guardianAcceptanceGate) BackfillGuardianAccountID(ctx context.Context, accountID int64, _ string) (int, error) {
	raw, _ := testpkg.TransactionFromContext(ctx)
	tx, ok := raw.(bun.Tx)
	if !ok {
		return 0, fmt.Errorf("acceptance transaction has type %T", raw)
	}
	state, err := testpkg.ReadAccountState(ctx, tx, accountID)
	if err != nil {
		return 0, err
	}
	if state.PasswordHash == nil {
		return 0, fmt.Errorf("accepted account has no password hash")
	}
	g.once.Do(func() {
		g.initialHash = *state.PasswordHash
		g.entered <- accountID
		select {
		case <-g.release:
		case <-ctx.Done():
		}
	})
	return 0, ctx.Err()
}

func TestGuardianInvitationService_Accept_RejectsCatererAccount(t *testing.T) {
	t.Parallel()
	env := setupGuardianInvitationTest(t)
	module := setupAuthFactory(t, env.db)
	acceptance, email := acceptedCatererAccount(t, env.db, module, testpkg.Tenant(t), "guardian-invitation")
	profile := testpkg.CreateTestGuardianProfileWithExactEmail(t, env.db, testpkg.Tenant(t), testpkg.GuardianProfileWithEmail{
		FirstName: "Parent", LastName: "Caterer", Email: email,
		PreferredContactMethod: "email", LanguagePreference: "de",
	})
	invitation, err := env.service.CreateGuardianInvitation(testpkg.Ctx(t), identityaccess.GuardianInvitationRequest{
		GuardianProfileID: profile.ID, CreatedBy: env.inviterAccountID(t),
	})
	require.NoError(t, err)
	_, err = env.service.AcceptGuardianInvitation(context.Background(), invitation.Token, identityaccess.GuardianRegistration{
		Password: strongTestPassword, ConfirmPassword: strongTestPassword,
	})
	require.ErrorIs(t, err, identityaccess.ErrEmailAlreadyExists)
	requireNoSchool(t, env.db, acceptance.AccountID)
	assert.Nil(t, testpkg.GuardianInvitationByID(t, env.db, invitation.ID).AcceptedAt)
}

// Revocation holds the existing parent's account before acceptance starts.
// Acceptance must wait there without holding the school, so revocation can
// take the school lock and commit without a deadlock (#6305).
func TestGuardianInvitationAcceptanceAndSchoolDeletionShareLockOrder(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	service, err := api.NewGuardianInvitationServiceForTests(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db), api.GuardianInvitationTestConfig{
		Outbox: api.NewCapturingOutbox(),
	})
	require.NoError(t, err)
	schoolID := testpkg.Tenant(t)
	profile := testpkg.CreateTestGuardianProfile(t, db, "invitation-school-deletion")
	account := testpkg.CreateTestAccountWithPassword(t, db, *profile.Email, strongTestPassword)
	testpkg.MapAccountToTenant(t, db, account.ID, schoolID)
	testpkg.CreateTestSessionFamily(t, db, account.ID, schoolID, "parent")
	_, creator := testpkg.CreateTestPersonWithAccount(t, db, "Inviter", "Deletion")
	invitation, err := service.CreateGuardianInvitation(testpkg.Ctx(t), identityaccess.GuardianInvitationRequest{
		GuardianProfileID: profile.ID, CreatedBy: creator.ID,
	})
	require.NoError(t, err)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	deletion, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	var workers sync.WaitGroup
	defer func() {
		_ = deletion.Rollback()
		cancel()
		workers.Wait()
	}()
	var deletionPID int
	require.NoError(t, deletion.NewRaw(`SELECT pg_backend_pid()`).Scan(ctx, &deletionPID))
	_, err = deletion.NewRaw(`SELECT id FROM auth.accounts WHERE id = ? FOR NO KEY UPDATE`, account.ID).Exec(ctx)
	require.NoError(t, err)

	accepted := make(chan error, 1)
	workers.Add(1)
	go func() {
		defer workers.Done()
		_, err := service.AcceptGuardianInvitation(ctx, invitation.Token, identityaccess.GuardianRegistration{
			Password: strongTestPassword, ConfirmPassword: strongTestPassword,
		})
		accepted <- err
	}()
	require.Eventually(t, func() bool {
		var waiting int
		err := db.NewRaw(`SELECT count(*) FROM pg_stat_activity
			WHERE datname = current_database() AND ? = ANY(pg_blocking_pids(pid))`, deletionPID).Scan(ctx, &waiting)
		return err == nil && waiting > 0
	}, 5*time.Second, 10*time.Millisecond, "acceptance waits for the account held by deletion")

	_, revokeErr := deletion.NewRaw(`SELECT count(*) FROM auth.revoke_school_sessions(?)`, schoolID).Exec(ctx)
	if revokeErr == nil {
		_, revokeErr = deletion.NewRaw(`UPDATE platform.schools SET deleted_at = NOW() WHERE id = ?`, schoolID).Exec(ctx)
	}
	if revokeErr == nil {
		revokeErr = deletion.Commit()
	} else {
		require.NoError(t, deletion.Rollback())
	}
	assert.NoError(t, revokeErr, "school deletion must not deadlock with acceptance")
	select {
	case acceptErr := <-accepted:
		assert.ErrorIs(t, acceptErr, identityaccess.ErrInvitationTenantDeleted, "acceptance sees the committed deletion")
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	assert.Zero(t, sessionsAtSchool(t, db, schoolID))
	assert.Nil(t, testpkg.GuardianInvitationByID(t, db, invitation.ID).AcceptedAt, "the refused invitation remains unspent")
}

// Provisioning now precedes the school lock. A school refusal must roll
// back a newly created account along with every invitation write.
func TestGuardianInvitationAcceptanceAtDeletedSchoolCreatesNoAccount(t *testing.T) {
	t.Parallel()
	env := setupGuardianInvitationTest(t)
	profile := testpkg.CreateTestGuardianProfile(t, env.db, "invitation-deleted-school")
	invitation, err := env.service.CreateGuardianInvitation(testpkg.Ctx(t), identityaccess.GuardianInvitationRequest{
		GuardianProfileID: profile.ID, CreatedBy: env.inviterAccountID(t),
	})
	require.NoError(t, err)
	deletion := endSchool(t, env.db, testpkg.Tenant(t))
	require.NoError(t, deletion.Commit())

	_, err = env.service.AcceptGuardianInvitation(t.Context(), invitation.Token, identityaccess.GuardianRegistration{
		Password: strongTestPassword, ConfirmPassword: strongTestPassword,
	})
	require.ErrorIs(t, err, identityaccess.ErrInvitationTenantDeleted)
	var accounts int
	require.NoError(t, env.db.NewRaw(`SELECT count(*) FROM auth.accounts WHERE LOWER(email) = LOWER(?)`, *profile.Email).Scan(t.Context(), &accounts))
	assert.Zero(t, accounts, "the new account is rolled back")
	assert.Nil(t, testpkg.GuardianInvitationByID(t, env.db, invitation.ID).AcceptedAt)
}

// Create and Accept of guardian invitations lock in one order: account,
// then school, then profile (#6305, #6294). A third transaction holds the
// guardian's account; an acceptance queues for it first, a new invitation
// for the same profile second. Once the account is free, the acceptance
// takes school and profile while the creation still waits for the account:
// a creation that held the profile while it waited would close a cycle with
// the acceptance and end one of them with a deadlock (40P01).
func TestGuardianInvitationCreateAndAcceptShareLockOrder(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	service, err := api.NewGuardianInvitationServiceForTests(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db), api.GuardianInvitationTestConfig{
		Outbox: api.NewCapturingOutbox(),
	})
	require.NoError(t, err)
	_, creator := testpkg.CreateTestPersonWithAccount(t, db, "Inviter", "CreateAccept")
	profile := testpkg.CreateTestGuardianProfile(t, db, "create-and-accept")
	account := testpkg.CreateTestAccountWithPassword(t, db, *profile.Email, strongTestPassword)
	invitation, err := service.CreateGuardianInvitation(testpkg.Ctx(t), identityaccess.GuardianInvitationRequest{
		GuardianProfileID: profile.ID, CreatedBy: creator.ID,
	})
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	holder, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	var holderPID int
	require.NoError(t, holder.NewRaw(`SELECT pg_backend_pid()`).Scan(ctx, &holderPID))
	_, err = holder.NewRaw(`SELECT id FROM auth.accounts WHERE id = ? FOR UPDATE`, account.ID).Exec(ctx)
	require.NoError(t, err)
	var workers sync.WaitGroup
	defer func() {
		_ = holder.Rollback()
		cancel()
		workers.Wait()
	}()
	waitersOfHolder := func() int {
		var waiting int
		if scanErr := db.NewRaw(`SELECT count(*) FROM pg_stat_activity
			WHERE datname = current_database() AND ? = ANY(pg_blocking_pids(pid))`, holderPID).Scan(ctx, &waiting); scanErr != nil {
			return 0
		}
		return waiting
	}

	accepted := make(chan error, 1)
	workers.Add(1)
	go func() {
		defer workers.Done()
		_, acceptErr := service.AcceptGuardianInvitation(ctx, invitation.Token, identityaccess.GuardianRegistration{
			Password: strongTestPassword, ConfirmPassword: strongTestPassword,
		})
		accepted <- acceptErr
	}()
	require.Eventually(t, func() bool { return waitersOfHolder() == 1 }, 10*time.Second, 10*time.Millisecond,
		"the acceptance queues for the account first")
	created := make(chan error, 1)
	workers.Add(1)
	go func() {
		defer workers.Done()
		_, createErr := service.CreateGuardianInvitation(testpkg.TenantContext(testpkg.Tenant(t)), identityaccess.GuardianInvitationRequest{
			GuardianProfileID: profile.ID, CreatedBy: creator.ID,
		})
		created <- createErr
	}()
	require.Eventually(t, func() bool {
		var waiting []string
		if scanErr := db.NewRaw(`SELECT left(query, 160) FROM pg_stat_activity
			WHERE datname = current_database() AND wait_event_type = 'Lock' AND pid <> ?`, holderPID).Scan(ctx, &waiting); scanErr != nil {
			return false
		}
		return len(waiting) == 2
	}, 10*time.Second, 200*time.Millisecond, "the creation queues second")
	require.NoError(t, holder.Rollback())

	for _, result := range []struct {
		name string
		ch   chan error
	}{{"acceptance", accepted}, {"creation", created}} {
		select {
		case err := <-result.ch:
			assert.NotContains(t, fmt.Sprint(err), "40P01", "%s must not deadlock", result.name)
			if result.name == "acceptance" {
				assert.NoError(t, err, "the acceptance queued first succeeds")
			}
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		}
	}
	assert.NotNil(t, testpkg.GuardianInvitationByID(t, db, invitation.ID).AcceptedAt)
}

// schoolThenProfile stands in for a transaction that holds the school
// exclusively and then locks a guardian profile. While it holds the school,
// run is started and must queue for the school without holding the profile
// (account, school, profile; #6305): the stand-in gets the profile at once,
// rolls back, and run then succeeds. A run that held the profile, or the
// child link that references it, would close a cycle (40P01).
func schoolThenProfile(t *testing.T, db *bun.DB, profileID int64, run func(context.Context) error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	holder, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	var workers sync.WaitGroup
	defer func() {
		_ = holder.Rollback()
		cancel()
		workers.Wait()
	}()
	var holderPID int
	require.NoError(t, holder.NewRaw(`SELECT pg_backend_pid()`).Scan(ctx, &holderPID))
	_, err = holder.NewRaw(`SELECT id FROM platform.schools WHERE id = ? FOR UPDATE`, testpkg.Tenant(t)).Exec(ctx)
	require.NoError(t, err)

	done := make(chan error, 1)
	workers.Add(1)
	go func() {
		defer workers.Done()
		done <- run(ctx)
	}()
	require.Eventually(t, func() bool {
		var waiting int
		err := db.NewRaw(`SELECT count(*) FROM pg_stat_activity
			WHERE datname = current_database() AND ? = ANY(pg_blocking_pids(pid))`, holderPID).Scan(ctx, &waiting)
		return err == nil && waiting > 0
	}, 10*time.Second, 10*time.Millisecond, "the invitation queues for the school the other transaction holds")

	_, err = holder.NewRaw(`SET LOCAL lock_timeout = '3s'`).Exec(ctx)
	require.NoError(t, err)
	_, err = holder.NewRaw(`SELECT id FROM users.guardian_profiles WHERE id = ? FOR UPDATE`, profileID).Exec(ctx)
	require.NoError(t, err, "the other transaction gets the profile: the invitation may not hold it while it waits for the school")
	require.NoError(t, holder.Rollback())
	select {
	case runErr := <-done:
		require.NoError(t, runErr)
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
}

func TestCreateGuardianInvitationLocksSchoolBeforeProfile(t *testing.T) {
	t.Parallel()
	env := setupGuardianInvitationTest(t)
	profile := testpkg.CreateTestGuardianProfile(t, env.db, "create-school-before-profile")
	creatorID := env.inviterAccountID(t)

	schoolThenProfile(t, env.db, profile.ID, func(ctx context.Context) error {
		_, err := env.service.CreateGuardianInvitation(testpkg.TenantContext(testpkg.Tenant(t)), identityaccess.GuardianInvitationRequest{
			GuardianProfileID: profile.ID, CreatedBy: creatorID,
		})
		return err
	})
}

// Inviting to a child writes the child link, a profile and the account's
// school mapping; each comes after the account, school and profile locks.
func TestInviteToStudentLocksSchoolBeforeProfileAndLink(t *testing.T) {
	t.Parallel()
	env := setupGuardianInvitationTest(t)
	creatorID := env.inviterAccountID(t)
	sibling := testpkg.CreateTestStudent(t, env.db, "Order", "Sibling", "2a")
	student := testpkg.CreateTestStudent(t, env.db, "Order", "Child", "2b")
	email := fmt.Sprintf("invite-school-before-profile-%d@example.test", time.Now().UnixNano())
	first, err := env.service.InviteToStudent(testpkg.Ctx(t), identityaccess.InviteToStudentRequest{
		StudentID: sibling.ID, Email: email, CreatedBy: creatorID,
	})
	require.NoError(t, err)

	schoolThenProfile(t, env.db, first.GuardianProfileID, func(ctx context.Context) error {
		_, err := env.service.InviteToStudent(testpkg.TenantContext(testpkg.Tenant(t)), identityaccess.InviteToStudentRequest{
			StudentID: student.ID, Email: email, CreatedBy: creatorID,
		})
		return err
	})
}

// An acceptance may have read the link before a contact edit or replacement
// invitation invalidated it. The final write must not spend that stale read.
func TestGuardianInvitationAcceptanceCannotOutrunInvalidation(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	service, err := api.NewGuardianInvitationServiceForTests(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db), api.GuardianInvitationTestConfig{Outbox: api.NewCapturingOutbox()})
	require.NoError(t, err)
	profile := testpkg.CreateTestGuardianProfile(t, db, "invalidation-fence")
	creator := testpkg.CreateTestAccount(t, db, "invalidation-creator")
	invitation, err := service.CreateGuardianInvitation(testpkg.Ctx(t), identityaccess.GuardianInvitationRequest{GuardianProfileID: profile.ID, CreatedBy: creator.ID})
	require.NoError(t, err)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	writer, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer func() { _ = writer.Rollback() }()
	_, err = writer.ExecContext(ctx, `SELECT id FROM users.guardian_profiles WHERE id = ? FOR UPDATE`, profile.ID)
	require.NoError(t, err)
	done := make(chan error, 1)
	go func() {
		_, err := service.AcceptGuardianInvitation(ctx, invitation.Token, identityaccess.GuardianRegistration{Password: strongTestPassword, ConfirmPassword: strongTestPassword})
		done <- err
	}()
	waitForLockWaiters(t, db, 1)
	_, err = writer.ExecContext(ctx, `UPDATE auth.guardian_invitations SET expires_at = NOW() - INTERVAL '1 minute' WHERE id = ?`, invitation.ID)
	require.NoError(t, err)
	require.NoError(t, writer.Commit())
	require.Error(t, awaitResult(t, done), "an invalidated link cannot commit an account from its earlier snapshot")
	require.Zero(t, accountCountByEmail(t, db, *profile.Email), "the provisional account rolls back too")
}
