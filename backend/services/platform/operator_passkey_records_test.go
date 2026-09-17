package platform_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/gofrs/uuid"
	platformModels "github.com/moto-nrw/project-phoenix/models/platform"
	"github.com/moto-nrw/project-phoenix/services"
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

	ctx := context.Background()
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

// TestOperatorPasskeyRecords_StoreFailureIsNotAMissingRow closes the
// connection under the root's binding: a store failure must stay an error on
// every call whose missing-row outcome the passkey flow turns into a
// client error.
func TestOperatorPasskeyRecords_StoreFailureIsNotAMissingRow(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	live := testpkg.SetupTestDB(t)
	operator := testpkg.CreateTestOperator(t, live)
	db := testpkg.SetupClosableTestDB(t)
	records := newTestOperatorPasskeyRecords(t, db)
	require.NoError(t, db.Close())

	credential, err := records.FindActiveCredential(ctx, []byte("credential"), []byte("handle"))
	require.Error(t, err)
	assert.Nil(t, credential)
	revoked, err := records.RevokeCredential(ctx, operator.ID, 1, time.Now())
	require.Error(t, err)
	assert.False(t, revoked)
	session, err := records.ConsumeSession(ctx, "session-id", platformModels.OperatorPasskeySessionPurposeLogin, time.Now())
	require.Error(t, err)
	assert.Nil(t, session)
}
