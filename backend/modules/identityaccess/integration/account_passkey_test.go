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

func newAccountPasskey(accountID int64, userHandle []byte) identityaccess.AccountPasskeyCredential {
	return identityaccess.AccountPasskeyCredential{
		AccountID: accountID, UserHandle: userHandle, CredentialID: []byte("credential-" + uuid.Must(uuid.NewV4()).String()),
		CredentialJSON: json.RawMessage(`{"id":"registered"}`), Name: "Laptop",
	}
}

// newAccountPasskeySession builds a ceremony whose row the test owns.
func newAccountPasskeySession(t *testing.T, db *bun.DB, accountID, tenantID *int64, purpose string, expiresAt time.Time) identityaccess.AccountPasskeySession {
	t.Helper()
	id := "passkey-" + uuid.Must(uuid.NewV4()).String()
	testpkg.OwnTestAccountPasskeySession(t, db, id)
	return identityaccess.AccountPasskeySession{
		ID: id, AccountID: accountID, TenantID: tenantID, Purpose: purpose,
		RPID: "school.localhost", ExpectedOrigin: "http://school.localhost:3000",
		SessionJSON: json.RawMessage(`{"challenge":"abc"}`), ExpiresAt: expiresAt,
	}
}

func TestAccountPasskeyCredentialLifecycle(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	records := newOperatorPasskeyRecords(t, db)
	ctx := testpkg.Ctx(t)
	account := testpkg.CreateTestAccount(t, db, "passkey-"+uuid.Must(uuid.NewV4()).String()+"@example.test")
	other := testpkg.CreateTestAccount(t, db, "passkey-"+uuid.Must(uuid.NewV4()).String()+"@example.test")
	handle := []byte("handle-" + uuid.Must(uuid.NewV4()).String())

	invalid := newAccountPasskey(account.ID, handle)
	invalid.CredentialJSON = json.RawMessage(`{`)
	_, err := records.CreateAccountPasskey(ctx, invalid)
	require.EqualError(t, err, "identity access: create account passkey: credential_json must be valid JSON")
	_, err = records.CreateAccountPasskey(ctx, newAccountPasskey(0, handle))
	require.EqualError(t, err, "identity access: create account passkey: account_id is required")

	first, err := records.CreateAccountPasskey(ctx, newAccountPasskey(account.ID, handle))
	require.NoError(t, err)
	require.NotZero(t, first.ID)
	require.NotZero(t, first.CreatedAt)
	assert.Equal(t, "Laptop", first.Name)
	duplicate := newAccountPasskey(account.ID, handle)
	duplicate.CredentialID = first.CredentialID
	_, err = records.CreateAccountPasskey(ctx, duplicate)
	require.Error(t, err, "a credential ID is registered once platform-wide")
	second, err := records.CreateAccountPasskey(ctx, newAccountPasskey(account.ID, handle))
	require.NoError(t, err)
	foreign, err := records.CreateAccountPasskey(ctx, newAccountPasskey(other.ID, []byte("foreign-handle")))
	require.NoError(t, err)

	listed, err := records.ListActiveAccountPasskeys(ctx, account.ID)
	require.NoError(t, err)
	require.Len(t, listed, 2, "another account's passkey is not listed")
	assert.Equal(t, first.ID, listed[0].ID, "the oldest registration comes first")
	assert.Equal(t, second.ID, listed[1].ID)

	found, err := records.FindActiveAccountPasskey(ctx, first.CredentialID, handle)
	require.NoError(t, err)
	assert.Equal(t, first.ID, found.ID)
	assert.Equal(t, account.ID, found.AccountID)
	_, err = records.FindActiveAccountPasskey(ctx, foreign.CredentialID, handle)
	require.ErrorIs(t, err, identityaccess.ErrAccountPasskeyNotFound, "a credential only matches its own user handle")

	usedAt := time.Now().Truncate(time.Microsecond)
	require.NoError(t, records.RecordAccountPasskeyUse(ctx, first.ID, json.RawMessage(`{"id":"used"}`), usedAt))
	found, err = records.FindActiveAccountPasskey(ctx, first.CredentialID, handle)
	require.NoError(t, err)
	require.NotNil(t, found.LastUsedAt)
	assert.True(t, usedAt.Equal(*found.LastUsedAt))
	assert.JSONEq(t, `{"id":"used"}`, string(found.CredentialJSON))

	require.ErrorIs(t, records.RevokeAccountPasskey(ctx, other.ID, first.ID, time.Now()), identityaccess.ErrAccountPasskeyNotFound,
		"an account cannot revoke another account's passkey")
	require.NoError(t, records.RevokeAccountPasskey(ctx, account.ID, first.ID, time.Now()))
	require.ErrorIs(t, records.RevokeAccountPasskey(ctx, account.ID, first.ID, time.Now()), identityaccess.ErrAccountPasskeyNotFound)
	_, err = records.FindActiveAccountPasskey(ctx, first.CredentialID, handle)
	require.ErrorIs(t, err, identityaccess.ErrAccountPasskeyNotFound, "a revoked passkey never verifies")
	require.ErrorIs(t, records.RecordAccountPasskeyUse(ctx, first.ID, json.RawMessage(`{"id":"late"}`), time.Now()), identityaccess.ErrAccountPasskeyNotFound,
		"a passkey revoked during a login is not updated")
}

