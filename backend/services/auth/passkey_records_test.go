package auth_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/gofrs/uuid"
	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/services"
	"github.com/moto-nrw/project-phoenix/services/auth"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func newTestPasskeyRecords(t *testing.T, db *bun.DB) auth.PasskeyRecords {
	t.Helper()
	records, err := services.NewPasskeyRecordsForTests(db)
	require.NoError(t, err)
	return records
}

// newTestPasskeyCredential registers a school-portal passkey through the
// root's binding.
func newTestPasskeyCredential(t *testing.T, db *bun.DB, accountID int64) *auth.PasskeyCredential {
	t.Helper()
	credential := &auth.PasskeyCredential{
		AccountID: accountID, UserHandle: []byte("handle-" + uuid.Must(uuid.NewV4()).String()),
		CredentialID: []byte("credential-" + uuid.Must(uuid.NewV4()).String()), CredentialJSON: json.RawMessage(`{}`),
	}
	require.NoError(t, newTestPasskeyRecords(t, db).CreateCredential(testpkg.Ctx(t), credential))
	return credential
}

// TestPasskeyRecords_RootBinding drives the root's binding over the real
// module: only the owner's not-found outcomes become a missing row or an
// unrevoked credential, and the stored identity and timestamps reach the
// caller's value.
func TestPasskeyRecords_RootBinding(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	records := newTestPasskeyRecords(t, db)
	account := testpkg.CreateTestAccount(t, db, "passkey-"+uuid.Must(uuid.NewV4()).String()+"@example.test")
	other := testpkg.CreateTestAccount(t, db, "passkey-"+uuid.Must(uuid.NewV4()).String()+"@example.test")
	credential := newTestPasskeyCredential(t, db, account.ID)
	require.NotZero(t, credential.ID)
	require.NotZero(t, credential.CreatedAt)

	found, err := records.FindActiveCredential(ctx, credential.CredentialID, credential.UserHandle)
	require.NoError(t, err)
	require.NotNil(t, found)
	assert.Equal(t, credential.ID, found.ID)
	missing, err := records.FindActiveCredential(ctx, []byte("unknown"), credential.UserHandle)
	require.NoError(t, err)
	assert.Nil(t, missing)

	revoked, err := records.RevokeCredential(ctx, other.ID, credential.ID, time.Now())
	require.NoError(t, err)
	assert.False(t, revoked, "another account's passkey is not revoked")
	revoked, err = records.RevokeCredential(ctx, account.ID, credential.ID, time.Now())
	require.NoError(t, err)
	assert.True(t, revoked)
	listed, err := records.ListActiveCredentials(ctx, account.ID)
	require.NoError(t, err)
	assert.Empty(t, listed)

	tenantID := testpkg.Tenant(t)
	session := &auth.PasskeySession{
		AccountID: &account.ID, TenantID: &tenantID, Purpose: auth.PasskeySessionPurposeRegistration,
		RPID: "school.localhost", ExpectedOrigin: "http://school.localhost:3000",
		SessionJSON: json.RawMessage(`{"challenge":"abc"}`), ExpiresAt: time.Now().Add(time.Hour),
	}
	session.ID = "passkey-" + uuid.Must(uuid.NewV4()).String()
	testpkg.OwnTestAccountPasskeySession(t, db, session.ID)
	require.NoError(t, records.CreateSession(ctx, session))
	require.NotZero(t, session.CreatedAt)
	consumed, err := records.ConsumeSession(ctx, session.ID, auth.PasskeySessionPurposeRegistration, time.Now())
	require.NoError(t, err)
	require.NotNil(t, consumed)
	require.NotNil(t, consumed.ConsumedAt)
	replayed, err := records.ConsumeSession(ctx, session.ID, auth.PasskeySessionPurposeRegistration, time.Now())
	require.NoError(t, err)
	assert.Nil(t, replayed, "a spent ceremony is a missing row")
}

