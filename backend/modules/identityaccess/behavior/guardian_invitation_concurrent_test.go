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
