package integration

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

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
// both points is the growth for 100 rows.

// importBudgetRun drives one config's ImportBatches with a query counter and
// fails on any row error, so a budget never measures a rejected row.
type importBudgetRun[T any] struct {
	importer importModels.RowImporter[T]
	ctx      context.Context
	counter  *testpkg.QueryCounter
	userID   int64
	audit    importModels.BatchAudit
}

// run imports rows and returns the statements of exactly this call.
func (r importBudgetRun[T]) run(t *testing.T, rows []T, mode importModels.ImportMode, dryRun bool) querybudgets.Measurement {
	t.Helper()
	r.counter.Reset()
	result, err := r.importer.ImportBatches(r.ctx, importModels.ImportRequest[T]{
		Rows: rows, Mode: mode, DryRun: dryRun, UserID: r.userID, SkipInvalidRows: true,
	}, r.audit)
	require.NoError(t, err)
	require.Zero(t, result.ErrorCount, "%+v", result.Errors)
	if mode == importModels.ImportModeUpdate && !dryRun {
		require.Equal(t, len(rows), result.UpdatedCount)
	} else {
		require.Equal(t, len(rows), result.CreatedCount)
	}
	t.Logf("%d rows, mode %s, dry run %t: %d statements", len(rows), mode, dryRun, r.counter.Total())
	return r.counter.Measurement(r.counter.Queries())
}

// previewAndCreate are the measurements of one file's preview and create.
type previewAndCreate struct{ preview, create querybudgets.Measurement }

// TestStudentImportUpdateBatchQueryBudget pins the update of a student file
// whose rows change fixed cells (Befund 6 of the addendum): every row moves the
// child to another city and its pickup to a later time, so the profile patch
// and the care write both run. The create that precedes each update is not
// measured.
func TestStudentImportUpdateBatchQueryBudget(t *testing.T) {
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
	update := func(rows []importModels.StudentImportRow) querybudgets.Measurement {
		t.Helper()
		runner.run(t, rows, importModels.ImportModeCreate, false)
		changed := make([]importModels.StudentImportRow, len(rows))
		for i, row := range rows {
			row.AddressCity = "Bonn"
			row.PickupSchedules = []importModels.PickupScheduleImportData{{Weekday: 2, PickupTime: "16:45"}}
			changed[i] = row
		}
		measured := runner.run(t, changed, importModels.ImportModeUpdate, false)
		// Outside the counted context: the fixed changes reached both owners.
		var stale, early int
		require.NoError(t, db.NewSelect().TableExpr("users.student_profiles").ColumnExpr("count(*)").
			Where("tenant_id = ? AND address_city IS DISTINCT FROM 'Bonn'", testpkg.Tenant(t)).Scan(testpkg.Ctx(t), &stale))
		require.NoError(t, db.NewSelect().TableExpr("schedule.student_pickup_schedules").ColumnExpr("count(*)").
			Where("tenant_id = ? AND pickup_time <> '16:45'", testpkg.Tenant(t)).Scan(testpkg.Ctx(t), &early))
		require.Zero(t, stale, "every updated child moved to the new city")
		require.Zero(t, early, "every updated pickup moved to the new time")
		return measured
	}
	for iteration := range 2 {
		update(tenStudentBatch(iteration))
	}
	querybudgets.AssertQueryBudget(t, "modules.dataimport.student_batch.update", update(tenStudentBatch(2)))
	querybudgets.AssertQueryBudget(t, "modules.dataimport.student_batch.update_110", update(studentBatch(3, 11*studentBatchRows)))
}

// TestStaffImportBatchQueryBudget pins preview, create and update of a staff
// file with role, personnel number, weekly hours and one qualification and
// without e-mail, so no invitation is sent. The update matches each row by its
// personnel number and changes the city and the weekly hours.
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
	imported := 0
	measure := func(iteration, n int) (previewAndCreate, querybudgets.Measurement) {
		t.Helper()
		file := make([]importModels.StaffImportRow, 0, n)
		changed := make([]importModels.StaffImportRow, 0, n)
		for i := range n {
			row := importModels.StaffImportRow{
				FirstName: fmt.Sprintf("Budget%d-%d", iteration, i), LastName: "Staff", RoleName: role.Name,
				PersonnelNumber: fmt.Sprintf("P-%d-%d", iteration, i), WeeklyHours: "19,5", Qualifications: "Erste Hilfe",
			}
			file = append(file, row)
			row.AddressCity, row.WeeklyHours = "Bonn", "30"
			changed = append(changed, row)
		}
		measured := previewAndCreate{
			preview: runner.run(t, file, importModels.ImportModeCreate, true),
			create:  runner.run(t, file, importModels.ImportModeCreate, false),
		}
		update := runner.run(t, changed, importModels.ImportModeUpdate, false)
		// Outside the counted context: the fixed changes reached Workforce.
		var updated int
		require.NoError(t, db.NewSelect().TableExpr("users.staff_master_data").ColumnExpr("count(*)").
			Where("tenant_id = ? AND address_city = 'Bonn' AND weekly_hours = 30", testpkg.Tenant(t)).Scan(testpkg.Ctx(t), &updated))
		imported += n
		require.Equal(t, imported, updated)
		return measured, update
	}
	for iteration := range 2 {
		measure(iteration, 10)
	}
	small, smallUpdate := measure(2, 10)
	querybudgets.AssertQueryBudget(t, "modules.dataimport.staff_batch.preview", small.preview)
	querybudgets.AssertQueryBudget(t, "modules.dataimport.staff_batch.import", small.create)
	querybudgets.AssertQueryBudget(t, "modules.dataimport.staff_batch.update", smallUpdate)
	large, largeUpdate := measure(3, 110)
	querybudgets.AssertQueryBudget(t, "modules.dataimport.staff_batch.preview_110", large.preview)
	querybudgets.AssertQueryBudget(t, "modules.dataimport.staff_batch.import_110", large.create)
	querybudgets.AssertQueryBudget(t, "modules.dataimport.staff_batch.update_110", largeUpdate)
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
			preview: runner.run(t, file, importModels.ImportModeCreate, true),
			create:  runner.run(t, file, importModels.ImportModeCreate, false),
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
			preview: runner.run(t, file, importModels.ImportModeCreate, true),
			create:  runner.run(t, file, importModels.ImportModeCreate, false),
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
