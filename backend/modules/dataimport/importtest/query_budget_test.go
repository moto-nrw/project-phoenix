package integration

import (
	"fmt"
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
	// before reading the limit, also without a Kinderkontingent. The count
	// scales with the rows, so the ceiling holds for this ten-row file only.
	testpkg.AssertQueryBudget(t, "modules.dataimport.student_batch.import", counter.Queries())
	// BUN labels RawQuery as SELECT, including the three owner INSERTs of each
	// guardian link; this is the driver-reported DML counter, not physical
	// table writes.
	require.EqualValues(t, 51, counter.WriteRows())
	smallImport := counter.Total()

	// A second point, eleven times the file: one more statement per row in
	// either path costs 110 statements here instead of 10 above.
	large := studentBatch(3, 11*studentBatchRows)
	run(large, true)
	testpkg.AssertQueryBudget(t, "modules.dataimport.student_batch.preview_110", counter.Queries())
	run(large, false)
	testpkg.AssertQueryBudget(t, "modules.dataimport.student_batch.import_110", counter.Queries())
	t.Logf("import statements: %d rows -> %d, %d rows -> %d", studentBatchRows, smallImport, len(large), counter.Total())
}

// studentBatchRows is the size of the budgeted and measured import file.
const studentBatchRows = 10

// tenStudentBatch builds a valid ten-row student file with one guardian,
// phone and pickup schedule per row; iteration keeps names and phones unique.
func tenStudentBatch(iteration int) []importModels.StudentImportRow {
	return studentBatch(iteration, studentBatchRows)
}

// studentBatch builds a valid n-row student file like tenStudentBatch; n
// stays below 1000 so the phone numbers of two iterations do not collide.
func studentBatch(iteration, n int) []importModels.StudentImportRow {
	rows := make([]importModels.StudentImportRow, 0, n)
	for i := range n {
		name := fmt.Sprintf("Runtime%d-%d", iteration, i)
		rows = append(rows, importModels.StudentImportRow{
			FirstName: name, LastName: "Evidence", SchoolClass: "4A", Birthday: "2017-04-05", DataRetentionDays: 30,
			AddressStreet: "Messweg 1", AddressPostalCode: "50667", AddressCity: "Köln",
			DepartureDays:   map[string]string{"mon": "bus", "tue": "pickup"},
			PrivacyAccepted: true,
			Guardians: []importModels.GuardianImportData{{
				FirstName: "Eltern", LastName: name, Email: fmt.Sprintf("%s.runtime@example.test", name), RelationshipType: "Mutter",
				PhoneNumbers: []importModels.PhoneImportData{{PhoneNumber: fmt.Sprintf("0151 %06d", iteration*1000+i), PhoneType: "mobile"}},
			}},
			PickupSchedules: []importModels.PickupScheduleImportData{{Weekday: 2, PickupTime: "15:30"}},
		})
	}
	return rows
}
