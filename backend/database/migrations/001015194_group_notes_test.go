package migrations

import (
	"context"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func groupNotesColumnExists(t *testing.T, db *testpkg.DB) bool {
	t.Helper()
	var exists bool
	require.NoError(t, db.NewRaw(`
		SELECT EXISTS (
			SELECT 1 FROM information_schema.columns
			WHERE table_schema = 'activities' AND table_name = 'groups'
				AND column_name = 'notes'
		)
	`).Scan(context.Background(), &exists))
	return exists
}

// TestGroupNotesMigration verifies migration 1.15.194: Up adds the
// activities.groups.notes column (Wochennotiz), Down removes it, and the
// round-trip is idempotent (#1837 follow-up).
func TestGroupNotesMigration(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := context.Background()

	// Baseline: the fully-migrated test DB has the column. Restore it on exit.
	if !groupNotesColumnExists(t, db) {
		require.NoError(t, groupNotesUp(ctx, db))
	}
	t.Cleanup(func() {
		if !groupNotesColumnExists(t, db) {
			require.NoError(t, groupNotesUp(ctx, db))
		}
	})
	require.True(t, groupNotesColumnExists(t, db), "baseline: notes column present")

	// Down removes the column.
	require.NoError(t, groupNotesDown(ctx, db))
	assert.False(t, groupNotesColumnExists(t, db), "Down should drop the notes column")

	// Up re-adds it (idempotent — safe to run twice).
	require.NoError(t, groupNotesUp(ctx, db))
	assert.True(t, groupNotesColumnExists(t, db), "Up should add the notes column")
	require.NoError(t, groupNotesUp(ctx, db), "Up must be idempotent (IF NOT EXISTS)")
}
