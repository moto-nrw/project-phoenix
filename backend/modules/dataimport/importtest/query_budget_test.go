package integration

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api"
	importModels "github.com/moto-nrw/project-phoenix/modules/dataimport"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// TestStudentImportBatchQueryBudget pins the statements and driver-reported
// write rows of one preview and one import of a ten-row student file, after
// the same two warmups the opt-in TestDataImportRuntimeEvidence runs (#4426).
func TestStudentImportBatchQueryBudget(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module, err := api.NewImportTestModule(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db))
	require.NoError(t, err)
	actor := newImporter(t, db)
	counter := testpkg.CaptureQueriesForContext(t, db)
	ctx := counter.Context(testpkg.Ctx(t))

	run := func(rows []importModels.StudentImportRow, dryRun bool) {
		t.Helper()
		counter.Reset()
		result, err := module.Import.ImportBatches(ctx, importModels.ImportRequest[importModels.StudentImportRow]{
			Rows: rows, Mode: importModels.ImportModeCreate, DryRun: dryRun, UserID: actor.staffID, SkipInvalidRows: true,
		}, importModels.BatchAudit{EntityType: "student", Filename: "budget.csv", AccountID: actor.accountID})
		require.NoError(t, err)
		require.Zero(t, result.ErrorCount)
	}
	for iteration := range 2 {
		rows := tenStudentBatch(iteration)
		run(rows, true)
		run(rows, false)
	}

	rows := tenStudentBatch(2)
	run(rows, true)
	testpkg.AssertQueryBudget(t, "modules.dataimport.student_batch.preview", counter.Queries())
	require.EqualValues(t, 1, counter.WriteRows())

	run(rows, false)
	// Each of the ten guardian links first clears childless_since for
	// guardian retention, then is one unit of work over three owners since
	// #2756: a savepoint, the People Directory relationship, the Care Plan
	// pickup permission and the Identity & Access portal access. Since #3567
	// each new child takes the class-write gate and Kinderkontingent lock
	// before reading the limit, also without a Kinderkontingent.
	testpkg.AssertQueryBudget(t, "modules.dataimport.student_batch.import", counter.Queries())
	// BUN labels RawQuery as SELECT, including the three owner INSERTs of each
	// guardian link; this is the driver-reported DML counter, not physical
	// table writes.
	require.EqualValues(t, 51, counter.WriteRows())
}
