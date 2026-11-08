package integration

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/api"
	importModels "github.com/moto-nrw/project-phoenix/modules/dataimport"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	querybudgets "github.com/moto-nrw/project-phoenix/test/query-budgets"
)

// The scenarios below are the N10/N110 measuring points of the ADR 0067
// addendum of 2026-11-08 (#5552, Messplan N10/N110) for every import config
// and mode beside the student create in query_budget_test.go. Each runs two
// warmups like TestStudentImportBatchQueryBudget, then a ten-row file and an
// eleven times larger file in two write transactions; the difference between
// both points is the growth for 100 rows. Student and staff accept create,
// update and upsert, each also as a preview; class list and opening balances
// are create-only over HTTP.
//
// Upsert mix: rows with an even index exist and change fixed cells, rows with
// an odd index are new. Every file has an even row count, so it is half new
// and half existing, and each 100-row chunk of N110 is too.

// outcome is the expected split of a file into created and updated rows.
type outcome struct{ created, updated int }

func allCreated(n int) outcome { return outcome{created: n} }
func allUpdated(n int) outcome { return outcome{updated: n} }
func upsertMix(n int) outcome  { return outcome{created: n / 2, updated: n / 2} }

// importBudgetRun drives one config's ImportBatches with a query counter and
// fails on any row error, so a budget never measures a rejected row.
type importBudgetRun[T any] struct {
	importer importModels.RowImporter[T]
	ctx      context.Context
	counter  *testpkg.QueryCounter
	userID   int64
	audit    importModels.BatchAudit
}

// run imports rows, requires the expected created and updated counters and
// returns the statements of exactly this call.
func (r importBudgetRun[T]) run(t *testing.T, rows []T, mode importModels.ImportMode, dryRun bool, want outcome) querybudgets.Measurement {
	t.Helper()
	r.counter.Reset()
	result, err := r.importer.ImportBatches(r.ctx, importModels.ImportRequest[T]{
		Rows: rows, Mode: mode, DryRun: dryRun, UserID: r.userID, SkipInvalidRows: true,
	}, r.audit)
	require.NoError(t, err)
	require.Zero(t, result.ErrorCount, "%+v", result.Errors)
	require.Equal(t, want, outcome{created: result.CreatedCount, updated: result.UpdatedCount}, "mode %s, dry run %t", mode, dryRun)
	t.Logf("%d rows, mode %s, dry run %t: %d statements", len(rows), mode, dryRun, r.counter.Total())
	return r.counter.Measurement(r.counter.Queries())
}

// previewAndCreate are the measurements of one file's preview and create.
type previewAndCreate struct{ preview, create querybudgets.Measurement }

// updateAndUpsert are the measurements of one file's update and upsert, each
// with its preview.
type updateAndUpsert struct{ previewUpdate, update, previewUpsert, upsert querybudgets.Measurement }

// countByFirstName counts rows of a raw count query restricted to persons
// whose first name starts with prefix, outside the counted context.
func countByFirstName(t *testing.T, db *bun.DB, query, prefix string, args ...any) int {
	t.Helper()
	var n int
	require.NoError(t, db.NewRaw(query, append([]any{testpkg.Tenant(t), prefix + "%"}, args...)...).Scan(testpkg.Ctx(t), &n))
	return n
}

const (
	studentsInCity = `SELECT count(*) FROM users.student_profiles s JOIN users.persons p ON p.id = s.person_id
		WHERE s.tenant_id = ? AND p.first_name LIKE ? AND s.address_city = ?`
	studentPickupsAt = `SELECT count(*) FROM schedule.student_pickup_schedules ps JOIN users.student_profiles s ON s.id = ps.student_id
		JOIN users.persons p ON p.id = s.person_id WHERE ps.tenant_id = ? AND p.first_name LIKE ? AND ps.pickup_time = ?::time`
	staffInCity = `SELECT count(*) FROM users.staff_master_data md JOIN users.staff_school_memberships s ON s.id = md.staff_id
		JOIN users.persons p ON p.id = s.person_id WHERE md.tenant_id = ? AND p.first_name LIKE ? AND md.address_city = ? AND md.weekly_hours = ?`
)

