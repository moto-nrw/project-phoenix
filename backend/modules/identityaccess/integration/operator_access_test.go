package integration

import (
	"context"
	"errors"
	"fmt"
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

func newOperatorAccess(t *testing.T, db *bun.DB) identityaccess.OperatorAccess {
	t.Helper()
	module, err := identityCompose.New(identityCompose.Dependencies{DB: db, Observe: func(identityCompose.Observation) {}})
	require.NoError(t, err)
	return module
}

var errInjected = errors.New("injected failure")

func uniqueEmail(prefix string) string {
	return fmt.Sprintf("%s-%s@test.local", prefix, uuid.Must(uuid.NewV4()).String()[:8])
}

func newSession(operatorID int64, familyID string, generation int) identityaccess.OperatorSession {
	return identityaccess.OperatorSession{
		OperatorID: operatorID,
		Token:      uuid.Must(uuid.NewV4()).String(),
		Expiry:     time.Now().Add(time.Hour),
		FamilyID:   familyID,
		Generation: generation,
	}
}

func TestOperatorLifecycle(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	access := newOperatorAccess(t, db)
	ctx := testpkg.Ctx(t)

	email := uniqueEmail("Operator-Lifecycle")
	created, err := access.CreateOperator(ctx, identityaccess.Operator{Email: "  " + email + " ", DisplayName: " Lifecycle ", PasswordHash: "hash", Active: true})
	require.NoError(t, err)
	testpkg.OwnTestOperator(t, db, created.ID)
	require.NotZero(t, created.ID)
	require.NotZero(t, created.CreatedAt)
	assert.Equal(t, "operator-lifecycle"+email[len("Operator-Lifecycle"):], created.Email, "address is normalized")
	assert.Equal(t, "Lifecycle", created.DisplayName)

	found, err := access.FindOperator(ctx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, created.Email, found.Email)
	byEmail, err := access.FindOperatorByEmail(ctx, created.Email)
	require.NoError(t, err)
	assert.Equal(t, created.ID, byEmail.ID)

	_, err = access.FindOperator(ctx, created.ID+1_000_000)
	require.ErrorIs(t, err, identityaccess.ErrOperatorNotFound)
	_, err = access.FindOperatorByEmail(ctx, uniqueEmail("nobody"))
	require.ErrorIs(t, err, identityaccess.ErrOperatorNotFound)

	found.DisplayName = "Renamed"
	found.Active = false
	updated, err := access.UpdateOperator(ctx, found)
	require.NoError(t, err)
	assert.Equal(t, "Renamed", updated.DisplayName)
	reloaded, err := access.FindOperator(ctx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, "Renamed", reloaded.DisplayName)
	assert.False(t, reloaded.Active)

	_, err = access.UpdateOperator(ctx, identityaccess.Operator{ID: created.ID + 1_000_000, Email: email, DisplayName: "Ghost", PasswordHash: "hash"})
	require.ErrorIs(t, err, identityaccess.ErrOperatorNotFound)

	_, err = access.CreateOperator(ctx, identityaccess.Operator{Email: "", DisplayName: "No address", PasswordHash: "hash"})
	require.EqualError(t, err, "identity access: create operator: email is required")

	listed, err := access.ListOperators(ctx)
	require.NoError(t, err)
	var seen bool
	for _, operator := range listed {
		seen = seen || operator.ID == created.ID
	}
	assert.True(t, seen, "list includes the created operator")

	require.NoError(t, access.RecordOperatorLogin(ctx, created.ID))
	reloaded, err = access.FindOperator(ctx, created.ID)
	require.NoError(t, err)
	require.NotNil(t, reloaded.LastLogin)

	require.NoError(t, access.DeleteOperator(ctx, created.ID))
	_, err = access.FindOperator(ctx, created.ID)
	require.ErrorIs(t, err, identityaccess.ErrOperatorNotFound)
}

func TestOperatorMFAAttemptsLockAtThreshold(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	access := newOperatorAccess(t, db)
	ctx := testpkg.Ctx(t)
	operator := testpkg.CreateTestOperator(t, db)

	first, err := access.IncrementOperatorMFAAttempts(ctx, operator.ID, 2, 15*time.Minute)
	require.NoError(t, err)
	assert.Equal(t, 1, first.Attempts)
	assert.Nil(t, first.LockedUntil)
	second, err := access.IncrementOperatorMFAAttempts(ctx, operator.ID, 2, 15*time.Minute)
	require.NoError(t, err)
	assert.Equal(t, 2, second.Attempts)
	require.NotNil(t, second.LockedUntil, "threshold applies the lockout")

	require.NoError(t, access.ResetOperatorMFAAttempts(ctx, operator.ID))
	reloaded, err := access.FindOperator(ctx, operator.ID)
	require.NoError(t, err)
	assert.Zero(t, reloaded.MFAAttempts)
	assert.Nil(t, reloaded.MFALockedUntil)

	_, err = access.IncrementOperatorMFAAttempts(ctx, operator.ID+1_000_000, 2, time.Minute)
	require.ErrorIs(t, err, identityaccess.ErrOperatorNotFound)
}

func TestOperatorSessionRotationHandoff(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	access := newOperatorAccess(t, db)
	ctx := testpkg.Ctx(t)
	operator := testpkg.CreateTestOperator(t, db)
	familyID := uuid.Must(uuid.NewV4()).String()

	predecessor, err := access.CreateOperatorSession(ctx, newSession(operator.ID, familyID, 0))
	require.NoError(t, err)
	require.NotZero(t, predecessor.ID)
	successor, err := access.CreateOperatorSession(ctx, newSession(operator.ID, familyID, 1))
	require.NoError(t, err)

	_, err = access.CreateOperatorSession(ctx, identityaccess.OperatorSession{OperatorID: operator.ID, Token: "", Expiry: time.Now(), FamilyID: familyID})
	require.EqualError(t, err, "identity access: create operator session: token value is required")

	latest, err := access.LatestOperatorSessionInFamily(ctx, familyID)
	require.NoError(t, err)
	assert.Equal(t, successor.ID, latest.ID)

	proof := []byte{1, 2, 3}
	rotatedAt := time.Now().Add(-10 * time.Minute)
	require.NoError(t, access.MarkOperatorSessionRotated(ctx, predecessor.ID, successor.Token, proof, rotatedAt))
	err = access.MarkOperatorSessionRotated(ctx, predecessor.ID, successor.Token, proof, rotatedAt)
	require.ErrorIs(t, err, identityaccess.ErrOperatorSessionRotated, "a second hand-off is a replay signal, never a silent re-rotation")

	stored, err := access.FindOperatorSessionForUpdate(ctx, predecessor.Token)
	require.NoError(t, err)
	require.NotNil(t, stored.RotatedAt)
	require.NotNil(t, stored.ReplacementToken)
	assert.Equal(t, successor.Token, *stored.ReplacementToken)
	assert.Equal(t, proof, stored.RecoveryProofHash)

	// A rotated predecessor stays as replay evidence until its refresh JWT
	// expires.
	require.NoError(t, access.DeleteExpiredRotatedOperatorSessions(ctx, familyID, time.Now()))
	_, err = access.FindOperatorSessionForUpdate(ctx, predecessor.Token)
	require.NoError(t, err)
	_, err = db.NewUpdate().Table("platform.operator_refresh_tokens").Set("expiry = ?", time.Now().Add(-time.Minute)).Where("id = ?", predecessor.ID).Exec(ctx)
	require.NoError(t, err)
	require.NoError(t, access.DeleteExpiredRotatedOperatorSessions(ctx, familyID, time.Now()))
	_, err = access.FindOperatorSessionForUpdate(ctx, predecessor.Token)
	require.ErrorIs(t, err, identityaccess.ErrOperatorSessionNotFound)
	kept, err := access.FindOperatorSessionForUpdate(ctx, successor.Token)
	require.NoError(t, err)
	assert.Equal(t, 1, kept.Generation)
}

func TestOperatorSessionRevocationReturnsDeletedRows(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	access := newOperatorAccess(t, db)
	ctx := testpkg.Ctx(t)
	operator := testpkg.CreateTestOperator(t, db)
	familyA := uuid.Must(uuid.NewV4()).String()
	familyB := uuid.Must(uuid.NewV4()).String()
	for generation := range 2 {
		_, err := access.CreateOperatorSession(ctx, newSession(operator.ID, familyA, generation))
		require.NoError(t, err)
	}
	_, err := access.CreateOperatorSession(ctx, newSession(operator.ID, familyB, 0))
	require.NoError(t, err)

	family, err := access.RevokeOperatorSessionFamily(ctx, familyA)
	require.NoError(t, err)
	assert.Len(t, family, 2)
	_, err = access.LatestOperatorSessionInFamily(ctx, familyA)
	require.ErrorIs(t, err, identityaccess.ErrOperatorSessionNotFound)

	all, err := access.RevokeOperatorSessions(ctx, operator.ID)
	require.NoError(t, err)
	require.Len(t, all, 1)
	assert.Equal(t, familyB, all[0].FamilyID)

	expired := newSession(operator.ID, uuid.Must(uuid.NewV4()).String(), 0)
	expired.Expiry = time.Now().Add(-time.Minute)
	_, err = access.CreateOperatorSession(ctx, expired)
	require.NoError(t, err)
	_, err = access.CreateOperatorSession(ctx, newSession(operator.ID, uuid.Must(uuid.NewV4()).String(), 0))
	require.NoError(t, err)
	deleted, err := access.DeleteExpiredOperatorSessions(ctx, time.Now())
	require.NoError(t, err)
	assert.GreaterOrEqual(t, deleted, 1)
	remaining, err := access.RevokeOperatorSessions(ctx, operator.ID)
	require.NoError(t, err)
	assert.Len(t, remaining, 1, "only the live session survived the expiry sweep")
}

// Operator flows open one administrative transaction around refresh and
// revocation. Every operation must join it: a failure after the session
// write rolls the write back, and a retry succeeds from a clean state.
func TestOperatorSessionWritesJoinAmbientTransaction(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	access := newOperatorAccess(t, db)
	operator := testpkg.CreateTestOperator(t, db)
	ctx := testpkg.WithTenantRuntime(t, testpkg.Ctx(t), db)
	familyID := uuid.Must(uuid.NewV4()).String()
	token := newSession(operator.ID, familyID, 0)

	err := testpkg.WithAdminTx(t, ctx, db, func(txCtx context.Context, _ bun.Tx) error {
		if _, err := access.CreateOperatorSession(txCtx, token); err != nil {
			return err
		}
		if err := access.RecordOperatorLogin(txCtx, operator.ID); err != nil {
			return err
		}
		return errInjected
	})
	require.ErrorIs(t, err, errInjected)
	_, err = access.FindOperatorSessionForUpdate(testpkg.Ctx(t), token.Token)
	require.ErrorIs(t, err, identityaccess.ErrOperatorSessionNotFound, "the rolled-back session must not exist")
	reloaded, err := access.FindOperator(testpkg.Ctx(t), operator.ID)
	require.NoError(t, err)
	assert.Nil(t, reloaded.LastLogin, "the rolled-back login stamp must not exist")

	err = testpkg.WithAdminTx(t, ctx, db, func(txCtx context.Context, _ bun.Tx) error {
		_, err := access.CreateOperatorSession(txCtx, token)
		return err
	})
	require.NoError(t, err, "the retry starts from a clean state")
	stored, err := access.FindOperatorSessionForUpdate(testpkg.Ctx(t), token.Token)
	require.NoError(t, err)
	assert.Equal(t, operator.ID, stored.OperatorID)
}

func TestOperatorObservationsCountRows(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	operator := testpkg.CreateTestOperator(t, db)
	var observations []identityCompose.Observation
	access, err := identityCompose.New(identityCompose.Dependencies{DB: db, Observe: func(observation identityCompose.Observation) {
		observations = append(observations, observation)
	}})
	require.NoError(t, err)
	ctx := testpkg.Ctx(t)

	_, err = access.FindOperatorByEmail(ctx, operator.Email)
	require.NoError(t, err)
	session, err := access.CreateOperatorSession(ctx, newSession(operator.ID, uuid.Must(uuid.NewV4()).String(), 0))
	require.NoError(t, err)
	_, err = access.RevokeOperatorSessions(ctx, operator.ID)
	require.NoError(t, err)
	_, err = access.FindOperatorSessionForUpdate(ctx, session.Token)
	require.ErrorIs(t, err, identityaccess.ErrOperatorSessionNotFound)

	require.Len(t, observations, 4)
	assert.Equal(t, "find_operator_by_email", observations[0].Operation)
	assert.Zero(t, observations[0].Stats.Rows)
	assert.Equal(t, "create_operator_session", observations[1].Operation)
	assert.EqualValues(t, 1, observations[1].Stats.Rows)
	assert.Equal(t, "revoke_operator_sessions", observations[2].Operation)
	assert.EqualValues(t, 1, observations[2].Stats.Rows)
	assert.Equal(t, "find_operator_session_for_update", observations[3].Operation)
	require.ErrorIs(t, observations[3].Err, identityaccess.ErrOperatorSessionNotFound)
	assert.Equal(t, "not_found", identityaccess.ErrorCode(observations[3].Err))
	for _, observation := range observations {
		assert.EqualValues(t, 1, observation.Stats.Queries, observation.Operation)
	}
}
