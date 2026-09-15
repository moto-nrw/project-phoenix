package integration

import (
	"context"
	"testing"
	"time"

	"github.com/gofrs/uuid"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	identityCompose "github.com/moto-nrw/project-phoenix/modules/identityaccess/compose"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func newAccountSessionAccess(t *testing.T, db *bun.DB) identityaccess.AccountSessionAccess {
	t.Helper()
	module, err := identityCompose.New(identityCompose.Dependencies{DB: db, Observe: func(identityCompose.Observation) {}})
	require.NoError(t, err)
	return module
}

func newAccountSession(accountID int64, portalScope, familyID string, generation int) identityaccess.AccountSession {
	return identityaccess.AccountSession{
		AccountID:   accountID,
		Token:       uuid.Must(uuid.NewV4()).String(),
		Expiry:      time.Now().Add(time.Hour),
		PortalScope: portalScope,
		FamilyID:    familyID,
		Generation:  generation,
	}
}

func newFamilyID() string { return uuid.Must(uuid.NewV4()).String() }

func TestAccountSessionLifecycle(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	access := newAccountSessionAccess(t, db)
	ctx := testpkg.Ctx(t)
	account := testpkg.CreateTestAccount(t, db, "session-lifecycle")

	familyID := newFamilyID()
	created, err := access.CreateAccountSession(ctx, identityaccess.AccountSession{AccountID: account.ID, Token: uuid.Must(uuid.NewV4()).String(), Expiry: time.Now().Add(time.Hour), FamilyID: familyID})
	require.NoError(t, err)
	require.NotZero(t, created.ID)
	require.NotZero(t, created.CreatedAt)
	assert.Equal(t, testpkg.Tenant(t), created.TenantID, "a session without a tenant is pinned to the caller's tenant")
	assert.Equal(t, "unknown", created.PortalScope, "an unset portal scope is stored as unknown")

	found, err := access.FindAccountSession(ctx, created.Token)
	require.NoError(t, err)
	assert.Equal(t, created.ID, found.ID)
	assert.Equal(t, account.ID, found.AccountID)
	locked, err := access.FindAccountSessionForUpdate(ctx, created.Token)
	require.NoError(t, err)
	assert.Equal(t, created.ID, locked.ID)
	latest, err := access.LatestAccountSessionInFamily(ctx, familyID)
	require.NoError(t, err)
	assert.Equal(t, created.ID, latest.ID)

	successor, err := access.CreateAccountSession(ctx, newAccountSession(account.ID, "tenant", familyID, 1))
	require.NoError(t, err)
	latest, err = access.LatestAccountSessionInFamily(ctx, familyID)
	require.NoError(t, err)
	assert.Equal(t, successor.ID, latest.ID, "the highest generation is the family's latest session")

	_, err = access.FindAccountSession(ctx, "no-such-handle")
	require.ErrorIs(t, err, identityaccess.ErrAccountSessionNotFound)
	assert.Equal(t, "not_found", identityaccess.ErrorCode(err))
	_, err = access.FindAccountSession(ctx, "")
	require.ErrorIs(t, err, identityaccess.ErrAccountSessionNotFound)
	_, err = access.LatestAccountSessionInFamily(ctx, newFamilyID())
	require.ErrorIs(t, err, identityaccess.ErrAccountSessionNotFound)

	_, err = access.CreateAccountSession(ctx, identityaccess.AccountSession{AccountID: account.ID, Token: "", Expiry: time.Now()})
	require.EqualError(t, err, "identity access: create account session: token value is required")
	_, err = access.CreateAccountSession(ctx, identityaccess.AccountSession{AccountID: 0, Token: "handle", Expiry: time.Now()})
	require.EqualError(t, err, "identity access: create account session: account ID is required")
	_, err = access.CreateAccountSession(ctx, identityaccess.AccountSession{AccountID: account.ID, Token: "handle", Expiry: time.Now(), PortalScope: "kiosk"})
	require.EqualError(t, err, "identity access: create account session: invalid portal scope")

	mobile := true
	mobileSession := newAccountSession(account.ID, "parent", newFamilyID(), 0)
	mobileSession.Mobile = true
	_, err = access.CreateAccountSession(ctx, mobileSession)
	require.NoError(t, err)
	expired := newAccountSession(account.ID, "parent", newFamilyID(), 0)
	expired.Expiry = time.Now().Add(-time.Minute)
	_, err = access.CreateAccountSession(ctx, expired)
	require.NoError(t, err)

	all, err := access.ListAccountSessions(ctx, identityaccess.AccountSessionFilter{AccountID: account.ID})
	require.NoError(t, err)
	assert.Len(t, all, 4)
	live, err := access.ListAccountSessions(ctx, identityaccess.AccountSessionFilter{AccountID: account.ID, Liveness: identityaccess.AccountSessionsLive})
	require.NoError(t, err)
	assert.Len(t, live, 3)
	expiredOnly, err := access.ListAccountSessions(ctx, identityaccess.AccountSessionFilter{AccountID: account.ID, Liveness: identityaccess.AccountSessionsExpired})
	require.NoError(t, err)
	require.Len(t, expiredOnly, 1)
	assert.Equal(t, expired.Token, expiredOnly[0].Token)
	mobileOnly, err := access.ListAccountSessions(ctx, identityaccess.AccountSessionFilter{AccountID: account.ID, Mobile: &mobile})
	require.NoError(t, err)
	require.Len(t, mobileOnly, 1)
	assert.Equal(t, mobileSession.Token, mobileOnly[0].Token)
	family, err := access.ListAccountSessions(ctx, identityaccess.AccountSessionFilter{FamilyID: familyID})
	require.NoError(t, err)
	assert.Len(t, family, 2)

	require.NoError(t, access.DeleteAccountSession(ctx, created.ID))
	_, err = access.FindAccountSession(ctx, created.Token)
	require.ErrorIs(t, err, identityaccess.ErrAccountSessionNotFound)
	require.NoError(t, access.DeleteAccountSession(ctx, created.ID), "deleting a missing session is a no-op")
}

func TestAccountSessionRotationHandoff(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	access := newAccountSessionAccess(t, db)
	ctx := testpkg.Ctx(t)
	account := testpkg.CreateTestAccount(t, db, "session-rotation")
	familyID := newFamilyID()

	predecessor, err := access.CreateAccountSession(ctx, newAccountSession(account.ID, "tenant", familyID, 0))
	require.NoError(t, err)
	successor, err := access.CreateAccountSession(ctx, newAccountSession(account.ID, "tenant", familyID, 1))
	require.NoError(t, err)

	proof := []byte{1, 2, 3}
	rotatedAt := time.Now().Add(-10 * time.Minute)
	require.NoError(t, access.MarkAccountSessionRotated(ctx, predecessor.ID, successor.Token, proof, rotatedAt))
	err = access.MarkAccountSessionRotated(ctx, predecessor.ID, successor.Token, proof, rotatedAt)
	require.ErrorIs(t, err, identityaccess.ErrAccountSessionRotated, "a second hand-off is a replay signal, never a silent re-rotation")
	assert.Equal(t, "conflict", identityaccess.ErrorCode(err))
	err = access.MarkAccountSessionRotated(ctx, predecessor.ID+1_000_000, successor.Token, proof, rotatedAt)
	require.ErrorIs(t, err, identityaccess.ErrAccountSessionRotated)

	stored, err := access.FindAccountSession(ctx, predecessor.Token)
	require.NoError(t, err)
	require.NotNil(t, stored.RotatedAt)
	require.NotNil(t, stored.ReplacementToken)
	assert.Equal(t, successor.Token, *stored.ReplacementToken)
	assert.Equal(t, proof, stored.RecoveryProofHash)

	// A rotated predecessor stays as replay evidence until its refresh JWT
	// expires; only then does the sweep remove it, and it never touches the
	// live successor.
	require.NoError(t, access.DeleteExpiredRotatedAccountSessions(ctx, account.ID, time.Now()))
	_, err = access.FindAccountSession(ctx, predecessor.Token)
	require.NoError(t, err)
	_, err = db.NewUpdate().Table("auth.tokens").Set("expiry = ?", time.Now().Add(-time.Minute)).Where("id = ?", predecessor.ID).Exec(ctx)
	require.NoError(t, err)
	require.NoError(t, access.DeleteExpiredRotatedAccountSessions(ctx, account.ID, time.Now()))
	_, err = access.FindAccountSession(ctx, predecessor.Token)
	require.ErrorIs(t, err, identityaccess.ErrAccountSessionNotFound)
	kept, err := access.FindAccountSession(ctx, successor.Token)
	require.NoError(t, err)
	assert.Equal(t, 1, kept.Generation)

	// A retired family caps its live session and every later successor.
	retired := newFamilyID()
	live, err := access.CreateAccountSession(ctx, newAccountSession(account.ID, "tenant", retired, 0))
	require.NoError(t, err)
	cap := time.Now().Add(time.Minute).Truncate(time.Millisecond)
	require.NoError(t, access.RetireAccountSessionFamily(ctx, account.ID, retired, cap))
	capped, err := access.FindAccountSession(ctx, live.Token)
	require.NoError(t, err)
	assert.WithinDuration(t, cap, capped.Expiry, time.Millisecond)
	require.NotNil(t, capped.FamilyExpiryCap)
	assert.WithinDuration(t, cap, *capped.FamilyExpiryCap, time.Millisecond)
}

func TestAccountSessionCapKeepsPortalGroupsApart(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	access := newAccountSessionAccess(t, db)
	ctx := testpkg.Ctx(t)
	account := testpkg.CreateTestAccount(t, db, "session-cap")

	var staff []identityaccess.AccountSession
	for i := range 5 {
		scope := "tenant"
		if i%2 == 1 {
			scope = "org"
		}
		session := newAccountSession(account.ID, scope, newFamilyID(), 0)
		session.Expiry = time.Now().Add(time.Duration(i+1) * time.Hour)
		stored, err := access.CreateAccountSession(ctx, session)
		require.NoError(t, err)
		staff = append(staff, stored)
	}
	parent, err := access.CreateAccountSession(ctx, newAccountSession(account.ID, "parent", newFamilyID(), 0))
	require.NoError(t, err)
	unknown, err := access.CreateAccountSession(ctx, newAccountSession(account.ID, "", newFamilyID(), 0))
	require.NoError(t, err)
	rotated, err := access.CreateAccountSession(ctx, newAccountSession(account.ID, "tenant", newFamilyID(), 0))
	require.NoError(t, err)
	require.NoError(t, access.MarkAccountSessionRotated(ctx, rotated.ID, staff[0].Token, []byte{9}, time.Now()))
	expired := newAccountSession(account.ID, "tenant", newFamilyID(), 0)
	expired.Expiry = time.Now().Add(-time.Minute)
	_, err = access.CreateAccountSession(ctx, expired)
	require.NoError(t, err)

	evicted, err := access.EnforceAccountSessionCap(ctx, account.ID, "tenant", 5)
	require.NoError(t, err)
	assert.Empty(t, evicted, "rotated, expired, parent and unknown rows do not count against the staff cap")

	newest, err := access.CreateAccountSession(ctx, newAccountSession(account.ID, "tenant", newFamilyID(), 0))
	require.NoError(t, err)
	evicted, err = access.EnforceAccountSessionCap(ctx, account.ID, "org", 5)
	require.NoError(t, err)
	require.Len(t, evicted, 1, "tenant and org share the staff allowance")
	assert.Equal(t, staff[0].ID, evicted[0].ID, "the session closest to expiry is evicted first")
	for _, token := range []string{parent.Token, unknown.Token, rotated.Token, expired.Token, newest.Token} {
		_, err = access.FindAccountSession(ctx, token)
		require.NoError(t, err, "the cap must leave other portals, hand-offs, expired rows and the newest session alone")
	}

	evicted, err = access.EnforceAccountSessionCap(ctx, account.ID, "parent", 0)
	require.NoError(t, err)
	require.Len(t, evicted, 1)
	assert.Equal(t, parent.ID, evicted[0].ID)
	_, err = access.FindAccountSession(ctx, unknown.Token)
	require.NoError(t, err, "unknown legacy rows stay outside every known portal cap")

	// A family retired by a tenant switch sorts last and is evicted before a
	// session on another device (#2952).
	require.NoError(t, access.RetireAccountSessionFamily(ctx, account.ID, staff[4].FamilyID, time.Now().Add(time.Second)))
	evicted, err = access.EnforceAccountSessionCap(ctx, account.ID, "tenant", 4)
	require.NoError(t, err)
	require.Len(t, evicted, 1)
	assert.Equal(t, staff[4].ID, evicted[0].ID)
}

func TestAccountSessionCapInAdminTransactionSpansSchools(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	access := newAccountSessionAccess(t, db)
	ctx := testpkg.WithTenantRuntime(t, testpkg.Ctx(t), db)
	account := testpkg.CreateTestAccount(t, db, "session-cap-admin")
	otherTenant, _ := testpkg.CreateTestTenant(t, db)
	testpkg.MapAccountToTenant(t, db, account.ID, otherTenant)

	var first []identityaccess.AccountSession
	for i := range 5 {
		session := newAccountSession(account.ID, "tenant", newFamilyID(), 0)
		session.Expiry = time.Now().Add(time.Duration(i+1) * time.Hour)
		stored, err := access.CreateAccountSession(ctx, session)
		require.NoError(t, err)
		first = append(first, stored)
	}
	other := newAccountSession(account.ID, "tenant", newFamilyID(), 0)
	other.TenantID = otherTenant
	other.Expiry = time.Now().Add(24 * time.Hour)
	_, err := access.CreateAccountSession(ctx, other)
	require.NoError(t, err)

	evicted, err := access.EnforceAccountSessionCap(testpkg.TenantContext(otherTenant), account.ID, "tenant", 5)
	require.NoError(t, err)
	assert.Empty(t, evicted, "a tenant-scoped caller counts only its own school's sessions")

	var adminEvicted []identityaccess.AccountSession
	require.NoError(t, testpkg.WithAdminTx(t, ctx, db, func(txCtx context.Context, _ bun.Tx) error {
		var err error
		adminEvicted, err = access.EnforceAccountSessionCap(txCtx, account.ID, "tenant", 5)
		return err
	}))
	require.Len(t, adminEvicted, 1, "the administrative login transaction caps across every school of the account")
	assert.Equal(t, first[0].ID, adminEvicted[0].ID)
}

func TestAccountSessionRevocations(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	access := newAccountSessionAccess(t, db)
	ctx := testpkg.WithTenantRuntime(t, testpkg.Ctx(t), db)
	account := testpkg.CreateTestAccount(t, db, "session-revocations")
	otherTenant, _ := testpkg.CreateTestTenant(t, db)
	testpkg.MapAccountToTenant(t, db, account.ID, otherTenant)

	familyA := newFamilyID()
	for generation := range 2 {
		_, err := access.CreateAccountSession(ctx, newAccountSession(account.ID, "tenant", familyA, generation))
		require.NoError(t, err)
	}
	local, err := access.CreateAccountSession(ctx, newAccountSession(account.ID, "tenant", newFamilyID(), 0))
	require.NoError(t, err)
	foreign := newAccountSession(account.ID, "tenant", newFamilyID(), 0)
	foreign.TenantID = otherTenant
	foreign, err = access.CreateAccountSession(ctx, foreign)
	require.NoError(t, err)

	family, err := access.RevokeAccountSessionFamily(ctx, familyA)
	require.NoError(t, err)
	assert.Len(t, family, 2)
	_, err = access.LatestAccountSessionInFamily(ctx, familyA)
	require.ErrorIs(t, err, identityaccess.ErrAccountSessionNotFound)

	_, err = access.RevokeAccountSessionsInTenant(testpkg.WithTenantRuntime(t, context.Background(), db), account.ID)
	require.ErrorIs(t, err, identityaccess.ErrTenantRequired, "a tenant-scoped revocation without a tenant is refused, not widened")
	assert.Equal(t, "tenant_required", identityaccess.ErrorCode(err))

	// An administrative transaction re-scoped to one school still revokes
	// only that school's sessions.
	var inTenant []identityaccess.AccountSession
	require.NoError(t, testpkg.WithAdminTx(t, ctx, db, func(txCtx context.Context, _ bun.Tx) error {
		var err error
		inTenant, err = access.RevokeAccountSessionsInTenant(testpkg.ContextForTenant(txCtx, testpkg.Tenant(t)), account.ID)
		return err
	}))
	require.Len(t, inTenant, 1)
	assert.Equal(t, local.ID, inTenant[0].ID)
	_, err = access.FindAccountSession(testpkg.TenantContext(otherTenant), foreign.Token)
	require.NoError(t, err, "the other school's session survives a tenant-scoped revocation")

	all, err := access.RevokeAllAccountSessions(ctx, account.ID)
	require.NoError(t, err)
	require.Len(t, all, 1, "the account-wide wipe ignores the caller's tenant")
	assert.Equal(t, foreign.ID, all[0].ID)

	// Sessions that existed at the cutoff go, including their later refresh
	// successors and legacy rows without a family; a login after the cutoff
	// keeps its new family.
	preexisting := newFamilyID()
	newFamily := newFamilyID()
	cutoff := time.Now().Add(-time.Minute)
	_, err = db.NewRaw(`
		INSERT INTO auth.tokens (account_id, token, expiry, tenant_id, portal_scope, family_id, generation, created_at)
		VALUES (?, ?, ?, ?, 'tenant', ?, 0, ?), (?, ?, ?, ?, 'tenant', ?, 1, ?), (?, ?, ?, ?, 'tenant', '', 0, ?), (?, ?, ?, ?, 'tenant', ?, 0, ?)`,
		account.ID, uuid.Must(uuid.NewV4()).String(), time.Now().Add(time.Hour), testpkg.Tenant(t), preexisting, cutoff.Add(-time.Minute),
		account.ID, uuid.Must(uuid.NewV4()).String(), time.Now().Add(time.Hour), testpkg.Tenant(t), preexisting, cutoff.Add(time.Minute),
		account.ID, uuid.Must(uuid.NewV4()).String(), time.Now().Add(time.Hour), testpkg.Tenant(t), cutoff.Add(time.Minute),
		account.ID, uuid.Must(uuid.NewV4()).String(), time.Now().Add(time.Hour), testpkg.Tenant(t), newFamily, cutoff.Add(time.Minute),
	).Exec(ctx)
	require.NoError(t, err)
	older, err := access.RevokeAccountSessionsCreatedAtOrBefore(ctx, account.ID, cutoff)
	require.NoError(t, err)
	assert.Len(t, older, 3)
	remaining, err := access.ListAccountSessions(ctx, identityaccess.AccountSessionFilter{AccountID: account.ID})
	require.NoError(t, err)
	require.Len(t, remaining, 1)
	assert.Equal(t, newFamily, remaining[0].FamilyID)

	byTenant, err := access.RevokeTenantAccountSessions(ctx, testpkg.Tenant(t))
	require.NoError(t, err)
	require.Len(t, byTenant, 1)
	_, err = access.RevokeTenantAccountSessions(ctx, 0)
	require.ErrorIs(t, err, identityaccess.ErrTenantRequired)
}

func TestAccountSessionExpirySweepAndReconciliation(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	access := newAccountSessionAccess(t, db)
	ctx := testpkg.Ctx(t)
	account := testpkg.CreateTestAccount(t, db, "session-expiry")
	inactive := testpkg.CreateTestAccount(t, db, "session-expiry-inactive")
	_, err := db.NewUpdate().Table("auth.accounts").Set("active = false").Where("id = ?", inactive.ID).Exec(ctx)
	require.NoError(t, err)

	before, err := access.CountExpiredAccountSessions(ctx)
	require.NoError(t, err)
	expired := newAccountSession(account.ID, "tenant", newFamilyID(), 0)
	expired.Expiry = time.Now().Add(-time.Hour)
	_, err = access.CreateAccountSession(ctx, expired)
	require.NoError(t, err)
	live, err := access.CreateAccountSession(ctx, newAccountSession(account.ID, "tenant", newFamilyID(), 0))
	require.NoError(t, err)
	orphan, err := access.CreateAccountSession(ctx, newAccountSession(inactive.ID, "tenant", newFamilyID(), 0))
	require.NoError(t, err)

	has, err := access.HasLiveAccountSessionsCreatedAfter(ctx, account.ID, time.Now().Add(-time.Hour))
	require.NoError(t, err)
	assert.True(t, has)
	has, err = access.HasLiveAccountSessionsCreatedAfter(ctx, account.ID, time.Now().Add(time.Hour))
	require.NoError(t, err)
	assert.False(t, has)

	inactiveIDs, err := access.ListInactiveAccountIDsWithLiveSessions(ctx)
	require.NoError(t, err)
	assert.Contains(t, inactiveIDs, inactive.ID, "a deactivated account with a live session is reported for the wipe")
	assert.NotContains(t, inactiveIDs, account.ID)

	count, err := access.CountExpiredAccountSessions(ctx)
	require.NoError(t, err)
	assert.Equal(t, before+1, count, "only the expired session is counted")
	deleted, err := access.DeleteExpiredAccountSessions(ctx)
	require.NoError(t, err)
	assert.Equal(t, count, deleted, "the preview matches the sweep")
	_, err = access.FindAccountSession(ctx, expired.Token)
	require.ErrorIs(t, err, identityaccess.ErrAccountSessionNotFound)
	for _, token := range []string{live.Token, orphan.Token} {
		_, err = access.FindAccountSession(ctx, token)
		require.NoError(t, err, "live sessions survive the expiry sweep")
	}
}

// Account sessions are tenant rows. A caller scoped to one school never sees
// or revokes another school's sessions; the tenantless pre-authentication
// flows (login, refresh, logout) see every school, which is what lets them
// resolve the session's school from the row itself.
func TestAccountSessionsAreScopedToTheCallerTenant(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	access := newAccountSessionAccess(t, db)
	account := testpkg.CreateTestAccount(t, db, "session-isolation")
	first := testpkg.Tenant(t)
	second, _ := testpkg.CreateTestTenant(t, db)
	testpkg.MapAccountToTenant(t, db, account.ID, second)

	ownFamily, foreignFamily := newFamilyID(), newFamilyID()
	own, err := access.CreateAccountSession(testpkg.TenantContext(first), newAccountSession(account.ID, "tenant", ownFamily, 0))
	require.NoError(t, err)
	foreign, err := access.CreateAccountSession(testpkg.TenantContext(second), newAccountSession(account.ID, "tenant", foreignFamily, 0))
	require.NoError(t, err)
	assert.Equal(t, second, foreign.TenantID)

	ownCtx := testpkg.TenantContext(first)
	_, err = access.FindAccountSession(ownCtx, foreign.Token)
	require.ErrorIs(t, err, identityaccess.ErrAccountSessionNotFound, "another school's session is invisible")
	_, err = access.FindAccountSessionForUpdate(ownCtx, foreign.Token)
	require.ErrorIs(t, err, identityaccess.ErrAccountSessionNotFound)
	_, err = access.LatestAccountSessionInFamily(ownCtx, foreignFamily)
	require.ErrorIs(t, err, identityaccess.ErrAccountSessionNotFound)
	listed, err := access.ListAccountSessions(ownCtx, identityaccess.AccountSessionFilter{AccountID: account.ID})
	require.NoError(t, err)
	require.Len(t, listed, 1)
	assert.Equal(t, own.ID, listed[0].ID)
	err = access.MarkAccountSessionRotated(ownCtx, foreign.ID, own.Token, []byte{1}, time.Now())
	require.ErrorIs(t, err, identityaccess.ErrAccountSessionRotated, "another school's session cannot be rotated")
	revoked, err := access.RevokeAccountSessionFamily(ownCtx, foreignFamily)
	require.NoError(t, err)
	assert.Empty(t, revoked, "another school's family cannot be revoked")
	require.NoError(t, access.DeleteAccountSession(ownCtx, foreign.ID))
	_, err = access.FindAccountSession(testpkg.TenantContext(second), foreign.Token)
	require.NoError(t, err, "another school's session cannot be deleted")

	tenantless := testpkg.WithTenantRuntime(t, context.Background(), db)
	for _, token := range []string{own.Token, foreign.Token} {
		_, err = access.FindAccountSession(tenantless, token)
		require.NoError(t, err, "the pre-authentication flows resolve every school's session")
	}
	both, err := access.ListAccountSessions(tenantless, identityaccess.AccountSessionFilter{AccountID: account.ID})
	require.NoError(t, err)
	assert.Len(t, both, 2)
}

// loginState carries the sessions a login or tenant switch works on between
// its writes.
type loginState struct {
	retired identityaccess.AccountSession
	minted  identityaccess.AccountSession
	evicted []identityaccess.AccountSession
}

// A login or tenant switch performs its authoritative session writes in one
// administrative transaction: the retirement of the presented family (switch),
// the mint of the new session and the cap eviction. A failure after any one
// of them must leave every session as it was, and the retry must then succeed
// from that clean state.
func TestAccountLoginMintRollsBackAfterEachWrite(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	access := newAccountSessionAccess(t, db)
	ctx := testpkg.WithTenantRuntime(t, testpkg.Ctx(t), db)
	tenantID := testpkg.Tenant(t)

	writes := []struct {
		name string
		run  func(context.Context, int64, *loginState) error
	}{
		{"family retirement", func(txCtx context.Context, accountID int64, state *loginState) error {
			return access.RetireAccountSessionFamily(txCtx, accountID, state.retired.FamilyID, time.Now().Add(time.Minute))
		}},
		{"session mint", func(txCtx context.Context, accountID int64, state *loginState) error {
			session := newAccountSession(accountID, "tenant", newFamilyID(), 0)
			session.TenantID = tenantID
			minted, err := access.CreateAccountSession(txCtx, session)
			state.minted = minted
			return err
		}},
		{"cap eviction", func(txCtx context.Context, accountID int64, state *loginState) error {
			evicted, err := access.EnforceAccountSessionCap(txCtx, accountID, "tenant", 5)
			state.evicted = evicted
			return err
		}},
	}

	for injectAfter, write := range writes {
		t.Run("fails after the "+write.name, func(t *testing.T) {
			account := testpkg.CreateTestAccount(t, db, "login-rollback")
			var sessions []identityaccess.AccountSession
			for i := range 5 {
				session := newAccountSession(account.ID, "tenant", newFamilyID(), 0)
				session.Expiry = time.Now().Add(time.Duration(i+2) * time.Hour)
				stored, err := access.CreateAccountSession(testpkg.Ctx(t), session)
				require.NoError(t, err)
				sessions = append(sessions, stored)
			}
			state := &loginState{retired: sessions[4]}

			err := testpkg.WithAdminTx(t, ctx, db, func(txCtx context.Context, _ bun.Tx) error {
				for step := 0; step <= injectAfter; step++ {
					if err := writes[step].run(txCtx, account.ID, state); err != nil {
						return err
					}
				}
				return errInjected
			})
			require.ErrorIs(t, err, errInjected)

			retired, err := access.FindAccountSession(testpkg.Ctx(t), state.retired.Token)
			require.NoError(t, err)
			assert.Nil(t, retired.FamilyExpiryCap, "the rolled-back retirement must not cap the family")
			assert.WithinDuration(t, state.retired.Expiry, retired.Expiry, time.Millisecond)
			if state.minted.Token != "" {
				_, err = access.FindAccountSession(testpkg.Ctx(t), state.minted.Token)
				require.ErrorIs(t, err, identityaccess.ErrAccountSessionNotFound, "the rolled-back mint must not exist")
			}
			live, err := access.ListAccountSessions(testpkg.Ctx(t), identityaccess.AccountSessionFilter{AccountID: account.ID, Liveness: identityaccess.AccountSessionsLive})
			require.NoError(t, err)
			assert.Len(t, live, 5, "the rolled-back cap eviction must leave every session in place")

			require.NoError(t, testpkg.WithAdminTx(t, ctx, db, func(txCtx context.Context, _ bun.Tx) error {
				for _, retry := range writes {
					if err := retry.run(txCtx, account.ID, state); err != nil {
						return err
					}
				}
				return nil
			}), "the retry must succeed from the clean state")
			_, err = access.FindAccountSession(testpkg.Ctx(t), state.minted.Token)
			require.NoError(t, err, "the retry mints the session")
			require.Len(t, state.evicted, 1, "the retry enforces the cap")
			assert.Equal(t, state.retired.ID, state.evicted[0].ID, "the retired family is the cap's first candidate")
		})
	}
}

// A refresh performs three authoritative session writes in one administrative
// transaction: the successor insert, the rotation hand-off and the
// expired-predecessor sweep. Injecting after each write in turn proves no
// write escapes the transaction and that the retry succeeds afterwards.
func TestAccountRefreshRollsBackAfterEachWrite(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	access := newAccountSessionAccess(t, db)
	ctx := testpkg.WithTenantRuntime(t, testpkg.Ctx(t), db)
	tenantID := testpkg.Tenant(t)

	writes := []struct {
		name string
		run  func(context.Context, int64, *refreshSessions) error
	}{
		{"successor insert", func(txCtx context.Context, accountID int64, state *refreshSessions) error {
			successor := newAccountSession(accountID, "tenant", state.predecessor.FamilyID, state.predecessor.Generation+1)
			successor.TenantID = tenantID
			stored, err := access.CreateAccountSession(txCtx, successor)
			state.successor = stored
			return err
		}},
		{"rotation hand-off", func(txCtx context.Context, _ int64, state *refreshSessions) error {
			return access.MarkAccountSessionRotated(txCtx, state.predecessor.ID, state.successor.Token, []byte{7}, time.Now())
		}},
		{"expired-predecessor sweep", func(txCtx context.Context, accountID int64, _ *refreshSessions) error {
			return access.DeleteExpiredRotatedAccountSessions(txCtx, accountID, time.Now())
		}},
	}

	for injectAfter, write := range writes {
		t.Run("fails after the "+write.name, func(t *testing.T) {
			account := testpkg.CreateTestAccount(t, db, "refresh-rollback")
			predecessor, err := access.CreateAccountSession(testpkg.Ctx(t), newAccountSession(account.ID, "tenant", newFamilyID(), 0))
			require.NoError(t, err)
			state := &refreshSessions{predecessor: predecessor}

			err = testpkg.WithAdminTx(t, ctx, db, func(txCtx context.Context, _ bun.Tx) error {
				for step := 0; step <= injectAfter; step++ {
					if err := writes[step].run(txCtx, account.ID, state); err != nil {
						return err
					}
				}
				return errInjected
			})
			require.ErrorIs(t, err, errInjected)

			stored, err := access.FindAccountSession(testpkg.Ctx(t), predecessor.Token)
			require.NoError(t, err, "the predecessor must survive the rollback")
			assert.Nil(t, stored.RotatedAt, "the rolled-back hand-off must not be recorded")
			assert.Nil(t, stored.ReplacementToken)
			if state.successor.Token != "" {
				_, err = access.FindAccountSession(testpkg.Ctx(t), state.successor.Token)
				require.ErrorIs(t, err, identityaccess.ErrAccountSessionNotFound, "the rolled-back successor must not exist")
			}
			latest, err := access.LatestAccountSessionInFamily(testpkg.Ctx(t), predecessor.FamilyID)
			require.NoError(t, err)
			assert.Equal(t, predecessor.ID, latest.ID, "the family must still end at the predecessor")

			require.NoError(t, testpkg.WithAdminTx(t, ctx, db, func(txCtx context.Context, _ bun.Tx) error {
				for _, retry := range writes {
					if err := retry.run(txCtx, account.ID, state); err != nil {
						return err
					}
				}
				return nil
			}), "the retry must succeed from the clean state")
			rotated, err := access.FindAccountSession(testpkg.Ctx(t), predecessor.Token)
			require.NoError(t, err)
			require.NotNil(t, rotated.RotatedAt, "the retry records the hand-off")
			latest, err = access.LatestAccountSessionInFamily(testpkg.Ctx(t), predecessor.FamilyID)
			require.NoError(t, err)
			assert.Equal(t, state.successor.ID, latest.ID, "the retry hands the family to the successor")
		})
	}
}

// refreshSessions carries the two sessions one refresh works on between its
// writes.
type refreshSessions struct {
	predecessor identityaccess.AccountSession
	successor   identityaccess.AccountSession
}

// Revocation and its audit evidence commit together or not at all: a failure
// after the family delete must leave every session in place, and a repeated
// revoke is a no-op rather than an error.
func TestAccountRevocationRollsBackWithItsCaller(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	access := newAccountSessionAccess(t, db)
	account := testpkg.CreateTestAccount(t, db, "revoke-rollback")
	ctx := testpkg.WithTenantRuntime(t, testpkg.Ctx(t), db)
	familyID := newFamilyID()
	for generation := range 2 {
		_, err := access.CreateAccountSession(testpkg.Ctx(t), newAccountSession(account.ID, "tenant", familyID, generation))
		require.NoError(t, err)
	}

	err := testpkg.WithAdminTx(t, ctx, db, func(txCtx context.Context, _ bun.Tx) error {
		deleted, err := access.RevokeAccountSessionFamily(txCtx, familyID)
		if err != nil {
			return err
		}
		require.Len(t, deleted, 2)
		return errInjected
	})
	require.ErrorIs(t, err, errInjected)

	latest, err := access.LatestAccountSessionInFamily(testpkg.Ctx(t), familyID)
	require.NoError(t, err, "the rolled-back revocation must leave the family intact")
	assert.Equal(t, 1, latest.Generation)

	require.NoError(t, testpkg.WithAdminTx(t, ctx, db, func(txCtx context.Context, _ bun.Tx) error {
		_, err := access.RevokeAllAccountSessions(txCtx, account.ID)
		return err
	}), "the retry revokes from the clean state")
	_, err = access.LatestAccountSessionInFamily(testpkg.Ctx(t), familyID)
	require.ErrorIs(t, err, identityaccess.ErrAccountSessionNotFound)

	require.NoError(t, testpkg.WithAdminTx(t, ctx, db, func(txCtx context.Context, _ bun.Tx) error {
		deleted, err := access.RevokeAllAccountSessions(txCtx, account.ID)
		assert.Empty(t, deleted, "an idempotent retry deletes nothing")
		return err
	}))
}

func TestAccountSessionObservationsCountRows(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	account := testpkg.CreateTestAccount(t, db, "session-observations")
	var observations []identityCompose.Observation
	access, err := identityCompose.New(identityCompose.Dependencies{DB: db, Observe: func(observation identityCompose.Observation) {
		observations = append(observations, observation)
	}})
	require.NoError(t, err)
	ctx := testpkg.Ctx(t)

	session, err := access.CreateAccountSession(ctx, newAccountSession(account.ID, "tenant", newFamilyID(), 0))
	require.NoError(t, err)
	_, err = access.FindAccountSession(ctx, session.Token)
	require.NoError(t, err)
	_, err = access.RevokeAccountSessionFamily(ctx, session.FamilyID)
	require.NoError(t, err)
	_, err = access.FindAccountSession(ctx, session.Token)
	require.ErrorIs(t, err, identityaccess.ErrAccountSessionNotFound)

	require.Len(t, observations, 4)
	assert.Equal(t, "create_account_session", observations[0].Operation)
	assert.EqualValues(t, 1, observations[0].Stats.Rows)
	assert.Equal(t, "find_account_session", observations[1].Operation)
	assert.Zero(t, observations[1].Stats.Rows)
	assert.Equal(t, "revoke_account_session_family", observations[2].Operation)
	assert.EqualValues(t, 1, observations[2].Stats.Rows)
	assert.Equal(t, "find_account_session", observations[3].Operation)
	require.ErrorIs(t, observations[3].Err, identityaccess.ErrAccountSessionNotFound)
	assert.Equal(t, "not_found", identityaccess.ErrorCode(observations[3].Err))
	for _, observation := range observations {
		assert.EqualValues(t, 1, observation.Stats.Queries, observation.Operation)
	}
}