// TestStudentImportUpdateAndUpsertBatchQueryBudget pins update and upsert of
// a student file, each with its preview, after an unmeasured create of the
// file. The update changes fixed cells (Befund 6 of the addendum): every row
// moves the child to another city and its pickup to a later time, so the
// profile patch and the care write both run. The upsert follows the mix above:
// existing children move once more, new ones come from another iteration.
func TestStudentImportUpdateAndUpsertBatchQueryBudget(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module, err := api.NewImportTestModule(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db))
	require.NoError(t, err)
	actor := newImporter(t, db)
	counter := testpkg.CaptureQueriesForContext(t, db)
	runner := importBudgetRun[importModels.StudentImportRow]{
		importer: module.Import, ctx: counter.Context(testpkg.Ctx(t)), counter: counter, userID: actor.staffID,
		audit: importModels.BatchAudit{EntityType: "student", Filename: "budget.csv", AccountID: actor.accountID},
	}
	measure := func(iteration, n int) updateAndUpsert {
		t.Helper()
		file := studentBatch(iteration, n)
		runner.run(t, file, importModels.ImportModeCreate, false, allCreated(n))
		changed := make([]importModels.StudentImportRow, n)
		mixed := make([]importModels.StudentImportRow, n)
		fresh := studentBatch(iteration+10, n)
		for i, row := range file {
			row.AddressCity = "Bonn"
			row.PickupSchedules = []importModels.PickupScheduleImportData{{Weekday: 2, PickupTime: "16:45"}}
			changed[i] = row
			row.AddressCity = "Münster"
			mixed[i] = row
			if i%2 == 1 {
				mixed[i] = fresh[i]
			}
		}
		var measured updateAndUpsert
		measured.previewUpdate = runner.run(t, changed, importModels.ImportModeUpdate, true, allUpdated(n))
		measured.update = runner.run(t, changed, importModels.ImportModeUpdate, false, allUpdated(n))
		existing := fmt.Sprintf("Runtime%d-", iteration)
		require.Equal(t, n, countByFirstName(t, db, studentsInCity, existing, "Bonn"), "every updated child moved to the new city")
		require.Equal(t, n, countByFirstName(t, db, studentPickupsAt, existing, "16:45"), "every updated pickup moved to the new time")
		measured.previewUpsert = runner.run(t, mixed, importModels.ImportModeUpsert, true, upsertMix(n))
		measured.upsert = runner.run(t, mixed, importModels.ImportModeUpsert, false, upsertMix(n))
		require.Equal(t, n/2, countByFirstName(t, db, studentsInCity, existing, "Münster"), "the existing half was updated")
		require.Equal(t, n/2, countByFirstName(t, db, studentsInCity, fmt.Sprintf("Runtime%d-", iteration+10), "Köln"), "the new half was created")
		return measured
	}
	for iteration := range 2 {
		measure(iteration, studentBatchRows)
	}
	small := measure(2, studentBatchRows)
	querybudgets.AssertQueryBudget(t, "modules.dataimport.student_batch.preview_update", small.previewUpdate)
	querybudgets.AssertQueryBudget(t, "modules.dataimport.student_batch.update", small.update)
	querybudgets.AssertQueryBudget(t, "modules.dataimport.student_batch.preview_upsert", small.previewUpsert)
	querybudgets.AssertQueryBudget(t, "modules.dataimport.student_batch.upsert", small.upsert)
	large := measure(3, 11*studentBatchRows)
	querybudgets.AssertQueryBudget(t, "modules.dataimport.student_batch.preview_update_110", large.previewUpdate)
	querybudgets.AssertQueryBudget(t, "modules.dataimport.student_batch.update_110", large.update)
	querybudgets.AssertQueryBudget(t, "modules.dataimport.student_batch.preview_upsert_110", large.previewUpsert)
	querybudgets.AssertQueryBudget(t, "modules.dataimport.student_batch.upsert_110", large.upsert)
}

