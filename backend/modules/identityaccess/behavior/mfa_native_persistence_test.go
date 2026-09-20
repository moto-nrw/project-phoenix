package behavior_test

import (
	"context"
	"errors"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/services"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func nativeMFARecords(t *testing.T, db *bun.DB) services.AccountMFARecords {
	t.Helper()
	var records services.AccountMFARecords
	newMFATestModule(t, db, services.WithAuthTestMFARecords(func(native services.AccountMFARecords) services.AccountMFARecords {
		records = native
		return native
	}))
	require.NotNil(t, records)
	return records
}

func TestNativeMFAAtomicTransitions(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	records := nativeMFARecords(t, db)
	ctx := testpkg.Ctx(t)
	account := testpkg.CreateTestAccount(t, db, "native-mfa-atomic")
	challenge, err := records.CreateChallenge(ctx, identityaccess.AccountMFAChallenge{
		AccountID: account.ID, TenantID: testpkg.Tenant(t), Scope: "tenant",
		CodeHash: "opaque-hash", ExpiresAt: time.Now().Add(time.Hour),
	})
	require.NoError(t, err)
	const workers = 12
	outcomes := make(chan error, workers)
	var group sync.WaitGroup
	for range workers {
		group.Go(func() { outcomes <- records.ConsumeChallenge(ctx, challenge.ID, time.Now()) })
	}
	group.Wait()
	close(outcomes)
	successes := 0
	for err := range outcomes {
		if err == nil {
			successes++
		}
	}
	require.Equal(t, 1, successes, "only one verifier may consume the challenge")
	increments := make(chan error, workers)
	for range workers {
		group.Go(func() {
			_, incrementErr := records.IncrementMFAAttempts(ctx, account.ID, 5, time.Minute)
			increments <- incrementErr
		})
	}
	group.Wait()
	close(increments)
	for err := range increments {
		require.NoError(t, err)
	}
	identity, found, err := records.FindAccountIdentity(ctx, account.ID)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, workers, identity.MFAAttempts)
	require.NotNil(t, identity.MFALockedUntil)
	require.True(t, identity.MFALockedUntil.After(time.Now()))
	require.NoError(t, records.ResetMFAAttempts(ctx, account.ID))
	identity, found, err = records.FindAccountIdentity(ctx, account.ID)
	require.NoError(t, err)
	require.True(t, found)
	require.Zero(t, identity.MFAAttempts)
	require.Nil(t, identity.MFALockedUntil)
}

func TestNativeMFAPersistenceJoinsAmbientTransaction(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	records := nativeMFARecords(t, db)
	ctx := testpkg.Ctx(t)
	account := testpkg.CreateTestAccount(t, db, "native-mfa-rollback")
	rollback := errors.New("rollback mfa enrollment")
	err := tenant.WithAdminTx(ctx, db, func(txCtx context.Context, _ bun.Tx) error {
		require.NoError(t, records.CreateCredential(txCtx, identityaccess.AccountMFACredential{AccountID: account.ID, Method: "email"}))
		credential, found, readErr := records.FindCredential(txCtx, account.ID)
		require.NoError(t, readErr)
		require.True(t, found)
		require.False(t, credential.EnrolledAt.IsZero())
		require.False(t, credential.CreatedAt.IsZero())
		_, writeErr := records.IncrementMFAAttempts(txCtx, account.ID, 5, time.Minute)
		require.NoError(t, writeErr)
		return rollback
	})
	require.ErrorIs(t, err, rollback)
	_, found, err := records.FindCredential(ctx, account.ID)
	require.NoError(t, err)
	require.False(t, found)
	identity, found, err := records.FindAccountIdentity(ctx, account.ID)
	require.NoError(t, err)
	require.True(t, found)
	require.Zero(t, identity.MFAAttempts)
}

func TestNativeMFAChallengeAndDeviceScope(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	records := nativeMFARecords(t, db)
	ctx := testpkg.Ctx(t)
	home := testpkg.Tenant(t)
	other, _ := testpkg.CreateTestTenant(t, db)
	account := testpkg.CreateTestAccount(t, db, "native-mfa-scope")
	peer := testpkg.CreateTestAccount(t, db, "native-mfa-peer")
	past := time.Now().Add(-time.Minute).Truncate(time.Microsecond)
	pending := time.Now()
	challenge, err := records.CreateChallenge(ctx, identityaccess.AccountMFAChallenge{
		AccountID: account.ID, TenantID: home, Scope: "tenant", CodeHash: "opaque-hash",
		ExpiresAt: time.Now().Add(time.Hour), ConsumedAt: &pending,
		CreatedAt: past, IPAddress: net.ParseIP("127.0.0.1"),
	})
	require.NoError(t, err)
	require.True(t, past.Equal(challenge.CreatedAt))
	require.True(t, challenge.IPAddress.Equal(net.ParseIP("127.0.0.1")))
	_, found, _ := records.FindActiveChallengeForAccount(ctx, challenge.ID, account.ID)
	require.False(t, found, "undelivered codes must not be redeemable")
	require.NoError(t, records.ActivateChallenge(ctx, challenge.ID))
	require.Error(t, records.ActivateChallenge(ctx, challenge.ID))
	for _, scope := range []struct {
		account, school int64
		portal          string
	}{
		{peer.ID, home, "tenant"}, {account.ID, other, "tenant"}, {account.ID, home, "school"},
	} {
		_, found, _ := records.FindActiveChallengeInScope(ctx, scope.account, scope.school, scope.portal)
		require.False(t, found)
	}
	active, found, err := records.FindActiveChallengeInScope(ctx, account.ID, home, "tenant")
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, challenge.ID, active.ID)
	count, err := records.CountChallengesSince(ctx, account.ID, past.Add(-time.Second))
	require.NoError(t, err)
	require.Equal(t, 1, count)
	device, err := records.CreateTrustedDevice(ctx, identityaccess.AccountTrustedDevice{
		AccountID: account.ID, TenantID: home, TokenHash: "opaque-token-hash", ExpiresAt: time.Now().Add(time.Hour),
	})
	require.NoError(t, err)
	_, found, _ = records.FindActiveTrustedDevice(ctx, account.ID, other, device.TokenHash)
	require.False(t, found)
	require.NoError(t, records.RevokeTenantTrustedDevices(ctx, account.ID, other, time.Now()))
	_, found, err = records.FindActiveTrustedDevice(ctx, account.ID, home, device.TokenHash)
	require.NoError(t, err)
	require.True(t, found)
	require.NoError(t, records.RevokeTrustedDevice(ctx, device.ID, time.Now()))
	require.Error(t, records.RevokeTrustedDevice(ctx, device.ID, time.Now()))
	devices, err := records.ListActiveTrustedDevices(ctx, account.ID, home)
	require.NoError(t, err)
	require.Empty(t, devices)
}
