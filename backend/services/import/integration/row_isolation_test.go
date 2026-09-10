package integration

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	importModels "github.com/moto-nrw/project-phoenix/models/import"
	"github.com/moto-nrw/project-phoenix/services"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// Two rows of one file claiming the same free card are told which card and
// which row collides, instead of the second row failing at write time with a
// generic error: the card is still unassigned while the batch is validated,
// so only the batch-wide check can see the clash (#2708).
func TestDataImportCutover_DuplicateTagInFileIsNamed(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	tenantID := testpkg.OwnTenant(t)
	module, err := services.NewImportTestModule(db, testpkg.TenantRuntime(t, db))
	require.NoError(t, err)
	actor := newImporter(t, db)
	card := testpkg.CreateTestRFIDCard(t, db, "DUP")

	row := func(name string) importModels.StudentImportRow {
		return importModels.StudentImportRow{FirstName: name, LastName: "Karte", SchoolClass: "3C", DataRetentionDays: 30, TagID: card.ID}
	}
	before := countImportRows(t, db, tenantID)
	result, err := runStudentImport(t, db, module, actor, importModels.ImportModeCreate, []importModels.StudentImportRow{row("Erste"), row("Zweite")}, nil)
	require.NoError(t, err)
	assert.Equal(t, 1, result.CreatedCount)
	require.Equal(t, 1, result.ErrorCount)
	require.Len(t, result.Errors, 1)
	assert.Equal(t, 3, result.Errors[0].RowNumber)
	require.Len(t, result.Errors[0].Errors, 1)
	assert.Equal(t, "duplicate_in_file", result.Errors[0].Errors[0].Code)
	assert.Contains(t, result.Errors[0].Errors[0].Message, card.ID)
	assert.Contains(t, result.Errors[0].Errors[0].Message, "Zeile 2")
	after := countImportRows(t, db, tenantID)
	assert.Equal(t, before.students+1, after.students, "only the first claim is written")
}

// A class-list entry and its audit row share a savepoint: a refused trail
// leaves no unaudited entry behind while the rest of the batch commits.
func TestDataImportCutover_ClassListEntryAndAuditShareASavepoint(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	tenantID := testpkg.Tenant(t)
	module, err := services.NewImportTestModule(db, testpkg.TenantRuntime(t, db))
	require.NoError(t, err)
	actor := newImporter(t, db)
	ctx := testpkg.Ctx(t)

	entryCount := func() int {
		t.Helper()
		var n int
		require.NoError(t, db.NewSelect().TableExpr("users.class_list_entries").ColumnExpr("count(*)").Where("tenant_id = ?", tenantID).Scan(ctx, &n))
		return n
	}

	// Make the audit trail refuse exactly the second row. The clone is this
	// test's own database, so the temporary constraint affects nothing else.
	_, err = db.NewRaw(`ALTER TABLE audit.class_list_entry_changes ADD CONSTRAINT tmp_refuse_second CHECK (new_value NOT LIKE 'Refused%')`).Exec(context.Background())
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = db.NewRaw(`ALTER TABLE audit.class_list_entry_changes DROP CONSTRAINT IF EXISTS tmp_refuse_second`).Exec(context.Background())
	})

	var result *importModels.ImportResult[importModels.ClassListEntryImportRow]
	require.NoError(t, testpkg.WithTenantTx(t, testpkg.Ctx(t), db, tenantID, func(txCtx context.Context, _ bun.Tx) error {
		var txErr error
		result, txErr = module.ClassListImport.Import(txCtx, importModels.ImportRequest[importModels.ClassListEntryImportRow]{
			Rows: []importModels.ClassListEntryImportRow{
				{FirstName: "Gute", LastName: "Zeile", SchoolClass: "1a"},
				{FirstName: "Refused", LastName: "Zeile", SchoolClass: "1a"},
			},
			Mode: importModels.ImportModeCreate, UserID: actor.staffID, SkipInvalidRows: true,
		})
		return txErr
	}))
	assert.Equal(t, 1, result.CreatedCount)
	assert.Equal(t, 1, result.ErrorCount)
	assert.Equal(t, 1, entryCount(), "the entry whose audit row was refused rolled back with it")
	var stored string
	require.NoError(t, db.NewSelect().TableExpr("users.class_list_entries").Column("first_name").Where("tenant_id = ?", tenantID).Scan(ctx, &stored))
	assert.Equal(t, "Gute", stored)
}
