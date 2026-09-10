package integration

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	importModels "github.com/moto-nrw/project-phoenix/models/import"
	"github.com/moto-nrw/project-phoenix/services"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// A row that carries only the Begleitung note keeps the stored mode sets: a
// day that allows several ways home does not collapse to one, while a row
// that names Gehweise days re-derives the whole plan from those cells.
func TestDataImportCutover_DepartureModesSurviveNoteOnlyUpdate(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	tenantID := testpkg.OwnTenant(t)
	module, err := services.NewImportTestModule(db, testpkg.TenantRuntime(t, db))
	require.NoError(t, err)
	actor := newImporter(t, db)
	ctx := testpkg.Ctx(t)

	row := importModels.StudentImportRow{
		FirstName: "Vielfalt", LastName: "Gehweise", SchoolClass: "2B", DataRetentionDays: 30,
		DepartureDays: map[string]string{"mon": "bus", "tue": "pickup"},
	}
	result, err := runStudentImport(t, db, module, actor, importModels.ImportModeCreate, []importModels.StudentImportRow{row}, nil)
	require.NoError(t, err)
	requireNoRowErrors(t, result)

	var studentID int64
	require.NoError(t, db.NewSelect().TableExpr("users.students AS s").Column("s.id").
		Join("JOIN users.persons p ON p.id = s.person_id").Where("s.tenant_id = ? AND p.last_name = ?", tenantID, "Gehweise").Scan(ctx, &studentID))

	// A second way home on Monday, stored by another writer (the parents
	// portal records these mode sets), must survive an import that carries
	// only the companion note.
	_, err = db.NewUpdate().TableExpr("users.students").
		Set(`allowed_departure_modes = '{"mon":["bus","accompanied"],"tue":["pickup"]}'::jsonb`).
		Where("id = ?", studentID).Exec(ctx)
	require.NoError(t, err)

	noteOnly := importModels.StudentImportRow{
		FirstName: "Vielfalt", LastName: "Gehweise", SchoolClass: "2B", DataRetentionDays: 30,
		DepartureCompanionNote: "mit Mia",
	}
	result, err = runStudentImport(t, db, module, actor, importModels.ImportModeUpdate, []importModels.StudentImportRow{noteOnly}, nil)
	require.NoError(t, err)
	requireNoRowErrors(t, result)

	type storedPlan struct {
		Allowed string  `bun:"allowed"`
		Note    *string `bun:"departure_companion_note"`
	}
	readPlan := func() storedPlan {
		t.Helper()
		var plan storedPlan
		require.NoError(t, db.NewSelect().TableExpr("users.students").
			ColumnExpr("allowed_departure_modes::text AS allowed, departure_companion_note").Where("id = ?", studentID).Scan(ctx, &plan))
		return plan
	}
	stored := readPlan()
	assert.Contains(t, stored.Allowed, "accompanied", "the second mode of the day survives a note-only import")
	assert.Contains(t, stored.Allowed, "bus")
	require.NotNil(t, stored.Note)
	assert.Equal(t, "mit Mia", *stored.Note)

	// A row that names Gehweise days re-derives the plan and drops the note
	// together with the accompanied mode it belonged to.
	replan := importModels.StudentImportRow{
		FirstName: "Vielfalt", LastName: "Gehweise", SchoolClass: "2B", DataRetentionDays: 30,
		DepartureDays: map[string]string{"mon": "alone"},
	}
	result, err = runStudentImport(t, db, module, actor, importModels.ImportModeUpdate, []importModels.StudentImportRow{replan}, nil)
	require.NoError(t, err)
	requireNoRowErrors(t, result)
	stored = readPlan()
	assert.NotContains(t, stored.Allowed, "accompanied")
	// "alone" is the absence of a special mode, so Monday drops out of the
	// stored set entirely.
	assert.NotContains(t, stored.Allowed, "mon")
	assert.Contains(t, stored.Allowed, "pickup", "a day the row does not name keeps its stored mode")
	assert.Nil(t, stored.Note, "the note never outlives the accompanied mode")
}
