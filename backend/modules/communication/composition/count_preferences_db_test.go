package compose

import (
	"fmt"
	"testing"
	"time"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCountPreferenceStore pins the store behind the personal count scope
// (#3673): no row reads as "", a second choice overwrites the first, each
// school keeps its own choice, and the table accepts only the three scopes.
func TestCountPreferenceStore(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	store := newCountPreferences(db)
	ctx := testpkg.Ctx(t)

	suffix := time.Now().UnixNano()
	account := testpkg.CreateTestAccount(t, db, fmt.Sprintf("count-scope-%d@example.com", suffix))
	other := testpkg.CreateTestAccount(t, db, fmt.Sprintf("count-scope-other-%d@example.com", suffix))

	scope, err := store.CountScope(ctx, account.ID)
	require.NoError(t, err)
	assert.Empty(t, scope, "an account that never chose has no row")

	require.NoError(t, store.SetCountScope(ctx, account.ID, "none"))
	require.NoError(t, store.SetCountScope(ctx, account.ID, "own_groups"))
	scope, err = store.CountScope(ctx, account.ID)
	require.NoError(t, err)
	assert.Equal(t, "own_groups", scope, "the second choice replaces the first")

	scope, err = store.CountScope(ctx, other.ID)
	require.NoError(t, err)
	assert.Empty(t, scope, "one account's choice is not another's")

	require.Error(t, store.SetCountScope(ctx, account.ID, "everything"), "the table rejects an unknown scope")

	t.Run("another school keeps its own choice", func(t *testing.T) {
		otherCtx := testpkg.OwnCtx(t)
		scope, err := store.CountScope(otherCtx, account.ID)
		require.NoError(t, err)
		assert.Empty(t, scope)

		require.NoError(t, store.SetCountScope(otherCtx, account.ID, "none"))
		scope, err = store.CountScope(ctx, account.ID)
		require.NoError(t, err)
		assert.Equal(t, "own_groups", scope, "the first school is untouched")
	})
}
