package migrations

import (
	"strings"
	"testing"

	testpkg "github.com/moto-ogs/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The five school erasure functions of 1.15.954 are those of 1.15.942 with
// the note notices added and nothing else changed: taking the added lines out
// again (and the fingerprint back) yields the base function byte for byte.
func TestStudentNoteTeamNoticeErasureExtendsTheErasureOf942(t *testing.T) {
	t.Parallel()

	functions, err := studentNoteTeamNoticeErasureSQL()
	require.NoError(t, err)
	bases := []string{
		holidayClosureErasurePlanSQL, holidayClosureErasurePreflightSQL,
		holidayClosureErasureResidualSQL, holidayClosureErasureDeleteSQL,
		holidayClosureErasurePreviewSQL,
	}
	require.Len(t, functions, len(bases))
	for i, function := range functions {
		assert.Contains(t, function, studentNoteTeamNoticesTable)
		kept := make([]string, 0)
		for line := range strings.SplitSeq(function, "\n") {
			if strings.Contains(line, studentNoteTeamNoticesTable) {
				continue
			}
			kept = append(kept, line)
		}
		restored := strings.Join(kept, "\n")
		restored = strings.Replace(restored, studentNoteTeamNoticesFingerprint, holidayClosureErasureFingerprint, 1)
		// The residual and preflight checks join their lines with UNION ALL;
		// the added check brought one more.
		restored = strings.Replace(restored, "    UNION ALL\n    UNION ALL\n", "    UNION ALL\n", 1)
		assert.Equal(t, bases[i], restored, "erasure function %d changes only by the note notices", i)
	}
}

// The notices are deleted before anything they cascade from, so the count the
// erasure reports is the number of rows it deleted, not zero after a cascade.
func TestStudentNoteTeamNoticeErasureDeletesNoticesFirst(t *testing.T) {
	t.Parallel()

	functions, err := studentNoteTeamNoticeErasureSQL()
	require.NoError(t, err)
	deletion := functions[3]
	notices := strings.Index(deletion, "DELETE FROM "+studentNoteTeamNoticesTable)
	require.GreaterOrEqual(t, notices, 0)
	for _, parent := range []string{"DELETE FROM users.student_notes", "DELETE FROM auth.account_tenants"} {
		index := strings.Index(deletion, parent)
		require.GreaterOrEqual(t, index, 0, parent)
		assert.Less(t, notices, index, parent)
	}
}

func TestStudentNoteTeamNoticeErasureRefusesAChangedBase(t *testing.T) {
	t.Parallel()

	_, err := applyErasureEdits("no anchor here", erasureEdit{anchor: "users.student_notes"})
	require.Error(t, err)
}

// The migration rolls back to the erasure of 1.15.942 without the table and
// applies again.
func TestStudentNoteTeamNoticesMigrationRoundTrip(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := t.Context()
	exists := func() bool {
		var found bool
		require.NoError(t, db.NewRaw(`SELECT to_regclass('users.student_note_team_notices') IS NOT NULL`).Scan(ctx, &found))
		return found
	}
	preflight := func() string {
		var definition string
		require.NoError(t, db.NewRaw(`SELECT pg_get_functiondef('platform.school_erasure_preflight(bigint,bigint)'::regprocedure)`).Scan(ctx, &definition))
		return definition
	}
	require.True(t, exists())
	assert.Contains(t, preflight(), studentNoteTeamNoticesFingerprint)

	require.NoError(t, studentNoteTeamNoticesDown(ctx, db))
	assert.False(t, exists())
	assert.Contains(t, preflight(), holidayClosureErasureFingerprint)
	assert.NotContains(t, preflight(), studentNoteTeamNoticesTable)

	require.NoError(t, studentNoteTeamNoticesUp(ctx, db))
	assert.True(t, exists())
	assert.Contains(t, preflight(), studentNoteTeamNoticesFingerprint)
}
