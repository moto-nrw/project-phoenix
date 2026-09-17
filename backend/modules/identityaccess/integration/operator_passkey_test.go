package integration

import (
	"context"
	"encoding/json"
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

func newOperatorPasskeyRecords(t *testing.T, db *bun.DB) *identityaccess.Module {
	t.Helper()
	module, err := identityCompose.New(identityCompose.Dependencies{DB: db, Observe: func(identityCompose.Observation) {}})
	require.NoError(t, err)
	return module
}

func newOperatorPasskey(operatorID int64, userHandle []byte) identityaccess.OperatorPasskeyCredential {
	return identityaccess.OperatorPasskeyCredential{
		OperatorID: operatorID, UserHandle: userHandle, CredentialID: []byte("credential-" + uuid.Must(uuid.NewV4()).String()),
		CredentialJSON: json.RawMessage(`{"id":"registered"}`), Name: "Admin laptop",
	}
}

// newOperatorPasskeySession builds a ceremony whose row the test owns.
func newOperatorPasskeySession(t *testing.T, db *bun.DB, operatorID *int64, purpose string, expiresAt time.Time) identityaccess.OperatorPasskeySession {
	t.Helper()
	id := "operator-passkey-" + uuid.Must(uuid.NewV4()).String()
	testpkg.OwnTestOperatorPasskeySession(t, db, id)
	return identityaccess.OperatorPasskeySession{
		ID: id, OperatorID: operatorID, Purpose: purpose,
		RPID: "operator.localhost", ExpectedOrigin: "http://operator.localhost:3000",
		SessionJSON: json.RawMessage(`{"challenge":"abc"}`), ExpiresAt: expiresAt,
	}
}

func TestOperatorPasskeyCredentialLifecycle(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	records := newOperatorPasskeyRecords(t, db)
	ctx := testpkg.Ctx(t)
	operator := testpkg.CreateTestOperator(t, db)
	other := testpkg.CreateTestOperator(t, db)
	handle := []byte("handle-" + uuid.Must(uuid.NewV4()).String())

	invalid := newOperatorPasskey(operator.ID, handle)
	invalid.CredentialJSON = json.RawMessage(`{`)
	_, err := records.CreateOperatorPasskey(ctx, invalid)
	require.EqualError(t, err, "identity access: create operator passkey: credential_json must be valid JSON")
	invalid = newOperatorPasskey(operator.ID, nil)
	_, err = records.CreateOperatorPasskey(ctx, invalid)
	require.EqualError(t, err, "identity access: create operator passkey: user_handle is required")
	_, err = records.CreateOperatorPasskey(ctx, newOperatorPasskey(0, handle))
	require.EqualError(t, err, "identity access: create operator passkey: operator_id is required")

	listed, err := records.ListActiveOperatorPasskeys(ctx, operator.ID)
	require.NoError(t, err)
	assert.Empty(t, listed)
	assert.NotNil(t, listed, "an empty listing is an empty slice")

	first, err := records.CreateOperatorPasskey(ctx, newOperatorPasskey(operator.ID, handle))
	require.NoError(t, err)
	require.NotZero(t, first.ID)
	require.NotZero(t, first.CreatedAt)
	assert.Equal(t, "Admin laptop", first.Name)
	assert.Nil(t, first.LastUsedAt)
	duplicate := newOperatorPasskey(operator.ID, handle)
	duplicate.CredentialID = first.CredentialID
	_, err = records.CreateOperatorPasskey(ctx, duplicate)
	require.Error(t, err, "a credential ID is registered once platform-wide")
	second, err := records.CreateOperatorPasskey(ctx, newOperatorPasskey(operator.ID, handle))
	require.NoError(t, err)
	foreign, err := records.CreateOperatorPasskey(ctx, newOperatorPasskey(other.ID, []byte("foreign-handle")))
	require.NoError(t, err)

	listed, err = records.ListActiveOperatorPasskeys(ctx, operator.ID)
	require.NoError(t, err)
	require.Len(t, listed, 2, "another operator's passkey is not listed")
	assert.Equal(t, first.ID, listed[0].ID, "the oldest registration comes first")
	assert.Equal(t, second.ID, listed[1].ID)
	assert.Equal(t, handle, listed[0].UserHandle)
	assert.JSONEq(t, `{"id":"registered"}`, string(listed[0].CredentialJSON))

	found, err := records.FindActiveOperatorPasskey(ctx, first.CredentialID, handle)
	require.NoError(t, err)
	assert.Equal(t, first.ID, found.ID)
	assert.Equal(t, operator.ID, found.OperatorID)
	_, err = records.FindActiveOperatorPasskey(ctx, foreign.CredentialID, handle)
	require.ErrorIs(t, err, identityaccess.ErrOperatorPasskeyNotFound, "a credential only matches its own user handle")
	_, err = records.FindActiveOperatorPasskey(ctx, []byte("unknown"), handle)
	require.ErrorIs(t, err, identityaccess.ErrOperatorPasskeyNotFound)

	usedAt := time.Now().Truncate(time.Microsecond)
	require.NoError(t, records.RecordOperatorPasskeyUse(ctx, first.ID, json.RawMessage(`{"id":"used"}`), usedAt))
	found, err = records.FindActiveOperatorPasskey(ctx, first.CredentialID, handle)
	require.NoError(t, err)
	require.NotNil(t, found.LastUsedAt)
	assert.True(t, usedAt.Equal(*found.LastUsedAt))
	assert.JSONEq(t, `{"id":"used"}`, string(found.CredentialJSON))
	require.EqualError(t, records.RecordOperatorPasskeyUse(ctx, first.ID, json.RawMessage(`{`), usedAt),
		"identity access: record operator passkey use: credential_json must be valid JSON")

	require.ErrorIs(t, records.RevokeOperatorPasskey(ctx, other.ID, first.ID, time.Now()), identityaccess.ErrOperatorPasskeyNotFound,
		"an operator cannot revoke another operator's passkey")
	require.NoError(t, records.RevokeOperatorPasskey(ctx, operator.ID, first.ID, time.Now()))
	require.ErrorIs(t, records.RevokeOperatorPasskey(ctx, operator.ID, first.ID, time.Now()), identityaccess.ErrOperatorPasskeyNotFound)
	_, err = records.FindActiveOperatorPasskey(ctx, first.CredentialID, handle)
	require.ErrorIs(t, err, identityaccess.ErrOperatorPasskeyNotFound, "a revoked passkey never verifies")
	require.ErrorIs(t, records.RecordOperatorPasskeyUse(ctx, first.ID, json.RawMessage(`{"id":"late"}`), time.Now()), identityaccess.ErrOperatorPasskeyNotFound,
		"a passkey revoked during a login is not updated")
	listed, err = records.ListActiveOperatorPasskeys(ctx, operator.ID)
	require.NoError(t, err)
	require.Len(t, listed, 1)
	assert.Equal(t, second.ID, listed[0].ID)
}

// A ceremony completes exactly once, only for its own purpose and only
// before it expires.
func TestOperatorPasskeySessionIsConsumedOnce(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	records := newOperatorPasskeyRecords(t, db)
	ctx := testpkg.Ctx(t)
	operator := testpkg.CreateTestOperator(t, db)

	invalid := newOperatorPasskeySession(t, db, &operator.ID, "recovery", time.Now().Add(time.Hour))
	_, err := records.CreateOperatorPasskeySession(ctx, invalid)
	require.EqualError(t, err, "identity access: create operator passkey session: unsupported operator passkey session purpose")
	invalid = newOperatorPasskeySession(t, db, &operator.ID, identityaccess.OperatorPasskeySessionPurposeLogin, time.Time{})
	_, err = records.CreateOperatorPasskeySession(ctx, invalid)
	require.EqualError(t, err, "identity access: create operator passkey session: expires_at is required")

	registration, err := records.CreateOperatorPasskeySession(ctx,
		newOperatorPasskeySession(t, db, &operator.ID, identityaccess.OperatorPasskeySessionPurposeRegistration, time.Now().Add(time.Hour)))
	require.NoError(t, err)
	require.NotZero(t, registration.CreatedAt)
	_, err = records.CreateOperatorPasskeySession(ctx, registration)
	require.Error(t, err, "a session ID is used once")

	_, err = records.ConsumeOperatorPasskeySession(ctx, registration.ID, identityaccess.OperatorPasskeySessionPurposeLogin, time.Now())
	require.ErrorIs(t, err, identityaccess.ErrOperatorPasskeySessionNotFound, "a registration ceremony cannot complete a login")
	consumedAt := time.Now().Truncate(time.Microsecond)
	consumed, err := records.ConsumeOperatorPasskeySession(ctx, registration.ID, identityaccess.OperatorPasskeySessionPurposeRegistration, consumedAt)
	require.NoError(t, err)
	require.NotNil(t, consumed.OperatorID)
	assert.Equal(t, operator.ID, *consumed.OperatorID)
	assert.Equal(t, "http://operator.localhost:3000", consumed.ExpectedOrigin)
	assert.Equal(t, "operator.localhost", consumed.RPID)
	assert.JSONEq(t, `{"challenge":"abc"}`, string(consumed.SessionJSON))
	require.NotNil(t, consumed.ConsumedAt)
	assert.True(t, consumedAt.Equal(*consumed.ConsumedAt))
	_, err = records.ConsumeOperatorPasskeySession(ctx, registration.ID, identityaccess.OperatorPasskeySessionPurposeRegistration, time.Now())
	require.ErrorIs(t, err, identityaccess.ErrOperatorPasskeySessionNotFound, "a replayed completion is refused")

	login, err := records.CreateOperatorPasskeySession(ctx,
		newOperatorPasskeySession(t, db, nil, identityaccess.OperatorPasskeySessionPurposeLogin, time.Now().Add(time.Hour)))
	require.NoError(t, err)
	consumed, err = records.ConsumeOperatorPasskeySession(ctx, login.ID, identityaccess.OperatorPasskeySessionPurposeLogin, time.Now())
	require.NoError(t, err)
	assert.Nil(t, consumed.OperatorID, "a discoverable login names no operator")

	expired, err := records.CreateOperatorPasskeySession(ctx,
		newOperatorPasskeySession(t, db, nil, identityaccess.OperatorPasskeySessionPurposeLogin, time.Now().Add(-time.Minute)))
	require.NoError(t, err)
	_, err = records.ConsumeOperatorPasskeySession(ctx, expired.ID, identityaccess.OperatorPasskeySessionPurposeLogin, time.Now())
	require.ErrorIs(t, err, identityaccess.ErrOperatorPasskeySessionNotFound, "an expired ceremony is refused")
	_, err = records.ConsumeOperatorPasskeySession(ctx, "unknown-"+expired.ID, identityaccess.OperatorPasskeySessionPurposeLogin, time.Now())
	require.ErrorIs(t, err, identityaccess.ErrOperatorPasskeySessionNotFound)
}

// The operator passkey tables carry no tenant column: a tenant context must
// neither hide nor reveal rows, because every other table this owner touches
// is tenant-scoped and a reader could assume the same here.
func TestOperatorPasskeyRecordsAreVisibleFromEveryTenantContext(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	records := newOperatorPasskeyRecords(t, db)
	operator := testpkg.CreateTestOperator(t, db)
	ctx := testpkg.Ctx(t)
	handle := []byte("handle-" + uuid.Must(uuid.NewV4()).String())
	credential, err := records.CreateOperatorPasskey(ctx, newOperatorPasskey(operator.ID, handle))
	require.NoError(t, err)

	first := testpkg.Tenant(t)
	second, _ := testpkg.CreateTestTenant(t, db)
	require.NotEqual(t, first, second)

	for name, tenantID := range map[string]int64{"own tenant": first, "foreign tenant": second} {
		t.Run(name, func(t *testing.T) {
			tenantCtx := testpkg.TenantContext(tenantID)
			listed, err := records.ListActiveOperatorPasskeys(tenantCtx, operator.ID)
			require.NoError(t, err)
			require.Len(t, listed, 1)
			found, err := records.FindActiveOperatorPasskey(tenantCtx, credential.CredentialID, handle)
			require.NoError(t, err)
			assert.Equal(t, credential.ID, found.ID)
			session, err := records.CreateOperatorPasskeySession(tenantCtx,
				newOperatorPasskeySession(t, db, &operator.ID, identityaccess.OperatorPasskeySessionPurposeRegistration, time.Now().Add(time.Hour)))
			require.NoError(t, err)
			_, err = records.ConsumeOperatorPasskeySession(ctx, session.ID, identityaccess.OperatorPasskeySessionPurposeRegistration, time.Now())
			require.NoError(t, err, "a ceremony started in one context completes in another")
		})
	}
}

// Completing a registration consumes the ceremony and stores the credential.
// A failure after either write inside one administrative transaction rolls
// both back, and a retry of the same ceremony then succeeds.
func TestOperatorPasskeyRegistrationRollsBackAfterEachWrite(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	records := newOperatorPasskeyRecords(t, db)
	operator := testpkg.CreateTestOperator(t, db)
	plain := testpkg.Ctx(t)
	ctx := testpkg.WithTenantRuntime(t, plain, db)
	handle := []byte("handle-" + uuid.Must(uuid.NewV4()).String())
	session, err := records.CreateOperatorPasskeySession(plain,
		newOperatorPasskeySession(t, db, &operator.ID, identityaccess.OperatorPasskeySessionPurposeRegistration, time.Now().Add(time.Hour)))
	require.NoError(t, err)
	credential := newOperatorPasskey(operator.ID, handle)

	writes := []func(context.Context) error{
		func(txCtx context.Context) error {
			_, err := records.ConsumeOperatorPasskeySession(txCtx, session.ID, identityaccess.OperatorPasskeySessionPurposeRegistration, time.Now())
			return err
		},
		func(txCtx context.Context) error {
			_, err := records.CreateOperatorPasskey(txCtx, credential)
			return err
		},
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
		listed, err := records.ListActiveOperatorPasskeys(plain, operator.ID)
		require.NoError(t, err)
		assert.Empty(t, listed, "the credential insert must be rolled back")
	}

	err = testpkg.WithAdminTx(t, ctx, db, func(txCtx context.Context, _ bun.Tx) error {
		for _, write := range writes {
			if err := write(txCtx); err != nil {
				return err
			}
		}
		return nil
	})
	require.NoError(t, err, "the retry finds the ceremony unconsumed")
	listed, err := records.ListActiveOperatorPasskeys(plain, operator.ID)
	require.NoError(t, err)
	require.Len(t, listed, 1)
	_, err = records.ConsumeOperatorPasskeySession(plain, session.ID, identityaccess.OperatorPasskeySessionPurposeRegistration, time.Now())
	require.ErrorIs(t, err, identityaccess.ErrOperatorPasskeySessionNotFound, "the committed ceremony is spent")
}

func TestOperatorPasskeyObservationsUseStableOperations(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	operator := testpkg.CreateTestOperator(t, db)
	var observations []identityCompose.Observation
	records, err := identityCompose.New(identityCompose.Dependencies{DB: db, Observe: func(observation identityCompose.Observation) {
		observations = append(observations, observation)
	}})
	require.NoError(t, err)
	ctx := testpkg.Ctx(t)
	handle := []byte("handle-" + uuid.Must(uuid.NewV4()).String())

	credential, err := records.CreateOperatorPasskey(ctx, newOperatorPasskey(operator.ID, handle))
	require.NoError(t, err)
	_, err = records.FindActiveOperatorPasskey(ctx, []byte("unknown"), handle)
	require.Error(t, err)
	require.NoError(t, records.RevokeOperatorPasskey(ctx, operator.ID, credential.ID, time.Now()))
	require.Error(t, records.RevokeOperatorPasskey(ctx, operator.ID, credential.ID, time.Now()))
	session, err := records.CreateOperatorPasskeySession(ctx,
		newOperatorPasskeySession(t, db, nil, identityaccess.OperatorPasskeySessionPurposeLogin, time.Now().Add(time.Hour)))
	require.NoError(t, err)
	_, err = records.ConsumeOperatorPasskeySession(ctx, session.ID, identityaccess.OperatorPasskeySessionPurposeLogin, time.Now())
	require.NoError(t, err)
	_, err = records.ConsumeOperatorPasskeySession(ctx, session.ID, identityaccess.OperatorPasskeySessionPurposeLogin, time.Now())
	require.Error(t, err)

	require.Len(t, observations, 7)
	expected := []struct {
		operation string
		rows      int64
		code      string
	}{
		{"create_operator_passkey", 1, "none"},
		{"find_active_operator_passkey", 0, "not_found"},
		{"revoke_operator_passkey", 1, "none"},
		{"revoke_operator_passkey", 0, "not_found"},
		{"create_operator_passkey_session", 1, "none"},
		{"consume_operator_passkey_session", 1, "none"},
		{"consume_operator_passkey_session", 0, "not_found"},
	}
	for index, want := range expected {
		observation := observations[index]
		assert.Equal(t, want.operation, observation.Operation)
		assert.Equal(t, want.rows, observation.Stats.Rows, want.operation)
		assert.Equal(t, want.code, identityaccess.ErrorCode(observation.Err), want.operation)
		assert.EqualValues(t, 1, observation.Stats.Queries, want.operation)
	}
}