// A ceremony completes exactly once, only for its own purpose and only
// before it expires.
func TestAccountPasskeySessionIsConsumedOnce(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	records := newOperatorPasskeyRecords(t, db)
	ctx := testpkg.Ctx(t)
	account := testpkg.CreateTestAccount(t, db, "passkey-"+uuid.Must(uuid.NewV4()).String()+"@example.test")
	tenantID := testpkg.Tenant(t)

	invalid := newAccountPasskeySession(t, db, &account.ID, &tenantID, "recovery", time.Now().Add(time.Hour))
	_, err := records.CreateAccountPasskeySession(ctx, invalid)
	require.EqualError(t, err, "identity access: create account passkey session: unsupported passkey session purpose")

	registration, err := records.CreateAccountPasskeySession(ctx,
		newAccountPasskeySession(t, db, &account.ID, &tenantID, identityaccess.PasskeySessionPurposeRegistration, time.Now().Add(time.Hour)))
	require.NoError(t, err)
	require.NotZero(t, registration.CreatedAt)

	_, err = records.ConsumeAccountPasskeySession(ctx, registration.ID, identityaccess.PasskeySessionPurposeLogin, time.Now())
	require.ErrorIs(t, err, identityaccess.ErrAccountPasskeySessionNotFound, "a registration ceremony cannot complete a login")
	consumedAt := time.Now().Truncate(time.Microsecond)
	consumed, err := records.ConsumeAccountPasskeySession(ctx, registration.ID, identityaccess.PasskeySessionPurposeRegistration, consumedAt)
	require.NoError(t, err)
	require.NotNil(t, consumed.AccountID)
	require.NotNil(t, consumed.TenantID)
	assert.Equal(t, account.ID, *consumed.AccountID)
	assert.Equal(t, tenantID, *consumed.TenantID, "the ceremony keeps the school whose portal started it")
	require.NotNil(t, consumed.ConsumedAt)
	assert.True(t, consumedAt.Equal(*consumed.ConsumedAt))
	_, err = records.ConsumeAccountPasskeySession(ctx, registration.ID, identityaccess.PasskeySessionPurposeRegistration, time.Now())
	require.ErrorIs(t, err, identityaccess.ErrAccountPasskeySessionNotFound, "a replayed completion is refused")

	login, err := records.CreateAccountPasskeySession(ctx,
		newAccountPasskeySession(t, db, nil, &tenantID, identityaccess.PasskeySessionPurposeLogin, time.Now().Add(time.Hour)))
	require.NoError(t, err)
	consumed, err = records.ConsumeAccountPasskeySession(ctx, login.ID, identityaccess.PasskeySessionPurposeLogin, time.Now())
	require.NoError(t, err)
	assert.Nil(t, consumed.AccountID, "a discoverable login names no account")

	expired, err := records.CreateAccountPasskeySession(ctx,
		newAccountPasskeySession(t, db, nil, &tenantID, identityaccess.PasskeySessionPurposeLogin, time.Now().Add(-time.Minute)))
	require.NoError(t, err)
	_, err = records.ConsumeAccountPasskeySession(ctx, expired.ID, identityaccess.PasskeySessionPurposeLogin, time.Now())
	require.ErrorIs(t, err, identityaccess.ErrAccountPasskeySessionNotFound, "an expired ceremony is refused")
}

// A passkey belongs to the account, not to a school: a tenant context must
// neither hide nor reveal one, and a ceremony started at one school is
// completed with its own tenant recorded. The login flow decides the school
// binding from that row, not from the connection.
func TestAccountPasskeyRecordsAreVisibleFromEveryTenantContext(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	records := newOperatorPasskeyRecords(t, db)
	ctx := testpkg.Ctx(t)
	account := testpkg.CreateTestAccount(t, db, "passkey-"+uuid.Must(uuid.NewV4()).String()+"@example.test")
	handle := []byte("handle-" + uuid.Must(uuid.NewV4()).String())
	credential, err := records.CreateAccountPasskey(ctx, newAccountPasskey(account.ID, handle))
	require.NoError(t, err)

	first := testpkg.Tenant(t)
	second, _ := testpkg.CreateTestTenant(t, db)
	require.NotEqual(t, first, second)

	for name, tenantID := range map[string]int64{"own tenant": first, "foreign tenant": second} {
		t.Run(name, func(t *testing.T) {
			tenantCtx := testpkg.TenantContext(tenantID)
			listed, err := records.ListActiveAccountPasskeys(tenantCtx, account.ID)
			require.NoError(t, err)
			require.Len(t, listed, 1)
			found, err := records.FindActiveAccountPasskey(tenantCtx, credential.CredentialID, handle)
			require.NoError(t, err)
			assert.Equal(t, credential.ID, found.ID)
			started := tenantID
			session, err := records.CreateAccountPasskeySession(tenantCtx,
				newAccountPasskeySession(t, db, &account.ID, &started, identityaccess.PasskeySessionPurposeLogin, time.Now().Add(time.Hour)))
			require.NoError(t, err)
			consumed, err := records.ConsumeAccountPasskeySession(testpkg.TenantContext(first+second-tenantID), session.ID,
				identityaccess.PasskeySessionPurposeLogin, time.Now())
			require.NoError(t, err, "a ceremony started in one school completes in the other's context")
			require.NotNil(t, consumed.TenantID)
			assert.Equal(t, started, *consumed.TenantID, "the recorded school is the one that started the ceremony")
		})
	}
}