// TestStaffImportBatchQueryBudget pins create, update and upsert of a staff
// file, each with its preview. Rows carry role, personnel number, weekly hours
// and one qualification and no e-mail, so no invitation is sent. The update
// matches each row by its personnel number and changes the city and the
// weekly hours; the upsert follows the mix above, and in upsert mode every row
// also looks up its existing record during validation
// (skipStaffRoleGrantCheck).
func TestStaffImportBatchQueryBudget(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module, err := api.NewImportTestModule(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db))
	require.NoError(t, err)
	actor := newImporter(t, db)
	role := testpkg.CreateTestRoleForTenant(t, db, "Betreuungskraft", testpkg.Tenant(t))
	counter := testpkg.CaptureQueriesForContext(t, db)
	ctx := tenant.WithUnitOfWork(testpkg.Ctx(t), testpkg.TenantRuntime(t, db))
	ctx = importModels.ContextWithImporterPermissions(counter.Context(ctx), []string{"admin:*"})
	runner := importBudgetRun[importModels.StaffImportRow]{
		importer: module.StaffImport, ctx: ctx, counter: counter, userID: actor.accountID,
		audit: importModels.BatchAudit{EntityType: "staff", Filename: "budget.csv", AccountID: actor.accountID},
	}
	staffRow := func(iteration, i int) importModels.StaffImportRow {
		return importModels.StaffImportRow{
			FirstName: fmt.Sprintf("Budget%d-%d", iteration, i), LastName: "Staff", RoleName: role.Name,
			PersonnelNumber: fmt.Sprintf("P-%d-%d", iteration, i), WeeklyHours: "19,5", Qualifications: "Erste Hilfe",
		}
	}
	measure := func(iteration, n int) (previewAndCreate, updateAndUpsert) {
		t.Helper()
		file := make([]importModels.StaffImportRow, 0, n)
		changed := make([]importModels.StaffImportRow, 0, n)
		mixed := make([]importModels.StaffImportRow, 0, n)
		for i := range n {
			row := staffRow(iteration, i)
			file = append(file, row)
			row.AddressCity, row.WeeklyHours = "Bonn", "30"
			changed = append(changed, row)
			row.AddressCity = "Münster"
			if i%2 == 1 {
				row = staffRow(iteration+10, i)
			}
			mixed = append(mixed, row)
		}
		created := previewAndCreate{
			preview: runner.run(t, file, importModels.ImportModeCreate, true, allCreated(n)),
			create:  runner.run(t, file, importModels.ImportModeCreate, false, allCreated(n)),
		}
		var measured updateAndUpsert
		measured.previewUpdate = runner.run(t, changed, importModels.ImportModeUpdate, true, allUpdated(n))
		measured.update = runner.run(t, changed, importModels.ImportModeUpdate, false, allUpdated(n))
		existing := fmt.Sprintf("Budget%d-", iteration)
		require.Equal(t, n, countByFirstName(t, db, staffInCity, existing, "Bonn", 30), "every update reached Workforce")
		measured.previewUpsert = runner.run(t, mixed, importModels.ImportModeUpsert, true, upsertMix(n))
		measured.upsert = runner.run(t, mixed, importModels.ImportModeUpsert, false, upsertMix(n))
		require.Equal(t, n/2, countByFirstName(t, db, staffInCity, existing, "Münster", 30), "the existing half was updated")
		require.Equal(t, n/2, countByFirstName(t, db, `SELECT count(*) FROM users.staff_school_memberships s
			JOIN users.persons p ON p.id = s.person_id WHERE s.tenant_id = ? AND p.first_name LIKE ?`, fmt.Sprintf("Budget%d-", iteration+10)), "the new half was created")
		return created, measured
	}
	for iteration := range 2 {
		measure(iteration, 10)
	}
	small, smallModes := measure(2, 10)
	querybudgets.AssertQueryBudget(t, "modules.dataimport.staff_batch.preview", small.preview)
	querybudgets.AssertQueryBudget(t, "modules.dataimport.staff_batch.import", small.create)
	querybudgets.AssertQueryBudget(t, "modules.dataimport.staff_batch.preview_update", smallModes.previewUpdate)
	querybudgets.AssertQueryBudget(t, "modules.dataimport.staff_batch.update", smallModes.update)
	querybudgets.AssertQueryBudget(t, "modules.dataimport.staff_batch.preview_upsert", smallModes.previewUpsert)
	querybudgets.AssertQueryBudget(t, "modules.dataimport.staff_batch.upsert", smallModes.upsert)
	large, largeModes := measure(3, 110)
	querybudgets.AssertQueryBudget(t, "modules.dataimport.staff_batch.preview_110", large.preview)
	querybudgets.AssertQueryBudget(t, "modules.dataimport.staff_batch.import_110", large.create)
	querybudgets.AssertQueryBudget(t, "modules.dataimport.staff_batch.preview_update_110", largeModes.previewUpdate)
	querybudgets.AssertQueryBudget(t, "modules.dataimport.staff_batch.update_110", largeModes.update)
	querybudgets.AssertQueryBudget(t, "modules.dataimport.staff_batch.preview_upsert_110", largeModes.previewUpsert)
	querybudgets.AssertQueryBudget(t, "modules.dataimport.staff_batch.upsert_110", largeModes.upsert)
}

