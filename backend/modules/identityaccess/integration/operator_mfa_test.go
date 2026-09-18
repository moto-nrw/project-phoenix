package integration

import (
	"context"
	"net"
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

func newOperatorMFARecords(t *testing.T, db *bun.DB) identityaccess.OperatorMFARecords {
	t.Helper()
	module, err := identityCompose.New(identityCompose.Dependencies{DB: db, Observe: func(identityCompose.Observation) {}})
	require.NoError(t, err)
	return module
}

func pendingChallenge(operatorID int64, expiresAt time.Time) identityaccess.OperatorMFAChallenge {
	consumed := time.Now()
	return identityaccess.OperatorMFAChallenge{
		OperatorID: operatorID, CodeHash: "hash-" + uuid.Must(uuid.NewV4()).String(), ExpiresAt: expiresAt,
		ConsumedAt: &consumed, IPAddress: net.ParseIP("203.0.113.5"),
	}
}

func newTrustedDevice(operatorID int64, expiresAt time.Time) identityaccess.OperatorTrustedDevice {
	userAgent := "Mozilla/5.0 (Test)"
	return identityaccess.OperatorTrustedDevice{
		OperatorID: operatorID, TokenHash: "token-" + uuid.Must(uuid.NewV4()).String(), UserAgent: &userAgent,
		IPAddress: net.ParseIP("203.0.113.6"), ExpiresAt: expiresAt,
	}
}

func TestOperatorMFACredentialLifecycle(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	records := newOperatorMFARecords(t, db)
	ctx := testpkg.Ctx(t)
	operator := testpkg.CreateTestOperator(t, db)

	_, err := records.FindOperatorMFACredential(ctx, operator.ID)
	require.ErrorIs(t, err, identityaccess.ErrOperatorMFACredentialNotFound)

	_, err = records.CreateOperatorMFACredential(ctx, identityaccess.OperatorMFACredential{OperatorID: operator.ID, Method: "totp"})
	require.EqualError(t, err, "identity access: create operator mfa credential: unsupported MFA method")
	_, err = records.CreateOperatorMFACredential(ctx, identityaccess.OperatorMFACredential{Method: "email"})
	require.EqualError(t, err, "identity access: create operator mfa credential: operator_id is required")

	enrolledAt := time.Now().Add(-time.Minute).Truncate(time.Microsecond)
	created, err := records.CreateOperatorMFACredential(ctx, identityaccess.OperatorMFACredential{OperatorID: operator.ID, Method: "email", EnrolledAt: enrolledAt})
	require.NoError(t, err)
	require.NotZero(t, created.ID)
	require.NotZero(t, created.CreatedAt)
	assert.True(t, enrolledAt.Equal(created.EnrolledAt))
	_, err = records.CreateOperatorMFACredential(ctx, identityaccess.OperatorMFACredential{OperatorID: operator.ID, Method: "email"})
	require.Error(t, err, "one enrollment per operator and method")

	found, err := records.FindOperatorMFACredential(ctx, operator.ID)
	require.NoError(t, err)
	assert.Equal(t, created.ID, found.ID)
	assert.Nil(t, found.LastUsedAt)

	usedAt := time.Now().Truncate(time.Microsecond)
	require.NoError(t, records.TouchOperatorMFACredential(ctx, created.ID, usedAt))
	found, err = records.FindOperatorMFACredential(ctx, operator.ID)
	require.NoError(t, err)
	require.NotNil(t, found.LastUsedAt)
	assert.True(t, usedAt.Equal(*found.LastUsedAt))

	require.NoError(t, records.DeleteOperatorMFACredentials(ctx, operator.ID))
	_, err = records.FindOperatorMFACredential(ctx, operator.ID)
	require.ErrorIs(t, err, identityaccess.ErrOperatorMFACredentialNotFound)
	require.NoError(t, records.DeleteOperatorMFACredentials(ctx, operator.ID), "deleting a missing enrollment is a no-op")
}

// A code is stored consumed until its delivery is confirmed, becomes
// redeemable exactly once, and is spent exactly once.
func TestOperatorMFAChallengeIsRedeemableOnce(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	records := newOperatorMFARecords(t, db)
	ctx := testpkg.Ctx(t)
	operator := testpkg.CreateTestOperator(t, db)
	before := time.Now().Add(-time.Second)

	invalid := pendingChallenge(operator.ID, time.Now().Add(time.Hour))
	invalid.CodeHash = ""
	_, err := records.CreateOperatorMFAChallenge(ctx, invalid)
	require.EqualError(t, err, "identity access: create operator mfa challenge: code_hash is required")

	challenge, err := records.CreateOperatorMFAChallenge(ctx, pendingChallenge(operator.ID, time.Now().Add(10*time.Minute)))
	require.NoError(t, err)
	require.NotZero(t, challenge.ID)
	assert.Equal(t, "203.0.113.5", challenge.IPAddress.String())

	_, err = records.FindActiveOperatorMFAChallenge(ctx, operator.ID)
	require.ErrorIs(t, err, identityaccess.ErrOperatorMFAChallengeNotFound, "an undelivered code is not redeemable")

	require.NoError(t, records.ActivateOperatorMFAChallenge(ctx, challenge.ID))
	require.ErrorIs(t, records.ActivateOperatorMFAChallenge(ctx, challenge.ID), identityaccess.ErrOperatorMFAChallengeStateChanged)

	active, err := records.FindActiveOperatorMFAChallenge(ctx, operator.ID)
	require.NoError(t, err)
	assert.Equal(t, challenge.ID, active.ID)
	assert.Equal(t, challenge.CodeHash, active.CodeHash)
	assert.Nil(t, active.ConsumedAt)

	require.NoError(t, records.ConsumeOperatorMFAChallenge(ctx, challenge.ID, time.Now()))
	require.ErrorIs(t, records.ConsumeOperatorMFAChallenge(ctx, challenge.ID, time.Now()), identityaccess.ErrOperatorMFAChallengeStateChanged,
		"the loser of two verifications must not redeem the code again")
	_, err = records.FindActiveOperatorMFAChallenge(ctx, operator.ID)
	require.ErrorIs(t, err, identityaccess.ErrOperatorMFAChallengeNotFound)
	require.ErrorIs(t, records.ConsumeOperatorMFAChallenge(ctx, challenge.ID+1_000_000, time.Now()), identityaccess.ErrOperatorMFAChallengeStateChanged)

	expired, err := records.CreateOperatorMFAChallenge(ctx, pendingChallenge(operator.ID, time.Now().Add(-time.Minute)))
	require.NoError(t, err)
	require.NoError(t, records.ActivateOperatorMFAChallenge(ctx, expired.ID))
	_, err = records.FindActiveOperatorMFAChallenge(ctx, operator.ID)
	require.ErrorIs(t, err, identityaccess.ErrOperatorMFAChallengeNotFound, "an expired code is not redeemable")

	older, err := records.CreateOperatorMFAChallenge(ctx, pendingChallenge(operator.ID, time.Now().Add(5*time.Minute)))
	require.NoError(t, err)
	newer, err := records.CreateOperatorMFAChallenge(ctx, pendingChallenge(operator.ID, time.Now().Add(9*time.Minute)))
	require.NoError(t, err)
	require.NoError(t, records.ActivateOperatorMFAChallenge(ctx, older.ID))
	require.NoError(t, records.ActivateOperatorMFAChallenge(ctx, newer.ID))
	active, err = records.FindActiveOperatorMFAChallenge(ctx, operator.ID)
	require.NoError(t, err)
	assert.Equal(t, newer.ID, active.ID, "the code with the latest expiry wins")

	count, err := records.CountOperatorMFAChallengesSince(ctx, operator.ID, before)
	require.NoError(t, err)
	assert.Equal(t, 4, count, "the rate limit counts every code sent, redeemed or not")
	count, err = records.CountOperatorMFAChallengesSince(ctx, operator.ID, time.Now().Add(time.Minute))
	require.NoError(t, err)
	assert.Zero(t, count)
}

func TestOperatorTrustedDeviceLifecycle(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	records := newOperatorMFARecords(t, db)
	ctx := testpkg.Ctx(t)
	operator := testpkg.CreateTestOperator(t, db)
	other := testpkg.CreateTestOperator(t, db)

	invalid := newTrustedDevice(operator.ID, time.Time{})
	_, err := records.CreateOperatorTrustedDevice(ctx, invalid)
	require.EqualError(t, err, "identity access: create operator trusted device: expires_at is required")

	first, err := records.CreateOperatorTrustedDevice(ctx, newTrustedDevice(operator.ID, time.Now().Add(time.Hour)))
	require.NoError(t, err)
	require.NotZero(t, first.ID)
	require.NotNil(t, first.UserAgent)
	assert.Equal(t, "Mozilla/5.0 (Test)", *first.UserAgent)
	second, err := records.CreateOperatorTrustedDevice(ctx, newTrustedDevice(operator.ID, time.Now().Add(time.Hour)))
	require.NoError(t, err)
	expired, err := records.CreateOperatorTrustedDevice(ctx, newTrustedDevice(operator.ID, time.Now().Add(-time.Minute)))
	require.NoError(t, err)
	foreign, err := records.CreateOperatorTrustedDevice(ctx, newTrustedDevice(other.ID, time.Now().Add(time.Hour)))
	require.NoError(t, err)

	found, err := records.FindActiveOperatorTrustedDevice(ctx, operator.ID, first.TokenHash)
	require.NoError(t, err)
	assert.Equal(t, first.ID, found.ID)
	_, err = records.FindActiveOperatorTrustedDevice(ctx, operator.ID, foreign.TokenHash)
	require.ErrorIs(t, err, identityaccess.ErrOperatorTrustedDeviceNotFound, "another operator's cookie never verifies")
	_, err = records.FindActiveOperatorTrustedDevice(ctx, operator.ID, expired.TokenHash)
	require.ErrorIs(t, err, identityaccess.ErrOperatorTrustedDeviceNotFound)
	_, err = records.FindActiveOperatorTrustedDevice(ctx, operator.ID, "")
	require.ErrorIs(t, err, identityaccess.ErrOperatorTrustedDeviceNotFound)

	require.NoError(t, records.TouchOperatorTrustedDevice(ctx, first.ID, time.Now()))
	listed, err := records.ListActiveOperatorTrustedDevices(ctx, operator.ID)
	require.NoError(t, err)
	require.Len(t, listed, 2, "expired and foreign devices are not listed")
	assert.Equal(t, first.ID, listed[0].ID, "the most recently used device comes first")
	assert.Equal(t, second.ID, listed[1].ID)
	require.NotNil(t, listed[0].LastUsedAt)

	require.NoError(t, records.RevokeOperatorTrustedDevice(ctx, first.ID, time.Now()))
	require.ErrorIs(t, records.RevokeOperatorTrustedDevice(ctx, first.ID, time.Now()), identityaccess.ErrOperatorTrustedDeviceNotFound)
	_, err = records.FindActiveOperatorTrustedDevice(ctx, operator.ID, first.TokenHash)
	require.ErrorIs(t, err, identityaccess.ErrOperatorTrustedDeviceNotFound)

	require.NoError(t, records.RevokeOperatorTrustedDevices(ctx, operator.ID, time.Now()))
	listed, err = records.ListActiveOperatorTrustedDevices(ctx, operator.ID)
	require.NoError(t, err)
	assert.Empty(t, listed)
	assert.NotNil(t, listed, "an empty listing is an empty slice")
	foreignListed, err := records.ListActiveOperatorTrustedDevices(ctx, other.ID)
	require.NoError(t, err)
	require.Len(t, foreignListed, 1, "revoking one operator's devices leaves other operators alone")
}

// The operator MFA tables carry no tenant column: a tenant context must
// neither hide nor reveal rows, because every other table this owner touches
// is tenant-scoped and a reader could assume the same here.
func TestOperatorMFARecordsAreVisibleFromEveryTenantContext(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	records := newOperatorMFARecords(t, db)
	operator := testpkg.CreateTestOperator(t, db)
	ctx := testpkg.Ctx(t)
	_, err := records.CreateOperatorMFACredential(ctx, identityaccess.OperatorMFACredential{OperatorID: operator.ID, Method: "email"})
	require.NoError(t, err)
	challenge, err := records.CreateOperatorMFAChallenge(ctx, pendingChallenge(operator.ID, time.Now().Add(time.Hour)))
	require.NoError(t, err)
	require.NoError(t, records.ActivateOperatorMFAChallenge(ctx, challenge.ID))
	device, err := records.CreateOperatorTrustedDevice(ctx, newTrustedDevice(operator.ID, time.Now().Add(time.Hour)))
	require.NoError(t, err)

	first := testpkg.Tenant(t)
	second, _ := testpkg.CreateTestTenant(t, db)
	require.NotEqual(t, first, second)

	for name, tenantID := range map[string]int64{"own tenant": first, "foreign tenant": second} {
		t.Run(name, func(t *testing.T) {
			tenantCtx := testpkg.TenantContext(tenantID)
			_, err := records.FindOperatorMFACredential(tenantCtx, operator.ID)
			require.NoError(t, err)
			active, err := records.FindActiveOperatorMFAChallenge(tenantCtx, operator.ID)
			require.NoError(t, err)
			assert.Equal(t, challenge.ID, active.ID)
			found, err := records.FindActiveOperatorTrustedDevice(tenantCtx, operator.ID, device.TokenHash)
			require.NoError(t, err)
			assert.Equal(t, device.ID, found.ID)
		})
	}
}

// Disabling MFA performs three authoritative writes in one administrative
// transaction: the enrollment delete, the trusted-device revocation and the
// lockout reset. A failure after any of them rolls every earlier write back,
// and a retry succeeds from the untouched state.
func TestOperatorMFADisableRollsBackAfterEachWrite(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module, err := identityCompose.New(identityCompose.Dependencies{DB: db, Observe: func(identityCompose.Observation) {}})
	require.NoError(t, err)
	operator := testpkg.CreateTestOperator(t, db)
	plain := testpkg.Ctx(t)
	ctx := testpkg.WithTenantRuntime(t, plain, db)

	_, err = module.CreateOperatorMFACredential(plain, identityaccess.OperatorMFACredential{OperatorID: operator.ID, Method: "email"})
	require.NoError(t, err)
	device, err := module.CreateOperatorTrustedDevice(plain, newTrustedDevice(operator.ID, time.Now().Add(time.Hour)))
	require.NoError(t, err)
	_, err = module.IncrementOperatorMFAAttempts(plain, operator.ID, 5, time.Minute)
	require.NoError(t, err)

	writes := []func(context.Context) error{
		func(txCtx context.Context) error { return module.DeleteOperatorMFACredentials(txCtx, operator.ID) },
		func(txCtx context.Context) error {
			return module.RevokeOperatorTrustedDevices(txCtx, operator.ID, time.Now())
		},
		func(txCtx context.Context) error { return module.ResetOperatorMFAAttempts(txCtx, operator.ID) },
	}
	assertUntouched := func(t *testing.T) {
		t.Helper()
		_, err := module.FindOperatorMFACredential(plain, operator.ID)
		require.NoError(t, err, "the enrollment delete must be rolled back")
		_, err = module.FindActiveOperatorTrustedDevice(plain, operator.ID, device.TokenHash)
		require.NoError(t, err, "the device revocation must be rolled back")
		reloaded, err := module.FindOperator(plain, operator.ID)
		require.NoError(t, err)
		assert.Equal(t, 1, reloaded.MFAAttempts, "the lockout reset must be rolled back")
	}

	for failAfter := range writes {
		err := testpkg.WithAdminTx(t, ctx, db, func(txCtx context.Context, _ bun.Tx) error {
			for index, write := range writes[:failAfter+1] {
				if err := write(txCtx); err != nil {
					t.Fatalf("write %d: %v", index, err)
				}
			}
			return errInjected
		})
		require.ErrorIs(t, err, errInjected)
		assertUntouched(t)
	}

	err = testpkg.WithAdminTx(t, ctx, db, func(txCtx context.Context, _ bun.Tx) error {
		for _, write := range writes {
			if err := write(txCtx); err != nil {
				return err
			}
		}
		return nil
	})
	require.NoError(t, err, "the retry starts from a clean state")
	_, err = module.FindOperatorMFACredential(plain, operator.ID)
	require.ErrorIs(t, err, identityaccess.ErrOperatorMFACredentialNotFound)
	_, err = module.FindActiveOperatorTrustedDevice(plain, operator.ID, device.TokenHash)
	require.ErrorIs(t, err, identityaccess.ErrOperatorTrustedDeviceNotFound)
	reloaded, err := module.FindOperator(plain, operator.ID)
	require.NoError(t, err)
	assert.Zero(t, reloaded.MFAAttempts)
}

func TestOperatorMFAObservationsUseStableOperations(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	operator := testpkg.CreateTestOperator(t, db)
	var observations []identityCompose.Observation
	records, err := identityCompose.New(identityCompose.Dependencies{DB: db, Observe: func(observation identityCompose.Observation) {
		observations = append(observations, observation)
	}})
	require.NoError(t, err)
	ctx := testpkg.Ctx(t)

	_, err = records.FindOperatorMFACredential(ctx, operator.ID)
	require.Error(t, err)
	challenge, err := records.CreateOperatorMFAChallenge(ctx, pendingChallenge(operator.ID, time.Now().Add(time.Hour)))
	require.NoError(t, err)
	require.NoError(t, records.ActivateOperatorMFAChallenge(ctx, challenge.ID))
	require.NoError(t, records.ConsumeOperatorMFAChallenge(ctx, challenge.ID, time.Now()))
	require.Error(t, records.ConsumeOperatorMFAChallenge(ctx, challenge.ID, time.Now()))

	require.Len(t, observations, 5)
	expected := []struct {
		operation string
		rows      int64
		code      string
	}{
		{"find_operator_mfa_credential", 0, "not_found"},
		{"create_operator_mfa_challenge", 1, "none"},
		{"activate_operator_mfa_challenge", 1, "none"},
		{"consume_operator_mfa_challenge", 1, "none"},
		{"consume_operator_mfa_challenge", 0, "conflict"},
	}
	for index, want := range expected {
		observation := observations[index]
		assert.Equal(t, want.operation, observation.Operation)
		assert.Equal(t, want.rows, observation.Stats.Rows, want.operation)
		assert.Equal(t, want.code, identityaccess.ErrorCode(observation.Err), want.operation)
		assert.EqualValues(t, 1, observation.Stats.Queries, want.operation)
	}
}
