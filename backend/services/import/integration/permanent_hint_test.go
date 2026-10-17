package integration

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api"
	importModels "github.com/moto-nrw/project-phoenix/modules/dataimport"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// The import's „Betreuernotizen" column is the one writer of a carried-over
// hint left after #4268: the cell becomes the child's permanent hint on the
// note card, an update replaces it, and an empty cell leaves it alone.
func TestDataImportCutover_BetreuernotizenBecomePermanentHint(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	tenantID := testpkg.OwnTenant(t)
	module, err := api.NewImportTestModule(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db))
	require.NoError(t, err)
	actor := newImporter(t, db)
	ctx := testpkg.Ctx(t)

	row := importModels.StudentImportRow{
		FirstName: "Hinweis", LastName: "Import", SchoolClass: "3C", DataRetentionDays: 30,
		SupervisorNotes: "  Braucht morgens etwas Zeit.  ",
	}
	result, err := runStudentImport(t, db, module, actor, importModels.ImportModeCreate, []importModels.StudentImportRow{row}, nil)
	require.NoError(t, err)
	requireNoRowErrors(t, result)

	var studentID int64
	require.NoError(t, db.NewSelect().TableExpr("users.student_profiles AS s").Column("s.id").
		Join("JOIN users.persons p ON p.id = s.person_id").Where("s.tenant_id = ? AND p.last_name = ?", tenantID, "Import").Scan(ctx, &studentID))

	type hint struct {
		Body       string `bun:"body"`
		Kind       string `bun:"kind"`
		Visibility string `bun:"visibility"`
		HasAuthor  bool   `bun:"has_author"`
	}
	liveHints := func() []hint {
		t.Helper()
		var hints []hint
		require.NoError(t, db.NewRaw(`SELECT body, kind, visibility, author_account_id IS NOT NULL AS has_author
			FROM users.student_notes
			WHERE tenant_id = ? AND student_id = ? AND origin = 'master_data' AND deleted_at IS NULL`,
			tenantID, studentID).Scan(ctx, &hints))
		return hints
	}

	hints := liveHints()
	require.Len(t, hints, 1, "the cell becomes exactly one permanent hint")
	assert.Equal(t, "Braucht morgens etwas Zeit.", hints[0].Body)
	assert.Equal(t, "permanent", hints[0].Kind)
	assert.Equal(t, "all_staff", hints[0].Visibility)
	assert.False(t, hints[0].HasAuthor, "an imported hint names no author")

	updated := row
	updated.SupervisorNotes = "Morgens bitte in Ruhe ankommen lassen."
	result, err = runStudentImport(t, db, module, actor, importModels.ImportModeUpdate, []importModels.StudentImportRow{updated}, nil)
	require.NoError(t, err)
	requireNoRowErrors(t, result)
	hints = liveHints()
	require.Len(t, hints, 1, "a re-import replaces the hint instead of adding one")
	assert.Equal(t, "Morgens bitte in Ruhe ankommen lassen.", hints[0].Body)

	empty := row
	empty.SupervisorNotes = ""
	empty.ExtraInfo = "Geschwisterkind in 1a"
	result, err = runStudentImport(t, db, module, actor, importModels.ImportModeUpdate, []importModels.StudentImportRow{empty}, nil)
	require.NoError(t, err)
	requireNoRowErrors(t, result)
	hints = liveHints()
	require.Len(t, hints, 1, "an empty cell never removes a hint")
	assert.Equal(t, "Morgens bitte in Ruhe ankommen lassen.", hints[0].Body)
}
