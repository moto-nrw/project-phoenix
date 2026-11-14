package postgres

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/gofrs/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-ogs/project-phoenix/modules/identityaccess/internal/domain"
	testpkg "github.com/moto-ogs/project-phoenix/test"
)

// The ceremonies trust the store for what "active" means: a credential
// answers only under its own user handle and only until it is revoked, and a
// use or a revocation of a credential that is gone changes nothing. Each
// statement reports the rows it wrote.
func TestAccountPasskeyStoreServesActiveCredentialsOnly(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	store := NewStore(db)
	holder, other := testpkg.CreateTestAccount(t, db, "passkey-store").ID, testpkg.CreateTestAccount(t, db, "passkey-store").ID
	handle := []byte("handle-" + uuid.Must(uuid.NewV4()).String())
	credentialID := []byte("credential-" + uuid.Must(uuid.NewV4()).String())
	var stored domain.AccountPasskeyCredential

	inAdmin := func(fn func(ctx context.Context)) {
		t.Helper()
		require.NoError(t, testpkg.WithinAdminContext(t, context.Background(), db, func(ctx context.Context) error {
			fn(ctx)
			return nil
		}))
	}
	find := func(id, userHandle []byte) bool {
		var found bool
		inAdmin(func(ctx context.Context) {
			var err error
			_, found, _, err = store.FindActiveAccountPasskey(ctx, id, userHandle)
			require.NoError(t, err)
		})
		return found
	}

	inAdmin(func(ctx context.Context) {
		var err error
		var stats domain.OperationStats
		stored, stats, err = store.InsertAccountPasskey(ctx, domain.AccountPasskeyCredential{
			AccountID: holder, UserHandle: handle, CredentialID: credentialID, CredentialJSON: json.RawMessage(`{"id":"registered"}`), Name: "Laptop",
		})
		require.NoError(t, err)
		assert.EqualValues(t, 1, stats.Rows)
	})
	assert.True(t, find(credentialID, handle))
	assert.False(t, find(credentialID, []byte("another-handle")), "a credential only matches its own user handle")
	assert.False(t, find([]byte("unknown"), handle))

	inAdmin(func(ctx context.Context) {
		changed, stats, err := store.RevokeAccountPasskey(ctx, other, stored.ID, time.Now())
		require.NoError(t, err)
		assert.False(t, changed, "another holder revokes nothing")
		assert.Zero(t, stats.Rows)
		changed, stats, err = store.UpdateAccountPasskeyAfterUse(ctx, stored.ID, []byte(`{"id":"used"}`), time.Now())
		require.NoError(t, err)
		assert.True(t, changed)
		assert.EqualValues(t, 1, stats.Rows)
		changed, stats, err = store.RevokeAccountPasskey(ctx, holder, stored.ID, time.Now())
		require.NoError(t, err)
		assert.True(t, changed)
		assert.EqualValues(t, 1, stats.Rows)
	})
	assert.False(t, find(credentialID, handle), "a revoked passkey is not active")
	inAdmin(func(ctx context.Context) {
		changed, stats, err := store.UpdateAccountPasskeyAfterUse(ctx, stored.ID, []byte(`{"id":"late"}`), time.Now())
		require.NoError(t, err)
		assert.False(t, changed, "a revoked passkey is not updated")
		assert.Zero(t, stats.Rows)
		changed, _, err = store.RevokeAccountPasskey(ctx, holder, stored.ID, time.Now())
		require.NoError(t, err)
		assert.False(t, changed, "a passkey is revoked once")
		listed, _, err := store.ListActiveAccountPasskeys(ctx, holder)
		require.NoError(t, err)
		assert.Empty(t, listed)
	})
}

// A ceremony row is written once and consumed once, and each statement
// reports the rows it wrote.
func TestAccountPasskeyStoreConsumesACeremonyOnce(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	store := NewStore(db)
	holder := testpkg.CreateTestAccount(t, db, "passkey-store").ID
	session := domain.AccountPasskeySession{
		ID: "passkey-" + uuid.Must(uuid.NewV4()).String(), AccountID: &holder, Purpose: domain.PasskeySessionPurposeRegistration,
		RPID: "localhost", ExpectedOrigin: "http://localhost", SessionJSON: json.RawMessage(`{"challenge":"abc"}`), ExpiresAt: time.Now().Add(time.Hour),
	}

	testpkg.OwnTestAccountPasskeySession(t, db, session.ID)
	require.NoError(t, testpkg.WithinAdminContext(t, context.Background(), db, func(ctx context.Context) error {
		_, stats, err := store.InsertAccountPasskeySession(ctx, session)
		require.NoError(t, err)
		assert.EqualValues(t, 1, stats.Rows)
		consumed, found, stats, err := store.ConsumeAccountPasskeySession(ctx, session.ID, session.Purpose, time.Now())
		require.NoError(t, err)
		require.True(t, found)
		assert.EqualValues(t, 1, stats.Rows)
		assert.Equal(t, "http://localhost", consumed.ExpectedOrigin)
		assert.Equal(t, "localhost", consumed.RPID)
		assert.JSONEq(t, `{"challenge":"abc"}`, string(consumed.SessionJSON))
		require.NotNil(t, consumed.AccountID)
		assert.Equal(t, holder, *consumed.AccountID)
		_, found, stats, err = store.ConsumeAccountPasskeySession(ctx, session.ID, session.Purpose, time.Now())
		require.NoError(t, err)
		assert.False(t, found, "a ceremony is consumed once")
		assert.Zero(t, stats.Rows)
		return nil
	}))
	err := testpkg.WithinAdminContext(t, context.Background(), db, func(ctx context.Context) error {
		_, _, err := store.InsertAccountPasskeySession(ctx, session)
		return err
	})
	require.Error(t, err, "a session ID is used once")
}
