package migrations

import (
	"strings"
	"testing"

	testpkg "github.com/moto-ogs/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The five school erasure functions of 1.15.956 are those of 1.15.954 with
// the pickup change notices added and nothing else changed: taking the added
// lines out again (and the fingerprint back) yields the base byte for byte.
func TestPickupChangeTeamNoticeErasureExtendsTheErasureOf954(t *testing.T) {
	t.Parallel()

	functions, err := pickupChangeTeamNoticeErasureSQL()
	require.NoError(t, err)
	bases, err := studentNoteTeamNoticeErasureSQL()
	require.NoError(t, err)
	require.Len(t, functions, len(bases))
	for i, function := range functions {
		assert.Contains(t, function, pickupChangeTeamNoticesTable)
		kept := make([]string, 0)
		for line := range strings.SplitSeq(function, "\n") {
			if strings.Contains(line, pickupChangeTeamNoticesTable) {
				continue
			}
			kept = append(kept, line)
		}
		restored := strings.Join(kept, "\n")
		restored = strings.Replace(restored, pickupChangeTeamNoticesFingerprint, studentNoteTeamNoticesFingerprint, 1)
		// The residual and preflight checks join their lines with UNION ALL;
		// the added check brought one more.
		restored = strings.Replace(restored, "    UNION ALL\n    UNION ALL\n", "    UNION ALL\n", 1)
		assert.Equal(t, bases[i], restored, "erasure function %d changes only by the pickup change notices", i)
	}
}

// The markers are deleted before anything they cascade from, so the count the
// erasure reports is the number of rows it deleted, not zero after a cascade.
func TestPickupChangeTeamNoticeErasureDeletesNoticesFirst(t *testing.T) {
	t.Parallel()

	functions, err := pickupChangeTeamNoticeErasureSQL()
	require.NoError(t, err)
	deletion := functions[3]
	notices := strings.Index(deletion, "DELETE FROM "+pickupChangeTeamNoticesTable)
	require.GreaterOrEqual(t, notices, 0)
	for _, parent := range []string{"DELETE FROM schedule.care_schedule_change_requests", "DELETE FROM auth.account_tenants"} {
		index := strings.Index(deletion, parent)
		require.GreaterOrEqual(t, index, 0, parent)
		assert.Less(t, notices, index, parent)
	}
}

// The migration rolls back to the erasure of 1.15.954 without the table and
// applies again.
func TestPickupChangeTeamNoticesMigrationRoundTrip(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := t.Context()
	exists := func() bool {
		var found bool
		require.NoError(t, db.NewRaw(`SELECT to_regclass('schedule.pickup_change_team_notices') IS NOT NULL`).Scan(ctx, &found))
		return found
	}
	preflight := func() string {
		var definition string
		require.NoError(t, db.NewRaw(`SELECT pg_get_functiondef('platform.school_erasure_preflight(bigint,bigint)'::regprocedure)`).Scan(ctx, &definition))
		return definition
	}
	require.True(t, exists())
	assert.Contains(t, preflight(), pickupChangeTeamNoticesFingerprint)

	require.NoError(t, pickupChangeTeamNoticesDown(ctx, db))
	assert.False(t, exists())
	assert.Contains(t, preflight(), studentNoteTeamNoticesFingerprint)
	assert.NotContains(t, preflight(), pickupChangeTeamNoticesTable)

	require.NoError(t, pickupChangeTeamNoticesUp(ctx, db))
	assert.True(t, exists())
	assert.Contains(t, preflight(), pickupChangeTeamNoticesFingerprint)
}
