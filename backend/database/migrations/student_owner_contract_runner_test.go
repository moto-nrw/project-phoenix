package migrations

import (
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

func TestStudentContractRunContextTreatsInitializedDatabaseAsNoFreshReplay(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx, err := studentContractRunContext(t.Context(), db)
	require.NoError(t, err)
	require.Equal(t, false, ctx.Value(freshStudentStorageKey{}), "an already initialized empty database is not a fresh replay")
}
