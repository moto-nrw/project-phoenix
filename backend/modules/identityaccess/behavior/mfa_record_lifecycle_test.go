package behavior_test

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

// The live credential operations replace the retired generic Update/List API.
func TestNativeMFACredentialLifecycle(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	records := nativeMFARecords(t, db)
	ctx := testpkg.Ctx(t)
	account := testpkg.CreateTestAccount(t, db, "mfa-credential-lifecycle")
	peer := testpkg.CreateTestAccount(t, db, "mfa-credential-peer")
	require.EqualError(t, records.CreateCredential(ctx, identityaccess.AccountMFACredential{Method: "email"}), "account_id is required")
	require.EqualError(t, records.CreateCredential(ctx, identityaccess.AccountMFACredential{AccountID: account.ID, Method: "totp"}), "unsupported MFA method")
	require.NoError(t, records.CreateCredential(ctx, identityaccess.AccountMFACredential{AccountID: account.ID, Method: "email"}))
	credential, found, err := records.FindCredential(ctx, account.ID)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, account.ID, credential.AccountID)
	require.Equal(t, "email", credential.Method)
	require.Positive(t, credential.ID)
	_, found, err = records.FindCredential(ctx, peer.ID)
	require.NoError(t, err)
	require.False(t, found)
	stamp := time.Now().Truncate(time.Microsecond)
	require.NoError(t, records.TouchCredential(ctx, credential.ID, stamp))
	touched, found, err := records.FindCredential(ctx, account.ID)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, credential.ID, touched.ID)
	require.NotNil(t, touched.LastUsedAt)
	require.True(t, stamp.Equal(*touched.LastUsedAt))
	require.True(t, touched.UpdatedAt.After(credential.UpdatedAt), "the database trigger timestamps credential updates")
	require.True(t, credential.CreatedAt.Equal(touched.CreatedAt))
	require.True(t, credential.EnrolledAt.Equal(touched.EnrolledAt))
	require.NoError(t, records.DeleteCredentials(ctx, account.ID))
	require.NoError(t, records.DeleteCredentials(ctx, account.ID))
	_, found, err = records.FindCredential(ctx, account.ID)
	require.NoError(t, err)
	require.False(t, found)
}

func TestNativeMFATrustedDeviceLifecycle(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	records := nativeMFARecords(t, db)
	ctx := testpkg.Ctx(t)
	account := testpkg.CreateTestAccount(t, db, "mfa-device-lifecycle")
	home := testpkg.Tenant(t)
	other, _ := testpkg.CreateTestTenant(t, db)
	first, err := records.CreateTrustedDevice(ctx, identityaccess.AccountTrustedDevice{
		AccountID: account.ID, TenantID: home, TokenHash: "first", ExpiresAt: time.Now().Add(time.Hour),
	})
	require.NoError(t, err)
	second, err := records.CreateTrustedDevice(ctx, identityaccess.AccountTrustedDevice{
		AccountID: account.ID, TenantID: other, TokenHash: "second", ExpiresAt: time.Now().Add(time.Hour),
	})
	require.NoError(t, err)
	stamp := time.Now().Truncate(time.Microsecond)
	require.NoError(t, records.TouchTrustedDevice(ctx, first.ID, stamp))
	active, found, err := records.FindActiveTrustedDevice(ctx, account.ID, home, first.TokenHash)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, first.ID, active.ID)
	require.NotNil(t, active.LastUsedAt)
	require.True(t, stamp.Equal(*active.LastUsedAt))
	require.NoError(t, records.RevokeAllTrustedDevices(ctx, account.ID, time.Now()))
	for _, device := range []identityaccess.AccountTrustedDevice{first, second} {
		_, found, _ := records.FindActiveTrustedDevice(ctx, account.ID, device.TenantID, device.TokenHash)
		require.False(t, found, "account-wide revocation must reach every school")
	}
	_, err = records.CreateTrustedDevice(ctx, identityaccess.AccountTrustedDevice{
		AccountID: account.ID, TenantID: home, TokenHash: "expired", ExpiresAt: time.Now().Add(-time.Minute),
	})
	require.NoError(t, err)
	devices, err := records.ListActiveTrustedDevices(ctx, account.ID, home)
	require.NoError(t, err)
	require.Empty(t, devices, "neither expired nor revoked devices may be listed")
}
