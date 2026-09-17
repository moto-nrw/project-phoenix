package platform_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/gofrs/uuid"
	platformModels "github.com/moto-nrw/project-phoenix/models/platform"
	"github.com/moto-nrw/project-phoenix/services"
	authService "github.com/moto-nrw/project-phoenix/services/auth"
	"github.com/moto-nrw/project-phoenix/services/platform"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// newTestOperatorPasskeyRecords binds the retained operator passkey records
// port to the Identity & Access module the way the service root does.
func newTestOperatorPasskeyRecords(t *testing.T, db *bun.DB) platform.OperatorPasskeyRecords {
	t.Helper()
	records, err := services.NewOperatorPasskeyRecordsForTests(db)
	require.NoError(t, err)
	return records
}

// TestOperatorPasskeyRecords_RootBinding drives the root's binding over the
// real module: only the owner's not-found outcomes become a missing row or
// an unrevoked credential, and the stored identity and timestamps reach the
// caller's value.
func TestOperatorPasskeyRecords_RootBinding(t *testing.T) {
	t.Parallel()

	ctx := testpkg.Ctx(t)
	db := testpkg.SetupTestDB(t)
	records := newTestOperatorPasskeyRecords(t, db)
	operator := testpkg.CreateTestOperator(t, db)
	other := testpkg.CreateTestOperator(t, db)
	handle := []byte("handle-" + uuid.Must(uuid.NewV4()).String())

	credential := &platformModels.OperatorPasskeyCredential{
		OperatorID: operator.ID, UserHandle: handle, CredentialID: []byte("credential-" + uuid.Must(uuid.NewV4()).String()),
		CredentialJSON: json.RawMessage(`{"id":"registered"}`), Name: "Admin laptop",
	}
	require.NoError(t, records.CreateCredential(ctx, credential))
	require.NotZero(t, credential.ID)
	require.NotZero(t, credential.CreatedAt)

	found, err := records.FindActiveCredential(ctx, credential.CredentialID, handle)
	require.NoError(t, err)
	require.NotNil(t, found)
	assert.Equal(t, credential.ID, found.ID)
	missing, err := records.FindActiveCredential(ctx, []byte("unknown"), handle)
	require.NoError(t, err)
	assert.Nil(t, missing)

	revoked, err := records.RevokeCredential(ctx, other.ID, credential.ID, time.Now())
	require.NoError(t, err)
	assert.False(t, revoked, "another operator's passkey is not revoked")
	revoked, err = records.RevokeCredential(ctx, operator.ID, credential.ID, time.Now())
	require.NoError(t, err)
	assert.True(t, revoked)
	listed, err := records.ListActiveCredentials(ctx, operator.ID)
	require.NoError(t, err)
	assert.Empty(t, listed)

	session := &platformModels.OperatorPasskeySession{
		OperatorID: &operator.ID, Purpose: platformModels.OperatorPasskeySessionPurposeRegistration,
		RPID: "operator.localhost", ExpectedOrigin: "http://operator.localhost:3000",
		SessionJSON: json.RawMessage(`{"challenge":"abc"}`), ExpiresAt: time.Now().Add(time.Hour),
	}
	session.ID = "operator-passkey-" + uuid.Must(uuid.NewV4()).String()
	require.NoError(t, records.CreateSession(ctx, session))
	require.NotZero(t, session.CreatedAt)
	consumed, err := records.ConsumeSession(ctx, session.ID, platformModels.OperatorPasskeySessionPurposeRegistration, time.Now())
	require.NoError(t, err)
	require.NotNil(t, consumed)
	require.NotNil(t, consumed.ConsumedAt)
	replayed, err := records.ConsumeSession(ctx, session.ID, platformModels.OperatorPasskeySessionPurposeRegistration, time.Now())
	require.NoError(t, err)
	assert.Nil(t, replayed, "a spent ceremony is a missing row")
}

// newTestOperatorPasskeyCredential registers a passkey through the root's
// binding.
func newTestOperatorPasskeyCredential(t *testing.T, db *bun.DB, operatorID int64) *platformModels.OperatorPasskeyCredential {
	t.Helper()
	credential := &platformModels.OperatorPasskeyCredential{
		OperatorID: operatorID, UserHandle: []byte("handle-" + uuid.Must(uuid.NewV4()).String()),
		CredentialID: []byte("credential-" + uuid.Must(uuid.NewV4()).String()), CredentialJSON: json.RawMessage(`{}`),
	}
	require.NoError(t, newTestOperatorPasskeyRecords(t, db).CreateCredential(testpkg.Ctx(t), credential))
	return credential
}

