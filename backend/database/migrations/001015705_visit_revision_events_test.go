package migrations

import (
	"context"
	"testing"
	"time"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func countRevisionEvents(t *testing.T, db *bun.DB, schoolID int64) int {
	t.Helper()
	var rows int
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM active.visit_revision_events WHERE tenant_id = ?`, schoolID).Scan(t.Context(), &rows))
	return rows
}

// TestSchoolErasureRemovesVisitRevisionEvents pins the erasure path of
// 1.15.705 (#5128): deleting the school's visits records events of its own,
// and the plan deletes the event table after active.visits, so the residual
// check finds nothing and the other school keeps its events.
func TestSchoolErasureRemovesVisitRevisionEvents(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := context.Background()
	open := func(schoolID int64) {
		student := testpkg.CreateTestStudentForTenant(t, db, schoolID, "Revision", "Erasure", "2b")
		group := testpkg.CreateTestActiveGroupForTenant(t, db, schoolID)
		_, err := db.ExecContext(ctx, `INSERT INTO active.visits (tenant_id, student_id, active_group_id, entry_time) VALUES (?, ?, ?, ?)`,
			schoolID, student.ID, group.ID, time.Now().Add(-time.Hour))
		require.NoError(t, err)
	}
	school := testpkg.UniqueTestTenantID(t)
	other := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, school)
	testpkg.EnsureTestTenant(t, db, other)
	open(school)
	open(other)
	require.Equal(t, 1, countRevisionEvents(t, db, school))

	result := eraseSchoolAsCLI(t, db, school, beginErasure(t, db, school))

	require.Equal(t, "db_done", result.Status, "preflight, deletion and residual checks pass: %v", result.ErrorCode)
	assert.Zero(t, countRevisionEvents(t, db, school))
	assert.Equal(t, 1, countRevisionEvents(t, db, other))
}

// TestVisitRevisionEventsMigrationRoundTrip pins that 1.15.705 runs twice,
// rolls back to the 1.15.681 erasure without the table, and applies again.
func TestVisitRevisionEventsMigrationRoundTrip(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := context.Background()
	exists := func() bool {
		var found bool
		require.NoError(t, db.NewRaw(`SELECT to_regclass('active.visit_revision_events') IS NOT NULL`).Scan(ctx, &found))
		return found
	}
	require.NoError(t, visitRevisionEventsUp(ctx, db), "the migration is idempotent")
	require.NoError(t, visitRevisionEventsDown(ctx, db))
	assert.False(t, exists())
	for _, entry := range readErasurePlan(t, db) {
		assert.NotEqual(t, "active.visit_revision_events", entry.Name)
	}
	require.NoError(t, visitRevisionEventsUp(ctx, db))
	assert.True(t, exists())
}
