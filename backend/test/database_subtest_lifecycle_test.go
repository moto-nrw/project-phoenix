package test

import (
	"sync"
	"testing"

	"github.com/moto-ogs/project-phoenix/internal/testdb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Parallel sibling subtests without OwnTenant share their top-level test's
// clone. The clone must outlive the sibling that happened to open it (#6451):
// the first sibling ends while the second still queries the database.
func TestIsolatedCloneOutlivesFirstParallelSibling(t *testing.T) {
	t.Parallel()
	if parallelism() < 2 {
		// The siblings must overlap. In serial diagnostic mode, run only this
		// test in a child whose default -test.parallel derives from GOMAXPROCS=2.
		// With one root test, the child's package leftover gate also covers it.
		output, exitCode, err := testdb.RunTestProcess(t.Context(), t.Name(),
			"GOMAXPROCS=2", leftoverModeEnv+"=1",
			testdb.RunIDEnv+"="+testdb.SanitizeRunID(""))
		require.NoError(t, err, "%s", output)
		require.Zero(t, exitCode, "%s", output)
		return
	}
	opened, joined, firstCleanedUp := make(chan struct{}), make(chan struct{}), make(chan struct{})
	// A failing sibling still releases the other instead of hanging it.
	closeOpened, closeJoined := sync.OnceFunc(func() { close(opened) }), sync.OnceFunc(func() { close(joined) })
	var clone string
	t.Run("siblings", func(t *testing.T) {
		t.Run("first", func(t *testing.T) {
			t.Parallel()
			t.Cleanup(func() { close(firstCleanedUp) }) // registered first, so it runs last
			defer closeOpened()
			db := SetupIsolatedTestDB(t)
			require.NoError(t, db.NewRaw(`SELECT current_database()`).Scan(t.Context(), &clone))
			closeOpened()
			<-joined
		})
		t.Run("second", func(t *testing.T) {
			t.Parallel()
			defer closeJoined()
			<-opened
			db := SetupIsolatedTestDB(t)
			closeJoined()
			<-firstCleanedUp
			var name string
			require.NoError(t, db.NewRaw(`SELECT current_database()`).Scan(t.Context(), &name),
				"the first sibling's cleanup must not close a clone its sibling still uses")
			assert.Equal(t, clone, name, "siblings of one scope share one clone")
		})
	})
	require.False(t, hasIsolatedTestDB(t), "the last user releases the scope")
	var exists bool
	require.NoError(t, SetupTestDB(t).NewRaw(`SELECT EXISTS (SELECT FROM pg_database WHERE datname = ?)`, clone).Scan(t.Context(), &exists))
	assert.False(t, exists, "the clone is dropped once its last user finished")
}
