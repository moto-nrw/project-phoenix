package migrations

import (
	"context"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestVisitRevisionSessionIndexCarriesChanges pins 1.15.718 (#5273): the
// session index carries the changes of every row, so the room revision reads
// the revision of each session it held from the index alone. The migration
// runs twice, rolls back to the key-only index and applies again.
func TestVisitRevisionSessionIndexCarriesChanges(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := context.Background()
	sessionIndex := func() string {
		var definition string
		require.NoError(t, db.NewRaw(`SELECT indexdef FROM pg_indexes
			WHERE schemaname = 'active' AND indexname = 'idx_visit_revision_events_session'`).Scan(ctx, &definition))
		return definition
	}

	assert.Contains(t, sessionIndex(), "(tenant_id, active_group_id) INCLUDE (changes)")
	require.NoError(t, visitRevisionSessionIndexChangesUp(ctx, db), "the migration is idempotent")
	assert.Contains(t, sessionIndex(), "(tenant_id, active_group_id) INCLUDE (changes)")

	require.NoError(t, visitRevisionSessionIndexChangesDown(ctx, db))
	assert.Contains(t, sessionIndex(), "(tenant_id, active_group_id)")
	assert.NotContains(t, sessionIndex(), "INCLUDE")

	require.NoError(t, visitRevisionSessionIndexChangesUp(ctx, db))
	assert.Contains(t, sessionIndex(), "(tenant_id, active_group_id) INCLUDE (changes)")
}
