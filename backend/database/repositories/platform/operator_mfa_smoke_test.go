package platform_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	platformRepo "github.com/moto-nrw/project-phoenix/database/repositories/platform"
	"github.com/moto-nrw/project-phoenix/models/platform"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// TestOperatorMFARepositoriesSmoke is the platform-side mirror of
// TestMFARepositoriesSmoke. Round-trips each method through the real test
// DB to verify the schema + Bun mappings line up. Uses the shared
// createTestOperator helper from announcement_repository_test.go so the
// platform_test package keeps a single operator-fixture shape.
func TestOperatorMFARepositoriesSmoke(t *testing.T) {
	db := testpkg.SetupTestDB(t)

	ctx := context.Background()

	uniqueEmail := fmt.Sprintf("op-mfa-smoke-%d@test.local", time.Now().UnixNano())
	op := createTestOperator(t, db, uniqueEmail, "MFA Smoke Operator")
	defer cleanupTestOperator(t, db, op.ID)

	credentialRepo := platformRepo.NewOperatorMFACredentialRepository(db)
	challengeRepo := platformRepo.NewOperatorMFAEmailChallengeRepository(db)
	deviceRepo := platformRepo.NewOperatorMFATrustedDeviceRepository(db)

	t.Run("credential lifecycle", func(t *testing.T) {
		cred := &platform.OperatorMFACredential{
			OperatorID: op.ID,
			Method:     platform.OperatorMFAMethodEmail,
		}
		require.NoError(t, credentialRepo.Create(ctx, cred))

		fetched, err := credentialRepo.FindByOperatorID(ctx, op.ID)
		require.NoError(t, err)
		assert.Equal(t, cred.ID, fetched.ID)

		require.NoError(t, credentialRepo.UpdateLastUsedAt(ctx, cred.ID, time.Now()))
		require.NoError(t, credentialRepo.DeleteByOperatorID(ctx, op.ID))

		_, err = credentialRepo.FindByOperatorID(ctx, op.ID)
		assert.Error(t, err, "expect error after delete")
	})

	t.Run("email challenge + rate-limit", func(t *testing.T) {
		challenge := &platform.OperatorMFAEmailChallenge{
			OperatorID: op.ID,
			CodeHash:   "$argon2id$placeholder-op-smoke",
			ExpiresAt:  time.Now().Add(10 * time.Minute),
		}
		require.NoError(t, challengeRepo.Create(ctx, challenge))

		active, err := challengeRepo.FindActiveByOperatorID(ctx, op.ID)
		require.NoError(t, err)
		assert.Equal(t, challenge.ID, active.ID)

		count, err := challengeRepo.CountRecentByOperatorID(ctx, op.ID, time.Now().Add(-1*time.Hour))
		require.NoError(t, err)
		assert.GreaterOrEqual(t, count, 1)

		require.NoError(t, challengeRepo.MarkConsumed(ctx, challenge.ID, time.Now()))
		_, err = challengeRepo.FindActiveByOperatorID(ctx, op.ID)
		assert.Error(t, err)

		expired := &platform.OperatorMFAEmailChallenge{
			OperatorID: op.ID,
			CodeHash:   "$argon2id$expired-op",
			ExpiresAt:  time.Now().Add(-5 * time.Minute),
		}
		require.NoError(t, challengeRepo.Create(ctx, expired))
		removed, err := challengeRepo.DeleteExpired(ctx)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, removed, 1)
	})

	t.Run("trusted device + revoke + delete-expired", func(t *testing.T) {
		device := &platform.OperatorMFATrustedDevice{
			OperatorID: op.ID,
			TokenHash:  "sha256:opaque-op-token-hash",
			ExpiresAt:  time.Now().Add(30 * 24 * time.Hour),
		}
		require.NoError(t, deviceRepo.Create(ctx, device))

		fetched, err := deviceRepo.FindActiveByOperatorIDAndTokenHash(ctx, op.ID, device.TokenHash)
		require.NoError(t, err)
		assert.Equal(t, device.ID, fetched.ID)

		listed, err := deviceRepo.ListActiveByOperatorID(ctx, op.ID)
		require.NoError(t, err)
		assert.Len(t, listed, 1)

		require.NoError(t, deviceRepo.UpdateLastUsedAt(ctx, device.ID, time.Now()))
		require.NoError(t, deviceRepo.Revoke(ctx, device.ID, time.Now()))

		listed, err = deviceRepo.ListActiveByOperatorID(ctx, op.ID)
		require.NoError(t, err)
		assert.Empty(t, listed)

		another := &platform.OperatorMFATrustedDevice{
			OperatorID: op.ID,
			TokenHash:  "sha256:another-op-token",
			ExpiresAt:  time.Now().Add(1 * time.Hour),
		}
		require.NoError(t, deviceRepo.Create(ctx, another))
		require.NoError(t, deviceRepo.RevokeAllByOperatorID(ctx, op.ID, time.Now()))

		expired := &platform.OperatorMFATrustedDevice{
			OperatorID: op.ID,
			TokenHash:  "sha256:expired-op-token",
			ExpiresAt:  time.Now().Add(-1 * time.Minute),
		}
		require.NoError(t, deviceRepo.Create(ctx, expired))
		removed, err := deviceRepo.DeleteExpired(ctx)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, removed, 1)
	})
}