// TestOperatorPasskeyRecords_StoreFailureIsNotAMissingRow closes the
// connection under the root's binding: a store failure must stay an error on
// every call whose missing-row outcome the passkey flow turns into a
// client error.
func TestOperatorPasskeyRecords_StoreFailureIsNotAMissingRow(t *testing.T) {
	t.Parallel()

	ctx := testpkg.Ctx(t)
	live := testpkg.SetupTestDB(t)
	operator := testpkg.CreateTestOperator(t, live)
	registered := newTestOperatorPasskeyCredential(t, live, operator.ID)
	db := testpkg.SetupClosableTestDB(t)
	records := newTestOperatorPasskeyRecords(t, db)
	require.NoError(t, db.Close())

	credential, err := records.FindActiveCredential(ctx, registered.CredentialID, registered.UserHandle)
	require.Error(t, err)
	assert.Nil(t, credential)
	revoked, err := records.RevokeCredential(ctx, operator.ID, registered.ID, time.Now())
	require.Error(t, err)
	assert.False(t, revoked)
	session, err := records.ConsumeSession(ctx, "session-id", platformModels.OperatorPasskeySessionPurposeLogin, time.Now())
	require.Error(t, err)
	assert.Nil(t, session)
}

// failingPasskeyLookupRecords serves the real records but fails the
// credential lookup like an unavailable store.
type failingPasskeyLookupRecords struct {
	platform.OperatorPasskeyRecords
	err error
}

func (r failingPasskeyLookupRecords) FindActiveCredential(context.Context, []byte, []byte) (*platformModels.OperatorPasskeyCredential, error) {
	return nil, r.err
}

// unusedOperatorSessions satisfies the token exchange dependency of a login
// that never gets that far.
type unusedOperatorSessions struct {
	platform.OperatorAuthService
}

// TestOperatorPasskeyService_LoginCompletionTransaction drives FinishLogin
// with the unit of work the API root attaches: a failed credential lookup
// rolls the ceremony consumption back, so the same ceremony can be completed
// again, and a refused assertion commits it, so it cannot be tried a third
// time.
func TestOperatorPasskeyService_LoginCompletionTransaction(t *testing.T) {
	t.Parallel()

	mfa, _, db := newTestOperatorMFAService(t)
	ctx := testpkg.WithTenantRuntime(t, testpkg.Ctx(t), db)
	operator := testpkg.CreateTestOperator(t, db)
	registered := newTestOperatorPasskeyCredential(t, db, operator.ID)
	records := newTestOperatorPasskeyRecords(t, db)
	storeDown := errors.New("connection reset")
	newService := func(records platform.OperatorPasskeyRecords) platform.OperatorPasskeyService {
		t.Helper()
		svc, err := platform.NewOperatorPasskeyService(platform.OperatorPasskeyServiceConfig{
			Records: records, Operators: newTestOperatorDirectory(db), MFAService: mfa, AuthService: unusedOperatorSessions{},
			DB: db, RPID: "operator.localhost", OperatorFrontendURL: "http://operator.localhost:3000",
		})
		require.NoError(t, err)
		return svc
	}

	assertion, err := newService(records).BeginLogin(ctx, "http://operator.localhost:3000")
	require.NoError(t, err)
	testpkg.OwnTestOperatorPasskeySession(t, db, assertion.SessionID)
	request := platform.OperatorPasskeyLoginFinishRequest{
		SessionID:          assertion.SessionID,
		CredentialResponse: platform.NewOperatorPasskeyAssertionForTests(t, registered.CredentialID, registered.UserHandle),
	}

	_, err = newService(failingPasskeyLookupRecords{OperatorPasskeyRecords: records, err: storeDown}).FinishLogin(ctx, request)
	require.ErrorIs(t, err, storeDown)

	_, err = newService(records).FinishLogin(ctx, request)
	var invalid *platform.InvalidCredentialsError
	require.ErrorAs(t, err, &invalid, "the rolled-back ceremony reaches verification again")

	_, err = newService(records).FinishLogin(ctx, request)
	require.ErrorIs(t, err, authService.ErrPasskeySessionInvalid, "the refused ceremony stays spent")
	listed, err := records.ListActiveCredentials(ctx, operator.ID)
	require.NoError(t, err)
	require.Len(t, listed, 1)
	assert.Nil(t, listed[0].LastUsedAt, "a refused assertion records no use")
}