// Completing a registration consumes the ceremony and stores the credential.
// A failure after either write inside one administrative transaction rolls
// both back, and a retry of the same ceremony then succeeds.
func TestAccountPasskeyRegistrationRollsBackAfterEachWrite(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	records := newOperatorPasskeyRecords(t, db)
	account := testpkg.CreateTestAccount(t, db, "passkey-"+uuid.Must(uuid.NewV4()).String()+"@example.test")
	plain := testpkg.Ctx(t)
	ctx := testpkg.WithTenantRuntime(t, plain, db)
	tenantID := testpkg.Tenant(t)
	handle := []byte("handle-" + uuid.Must(uuid.NewV4()).String())
	session, err := records.CreateAccountPasskeySession(plain,
		newAccountPasskeySession(t, db, &account.ID, &tenantID, identityaccess.PasskeySessionPurposeRegistration, time.Now().Add(time.Hour)))
	require.NoError(t, err)
	credential := newAccountPasskey(account.ID, handle)

	writes := []func(context.Context) error{
		func(txCtx context.Context) error {
			_, err := records.ConsumeAccountPasskeySession(txCtx, session.ID, identityaccess.PasskeySessionPurposeRegistration, time.Now())
			return err
		},
		func(txCtx context.Context) error {
			_, err := records.CreateAccountPasskey(txCtx, credential)
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
		listed, err := records.ListActiveAccountPasskeys(plain, account.ID)
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
	listed, err := records.ListActiveAccountPasskeys(plain, account.ID)
	require.NoError(t, err)
	require.Len(t, listed, 1)
	_, err = records.ConsumeAccountPasskeySession(plain, session.ID, identityaccess.PasskeySessionPurposeRegistration, time.Now())
	require.ErrorIs(t, err, identityaccess.ErrAccountPasskeySessionNotFound, "the committed ceremony is spent")
}

func TestAccountPasskeyObservationsUseStableOperations(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	account := testpkg.CreateTestAccount(t, db, "passkey-"+uuid.Must(uuid.NewV4()).String()+"@example.test")
	var observations []identityCompose.Observation
	records, err := identityCompose.New(identityCompose.Dependencies{DB: db, Observe: func(observation identityCompose.Observation) {
		observations = append(observations, observation)
	}})
	require.NoError(t, err)
	ctx := testpkg.Ctx(t)
	tenantID := testpkg.Tenant(t)
	handle := []byte("handle-" + uuid.Must(uuid.NewV4()).String())

	credential, err := records.CreateAccountPasskey(ctx, newAccountPasskey(account.ID, handle))
	require.NoError(t, err)
	_, err = records.FindActiveAccountPasskey(ctx, []byte("unknown"), handle)
	require.Error(t, err)
	require.NoError(t, records.RevokeAccountPasskey(ctx, account.ID, credential.ID, time.Now()))
	session, err := records.CreateAccountPasskeySession(ctx,
		newAccountPasskeySession(t, db, nil, &tenantID, identityaccess.PasskeySessionPurposeLogin, time.Now().Add(time.Hour)))
	require.NoError(t, err)
	_, err = records.ConsumeAccountPasskeySession(ctx, session.ID, identityaccess.PasskeySessionPurposeLogin, time.Now())
	require.NoError(t, err)
	_, err = records.ConsumeAccountPasskeySession(ctx, session.ID, identityaccess.PasskeySessionPurposeLogin, time.Now())
	require.Error(t, err)

	require.Len(t, observations, 6)
	expected := []struct {
		operation string
		rows      int64
		code      string
	}{
		{"create_account_passkey", 1, "none"},
		{"find_active_account_passkey", 0, "not_found"},
		{"revoke_account_passkey", 1, "none"},
		{"create_account_passkey_session", 1, "none"},
		{"consume_account_passkey_session", 1, "none"},
		{"consume_account_passkey_session", 0, "not_found"},
	}
	for index, want := range expected {
		observation := observations[index]
		assert.Equal(t, want.operation, observation.Operation)
		assert.Equal(t, want.rows, observation.Stats.Rows, want.operation)
		assert.Equal(t, want.code, identityaccess.ErrorCode(observation.Err), want.operation)
		assert.EqualValues(t, 1, observation.Stats.Queries, want.operation)
	}
}
