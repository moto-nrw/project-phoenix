package postgres

import (
	"context"
	"fmt"
	"testing"
	"time"

	testpkg "github.com/moto-ogs/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-ogs/project-phoenix/modules/identityaccess/internal/domain"
)

// The first credential of an account lands only on a row without a password
// (#6289): an initial write must never replace an existing password, which
// would skip the step of the credential generation.
func TestInitialCredentialWriteLeavesAnExistingPasswordAlone(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := context.Background()
	store := lockingStore(db, 0)
	account := testpkg.CreateTestAccountWithPassword(t, db, fmt.Sprintf("initial-credential-%d@test.local", time.Now().UnixNano()), testpkg.NewTestPassword())
	var before string
	require.NoError(t, db.NewRaw(`SELECT password_hash FROM auth.accounts WHERE id = ?`, account.ID).Scan(ctx, &before))

	written, _, err := store.WriteAccountCredential(ctx, account.ID, domain.CredentialWrite{Initial: true, PasswordHash: "replaced"})
	require.NoError(t, err)
	assert.False(t, written, "an account that has a password takes no initial one")

	var after string
	var generation int64
	require.NoError(t, db.NewRaw(`SELECT password_hash, mfa_reset_generation FROM auth.accounts WHERE id = ?`, account.ID).Scan(ctx, &after, &generation))
	assert.Equal(t, before, after)
	assert.Zero(t, generation)
}

// An account created without a password takes its first one and keeps its
// generation.
func TestInitialCredentialWriteSetsTheFirstPassword(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := context.Background()
	store := lockingStore(db, 0)
	var id int64
	require.NoError(t, db.NewRaw(`INSERT INTO auth.accounts (email, active) VALUES (?, TRUE) RETURNING id`,
		fmt.Sprintf("initial-credential-new-%d@test.local", time.Now().UnixNano())).Scan(ctx, &id))
	t.Cleanup(func() {
		_, err := db.NewRaw(`DELETE FROM auth.accounts WHERE id = ?`, id).Exec(context.Background())
		require.NoError(t, err)
	})

	written, _, err := store.WriteAccountCredential(ctx, id, domain.CredentialWrite{Initial: true, PasswordHash: "first"})
	require.NoError(t, err)
	require.True(t, written)
	var hash string
	var generation int64
	require.NoError(t, db.NewRaw(`SELECT password_hash, mfa_reset_generation FROM auth.accounts WHERE id = ?`, id).Scan(ctx, &hash, &generation))
	assert.Equal(t, "first", hash)
	assert.Zero(t, generation)
}