// TestPasskeyRecords_StoreFailureIsNotAMissingRow closes the connection
// under the root's binding: a store failure must stay an error on every call
// whose missing-row outcome the passkey flow turns into a client error.
func TestPasskeyRecords_StoreFailureIsNotAMissingRow(t *testing.T) {
	t.Parallel()

	ctx := testpkg.Ctx(t)
	live := testpkg.SetupTestDB(t)
	account := testpkg.CreateTestAccount(t, live, "passkey-"+uuid.Must(uuid.NewV4()).String()+"@example.test")
	registered := newTestPasskeyCredential(t, live, account.ID)
	db := testpkg.SetupClosableTestDB(t)
	records := newTestPasskeyRecords(t, db)
	require.NoError(t, db.Close())

	credential, err := records.FindActiveCredential(ctx, registered.CredentialID, registered.UserHandle)
	require.Error(t, err)
	assert.Nil(t, credential)
	revoked, err := records.RevokeCredential(ctx, account.ID, registered.ID, time.Now())
	require.Error(t, err)
	assert.False(t, revoked)
	session, err := records.ConsumeSession(ctx, "session-id", auth.PasskeySessionPurposeLogin, time.Now())
	require.Error(t, err)
	assert.Nil(t, session)
}

// failingPasskeyLookupRecords serves the real records but fails the
// credential lookup like an unavailable store.
type failingPasskeyLookupRecords struct {
	auth.PasskeyRecords
	err error
}

func (r failingPasskeyLookupRecords) FindActiveCredential(context.Context, []byte, []byte) (*auth.PasskeyCredential, error) {
	return nil, r.err
}

// TestPasskeyService_LoginCompletionTransaction drives FinishLogin with the
// unit of work the API root attaches: a failed credential lookup rolls the
// ceremony consumption back, so the same ceremony can be completed again,
// and a refused assertion commits it, so it cannot be tried a third time.
func TestPasskeyService_LoginCompletionTransaction(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	ctx := testpkg.WithTenantRuntime(t, testpkg.Ctx(t), db)
	tenantID := testpkg.Tenant(t)
	account := testpkg.CreateTestAccount(t, db, "passkey-"+uuid.Must(uuid.NewV4()).String()+"@example.test")
	registered := newTestPasskeyCredential(t, db, account.ID)
	records := newTestPasskeyRecords(t, db)
	storeDown := errors.New("connection reset")
	newService := func(records auth.PasskeyRecords) auth.PasskeyService {
		t.Helper()
		svc, err := auth.NewPasskeyService(auth.PasskeyServiceConfig{
			Repos: repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db)), Records: records, MFAService: &auth.MFAStub{}, AuthService: &auth.Service{},
			DB: db, RPID: "localhost", TenantDomain: "localhost",
		})
		require.NoError(t, err)
		return svc
	}

	assertion, err := newService(records).BeginLogin(ctx, auth.PasskeyLoginStartRequest{
		TenantID: tenantID, TenantSubdomain: "school", ExpectedOrigin: "http://school.localhost:3000",
	})
	require.NoError(t, err)
	testpkg.OwnTestAccountPasskeySession(t, db, assertion.SessionID)
	request := auth.PasskeyLoginFinishRequest{
		SessionID:          assertion.SessionID,
		CredentialResponse: auth.NewPasskeyAssertionForTests(t, registered.CredentialID, registered.UserHandle),
	}

	_, err = newService(failingPasskeyLookupRecords{PasskeyRecords: records, err: storeDown}).FinishLogin(ctx, request)
	require.ErrorIs(t, err, storeDown)

	_, err = newService(records).FinishLogin(ctx, request)
	require.ErrorIs(t, err, auth.ErrInvalidCredentials, "the rolled-back ceremony reaches verification again")

	_, err = newService(records).FinishLogin(ctx, request)
	require.ErrorIs(t, err, auth.ErrPasskeySessionInvalid, "the refused ceremony stays spent")
	listed, err := records.ListActiveCredentials(ctx, account.ID)
	require.NoError(t, err)
	require.Len(t, listed, 1)
	assert.Nil(t, listed[0].LastUsedAt, "a refused assertion records no use")
}
