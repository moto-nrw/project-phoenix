package auth_test

import (
	"context"
	"testing"
	"time"

	authjwt "github.com/moto-nrw/project-phoenix/auth/jwt"
	authRepo "github.com/moto-nrw/project-phoenix/database/repositories/auth"
	"github.com/moto-nrw/project-phoenix/models/auth"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMFARepositoriesSmoke verifies that the four MFA repositories created in
// Phase 2 of issue #1308 round-trip basic data through the schema created in
// Phase 1. It exercises the most common paths (create, find-active, mark-used,
// list-active, delete-expired) without trying to be exhaustive — Phase 13
// adds full coverage.
func TestMFARepositoriesSmoke(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	ctx := context.Background()

	account := testpkg.CreateTestAccount(t, db, "mfa-smoke@test.local")
	defer testpkg.CleanupAccount(t, db, account.ID)

	credentialRepo := authRepo.NewMFACredentialRepository(db)
	challengeRepo := authRepo.NewMFAEmailChallengeRepository(db)
	deviceRepo := authRepo.NewMFATrustedDeviceRepository(db)

	t.Run("credential lifecycle", func(t *testing.T) {
		cred := &auth.MFACredential{
			AccountID: account.ID,
			Method:    auth.MFAMethodEmail,
		}
		require.NoError(t, credentialRepo.Create(ctx, cred))

		fetched, err := credentialRepo.FindByAccountID(ctx, account.ID)
		require.NoError(t, err)
		assert.Equal(t, cred.ID, fetched.ID)
		assert.Equal(t, auth.MFAMethodEmail, fetched.Method)

		now := time.Now()
		require.NoError(t, credentialRepo.UpdateLastUsedAt(ctx, cred.ID, now))

		require.NoError(t, credentialRepo.DeleteByAccountID(ctx, account.ID))
		_, err = credentialRepo.FindByAccountID(ctx, account.ID)
		assert.Error(t, err, "expect error after delete")
	})

	t.Run("email challenge lifecycle + rate-limit count", func(t *testing.T) {
		challenge := &auth.MFAEmailChallenge{
			AccountID: account.ID,
			Scope:     authjwt.MFAChallengeScopeTenant,
			CodeHash:  "$argon2id$placeholder-hash-for-smoke-test",
			ExpiresAt: time.Now().Add(10 * time.Minute),
		}
		require.NoError(t, challengeRepo.Create(ctx, challenge))

		active, err := challengeRepo.FindActiveByAccountIDInScope(ctx, account.ID, 0, authjwt.MFAChallengeScopeTenant)
		require.NoError(t, err)
		assert.Equal(t, challenge.ID, active.ID)

		count, err := challengeRepo.CountRecentByAccountID(ctx, account.ID, time.Now().Add(-1*time.Hour))
		require.NoError(t, err)
		assert.GreaterOrEqual(t, count, 1)

		require.NoError(t, challengeRepo.MarkConsumed(ctx, challenge.ID, time.Now()))

		_, err = challengeRepo.FindActiveByAccountIDInScope(ctx, account.ID, 0, authjwt.MFAChallengeScopeTenant)
		assert.Error(t, err, "expect no active challenge after consume")

		// Insert an expired one and verify cleanup picks it up.
		expired := &auth.MFAEmailChallenge{
			AccountID: account.ID,
			Scope:     authjwt.MFAChallengeScopeTenant,
			CodeHash:  "$argon2id$expired",
			ExpiresAt: time.Now().Add(-5 * time.Minute),
		}
		require.NoError(t, challengeRepo.Create(ctx, expired))
		removed, err := challengeRepo.DeleteExpired(ctx)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, removed, 1)
	})

	t.Run("trusted device lifecycle + revoke", func(t *testing.T) {
		// #1430 review item #9: rows are scoped to (account, tenant). Pick a
		// unique tenant for this test so it doesn't collide with parallel
		// suites that use shared seed IDs.
		tenantID := testpkg.UniqueTestTenantID(t)
		testpkg.EnsureTestTenant(t, db, tenantID)

		device := &auth.MFATrustedDevice{
			AccountID: account.ID,
			TenantID:  tenantID,
			TokenHash: "sha256:opaque-token-hash",
			ExpiresAt: time.Now().Add(30 * 24 * time.Hour),
		}
		require.NoError(t, deviceRepo.Create(ctx, device))

		fetched, err := deviceRepo.FindActiveByAccountTenantAndTokenHash(ctx, account.ID, tenantID, device.TokenHash)
		require.NoError(t, err)
		assert.Equal(t, device.ID, fetched.ID)

		listed, err := deviceRepo.ListActiveByAccountTenant(ctx, account.ID, tenantID)
		require.NoError(t, err)
		assert.Len(t, listed, 1)

		require.NoError(t, deviceRepo.UpdateLastUsedAt(ctx, device.ID, time.Now()))

		require.NoError(t, deviceRepo.Revoke(ctx, device.ID, time.Now()))
		listed, err = deviceRepo.ListActiveByAccountTenant(ctx, account.ID, tenantID)
		require.NoError(t, err)
		assert.Len(t, listed, 0)

		// Insert another and verify RevokeAllByAccountID + DeleteExpired.
		another := &auth.MFATrustedDevice{
			AccountID: account.ID,
			TenantID:  tenantID,
			TokenHash: "sha256:another-token",
			ExpiresAt: time.Now().Add(1 * time.Hour),
		}
		require.NoError(t, deviceRepo.Create(ctx, another))
		require.NoError(t, deviceRepo.RevokeAllByAccountID(ctx, account.ID, time.Now()))

		expired := &auth.MFATrustedDevice{
			AccountID: account.ID,
			TenantID:  tenantID,
			TokenHash: "sha256:expired-token",
			ExpiresAt: time.Now().Add(-1 * time.Minute),
		}
		require.NoError(t, deviceRepo.Create(ctx, expired))
		removed, err := deviceRepo.DeleteExpired(ctx)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, removed, 1)
	})
}