// TestClassListImportBatchQueryBudget pins preview and create of a class list
// with name and class. The class-list import is create-only.
func TestClassListImportBatchQueryBudget(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module, err := api.NewImportTestModule(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db))
	require.NoError(t, err)
	actor := newImporter(t, db)
	counter := testpkg.CaptureQueriesForContext(t, db)
	runner := importBudgetRun[importModels.ClassListEntryImportRow]{
		importer: module.ClassListImport, ctx: counter.Context(testpkg.Ctx(t)), counter: counter, userID: actor.staffID,
		audit: importModels.BatchAudit{EntityType: "class_list_entries", Filename: "budget.csv", AccountID: actor.accountID},
	}
	measure := func(iteration, n int) previewAndCreate {
		t.Helper()
		file := make([]importModels.ClassListEntryImportRow, 0, n)
		for i := range n {
			file = append(file, importModels.ClassListEntryImportRow{FirstName: fmt.Sprintf("Budget%d-%d", iteration, i), LastName: "Liste", SchoolClass: "1a"})
		}
		return previewAndCreate{
			preview: runner.run(t, file, importModels.ImportModeCreate, true, allCreated(n)),
			create:  runner.run(t, file, importModels.ImportModeCreate, false, allCreated(n)),
		}
	}
	for iteration := range 2 {
		measure(iteration, 10)
	}
	small := measure(2, 10)
	querybudgets.AssertQueryBudget(t, "modules.dataimport.class_list_batch.preview", small.preview)
	querybudgets.AssertQueryBudget(t, "modules.dataimport.class_list_batch.import", small.create)
	large := measure(3, 110)
	querybudgets.AssertQueryBudget(t, "modules.dataimport.class_list_batch.preview_110", large.preview)
	querybudgets.AssertQueryBudget(t, "modules.dataimport.class_list_batch.import_110", large.create)
}

// TestOpeningBalanceImportBatchQueryBudget pins preview and create of opening
// balances with hours, entitlement, carryover and remaining days for staff
// matched by name. Every file names staff without an opening of their own.
func TestOpeningBalanceImportBatchQueryBudget(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module, err := api.NewImportTestModuleWithOptions(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db), api.ImportTestOptions{
		Clock: func() time.Time { return time.Date(2026, time.September, 12, 12, 0, 0, 0, time.UTC) },
	})
	require.NoError(t, err)
	actor := newImporter(t, db)
	counter := testpkg.CaptureQueriesForContext(t, db)
	runner := importBudgetRun[importModels.OpeningBalanceImportRow]{
		importer: module.OpeningBalanceImport(testpkg.Date(2026, time.September, 1), "Übernahme", actor.staffID),
		ctx:      counter.Context(tenant.WithUnitOfWork(testpkg.Ctx(t), testpkg.TenantRuntime(t, db))), counter: counter, userID: actor.accountID,
		audit: importModels.BatchAudit{EntityType: "opening_balance", Filename: "budget.csv", AccountID: actor.accountID, Options: "2026-09-01"},
	}
	measure := func(iteration, n int) previewAndCreate {
		t.Helper()
		file := make([]importModels.OpeningBalanceImportRow, 0, n)
		for i := range n {
			first := fmt.Sprintf("Budget%d-%d", iteration, i)
			testpkg.CreateTestStaff(t, db, first, "Salden")
			file = append(file, importModels.OpeningBalanceImportRow{
				FirstName: first, LastName: "Salden", HoursBalance: "-3,25", VacationEntitled: "30", VacationCarryover: "4", VacationRemaining: "12,5",
			})
		}
		return previewAndCreate{
			preview: runner.run(t, file, importModels.ImportModeCreate, true, allCreated(n)),
			create:  runner.run(t, file, importModels.ImportModeCreate, false, allCreated(n)),
		}
	}
	for iteration := range 2 {
		measure(iteration, 10)
	}
	small := measure(2, 10)
	querybudgets.AssertQueryBudget(t, "modules.dataimport.opening_balance_batch.preview", small.preview)
	querybudgets.AssertQueryBudget(t, "modules.dataimport.opening_balance_batch.import", small.create)
	large := measure(3, 110)
	querybudgets.AssertQueryBudget(t, "modules.dataimport.opening_balance_batch.preview_110", large.preview)
	querybudgets.AssertQueryBudget(t, "modules.dataimport.opening_balance_batch.import_110", large.create)
}
