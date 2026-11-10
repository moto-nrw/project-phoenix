package integration

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/api"
	importModels "github.com/moto-nrw/project-phoenix/modules/dataimport"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// Failure in row k (ADR 0067 addendum of 2026-11-08, "Jeder Schritt prüft"):
// a trigger refuses exactly row k of a file of 10 or 110 rows, once per owner
// table. Status, code, row error, acknowledged counters and persistence must
// be those of the row path: the chunk of row k rolls back completely, earlier
// chunks stay. This test ran green unchanged on the head before #5552 step 3.

// refusedMarkers is, per owner table, the column and the value only row k
// carries. Each table's trigger refuses exactly that value, so the triggers
// of one isolated database serve all parallel subtests.
var refusedMarkers = map[string][2]string{
	"users.persons":                     {"first_name", "Refused"},
	"users.student_profiles":            {"address_street", "Refusedweg 1"},
	"schedule.student_pickup_schedules": {"pickup_time", "17:17:00"},
	"users.staff_master_data":           {"address_city", "Refusedstadt"},
	"users.staff_qualifications":        {"name", "Refusedkurs"},
	"users.class_list_entries":          {"last_name", "Refused"},
	"active.staff_balance_adjustments":  {"minutes_delta", "-150"},
}

func installRefusedMarkers(t *testing.T, db *bun.DB) {
	t.Helper()
	ctx := context.Background()
	_, err := db.ExecContext(ctx, `CREATE OR REPLACE FUNCTION public.refuse_import_marked_row() RETURNS trigger LANGUAGE plpgsql AS $$
		BEGIN
			IF to_jsonb(NEW) ->> TG_ARGV[0] = TG_ARGV[1] THEN
				RAISE EXCEPTION 'injected owner write failure' USING ERRCODE = '23514';
			END IF;
			RETURN NULL;
		END $$`)
	require.NoError(t, err)
	for table, marker := range refusedMarkers {
		// table, column and value come from the fixed map above, never input.
		_, err := db.ExecContext(ctx, fmt.Sprintf(`CREATE TRIGGER refuse_import_marked_row AFTER INSERT OR UPDATE ON %s FOR EACH ROW EXECUTE FUNCTION public.refuse_import_marked_row('%s', '%s')`, table, marker[0], marker[1]))
		require.NoError(t, err)
	}
}

// positionOutcome is what a failed upload observably left behind.
type positionOutcome struct {
	created, updated int
	failures         map[int][]string // row number → blocking codes with message
	persisted        int              // rows of this upload the owner tables keep
}

func blockingFailures[T any](result *importModels.ImportResult[T]) map[int][]string {
	failures := map[int][]string{}
	for _, row := range result.Errors {
		for _, e := range row.Errors {
			if e.Severity == importModels.ErrorSeverityError {
				failures[row.RowNumber] = append(failures[row.RowNumber], e.Code+": "+e.Message)
			}
		}
	}
	return failures
}

func countIn(t *testing.T, db *bun.DB, query string, args ...any) int {
	t.Helper()
	var n int
	require.NoError(t, db.NewRaw(query, append([]any{testpkg.Tenant(t)}, args...)...).Scan(context.Background(), &n))
	return n
}

// TestImportBatchesRefusedRowKeepsTheRowPathOutcome runs every owner table at
// k = 1, 5, 10 of 10 and k = 1, 55, 101, 110 of 110.
func TestImportBatchesRefusedRowKeepsTheRowPathOutcome(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	installRefusedMarkers(t, db)
	positions := []struct{ n, k int }{{10, 1}, {10, 5}, {10, 10}, {110, 1}, {110, 55}, {110, 101}, {110, 110}}
	cases := []struct {
		name, table, code string
		run               func(t *testing.T, n, k int) positionOutcome
	}{
		{"student create", "users.persons", "creation_failed", studentCreateAt(db, func(row *importModels.StudentImportRow) { row.FirstName = "Refused" })},
		{"student create", "users.student_profiles", "creation_failed", studentCreateAt(db, func(row *importModels.StudentImportRow) { row.AddressStreet = "Refusedweg 1" })},
		{"student create", "schedule.student_pickup_schedules", "creation_failed", studentCreateAt(db, func(row *importModels.StudentImportRow) {
			row.PickupSchedules = []importModels.PickupScheduleImportData{{Weekday: 2, PickupTime: "17:17"}}
		})},
		{"student update", "users.student_profiles", "update_failed", studentUpdateAt(db)},
		{"staff create", "users.persons", "creation_failed", staffCreateAt(db, func(row *importModels.StaffImportRow) { row.FirstName = "Refused" })},
		{"staff create", "users.staff_master_data", "creation_failed", staffCreateAt(db, func(row *importModels.StaffImportRow) { row.AddressCity = "Refusedstadt" })},
		{"staff create", "users.staff_qualifications", "creation_failed", staffCreateAt(db, func(row *importModels.StaffImportRow) { row.Qualifications = "Refusedkurs" })},
		{"staff update", "users.staff_master_data", "update_failed", staffUpdateAt(db)},
		{"class list", "users.class_list_entries", "creation_failed", classListAt(db)},
		{"opening balance", "active.staff_balance_adjustments", "creation_failed", openingBalanceAt(db)},
	}
	// The subtests run concurrently without t.Parallel: this test holds the
	// package's one isolated clone (test/database_scope.go), and parallel
	// subtests would wait for -parallel slots that other top-level tests hold
	// while they wait for that clone, a deadlock until the test timeout.
	slots := make(chan struct{}, 4)
	var running sync.WaitGroup
	for _, c := range cases {
		for _, p := range positions {
			slots <- struct{}{}
			running.Add(1)
			go func() {
				defer running.Done()
				defer func() { <-slots }()
				t.Run(fmt.Sprintf("%s/%s/k%d_of_%d", c.name, c.table, p.k, p.n), func(t *testing.T) {
					testpkg.OwnTenant(t)
					outcome := c.run(t, p.n, p.k)
					committed := 0
					if p.k > 100 {
						committed = 100 // the first chunk committed before row k's chunk failed
					}
					require.Equal(t, committed, outcome.created+outcome.updated, "acknowledged counters")
					require.Equal(t, committed, outcome.persisted, "only earlier chunks persist")
					require.Equal(t, map[int][]string{p.k + 1: {c.code + ": injected owner write failure"}}, trimFailures(outcome.failures))
				})
			}()
		}
	}
	running.Wait()
}

// trimFailures keeps the code and the injected cause of each message.
func trimFailures(failures map[int][]string) map[int][]string {
	trimmed := make(map[int][]string, len(failures))
	for row, messages := range failures {
		for _, message := range messages {
			code, _, _ := strings.Cut(message, ": ")
			if strings.Contains(message, "injected owner write failure") {
				message = code + ": injected owner write failure"
			}
			trimmed[row] = append(trimmed[row], message)
		}
	}
	return trimmed
}

func uploadContext(t *testing.T, db *bun.DB, permissions ...string) context.Context {
	ctx := tenant.WithUnitOfWork(testpkg.Ctx(t), testpkg.TenantRuntime(t, db))
	if len(permissions) > 0 {
		ctx = importModels.ContextWithImporterPermissions(ctx, permissions)
	}
	return ctx
}

func studentCreateAt(db *bun.DB, mark func(*importModels.StudentImportRow)) func(*testing.T, int, int) positionOutcome {
	return func(t *testing.T, n, k int) positionOutcome {
		module, err := api.NewImportTestModule(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db))
		require.NoError(t, err)
		actor := newImporter(t, db)
		rows := studentBatch(0, n)
		mark(&rows[k-1])
		result, err := module.Import.ImportBatches(uploadContext(t, db), importModels.ImportRequest[importModels.StudentImportRow]{
			Rows: rows, Mode: importModels.ImportModeCreate, UserID: actor.staffID, SkipInvalidRows: true,
		}, importModels.BatchAudit{EntityType: "student", Filename: "position.csv", AccountID: actor.accountID})
		require.Error(t, err)
		return positionOutcome{
			created: result.CreatedCount, updated: result.UpdatedCount, failures: blockingFailures(result),
			persisted: countIn(t, db, `SELECT count(*) FROM users.student_profiles WHERE tenant_id = ?`),
		}
	}
}

func studentUpdateAt(db *bun.DB) func(*testing.T, int, int) positionOutcome {
	return func(t *testing.T, n, k int) positionOutcome {
		module, err := api.NewImportTestModule(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db))
		require.NoError(t, err)
		actor := newImporter(t, db)
		audit := importModels.BatchAudit{EntityType: "student", Filename: "position.csv", AccountID: actor.accountID}
		rows := studentBatch(0, n)
		created, err := module.Import.ImportBatches(uploadContext(t, db), importModels.ImportRequest[importModels.StudentImportRow]{
			Rows: rows, Mode: importModels.ImportModeCreate, UserID: actor.staffID, SkipInvalidRows: true,
		}, audit)
		require.NoError(t, err)
		require.Equal(t, n, created.CreatedCount)
		for i := range rows {
			rows[i].AddressCity = "Bonn"
		}
		rows[k-1].AddressStreet = "Refusedweg 1"
		audit.Filename = "position-update.csv"
		result, err := module.Import.ImportBatches(uploadContext(t, db), importModels.ImportRequest[importModels.StudentImportRow]{
			Rows: rows, Mode: importModels.ImportModeUpdate, UserID: actor.staffID, SkipInvalidRows: true,
		}, audit)
		require.Error(t, err)
		return positionOutcome{
			created: result.CreatedCount, updated: result.UpdatedCount, failures: blockingFailures(result),
			persisted: countIn(t, db, `SELECT count(*) FROM users.student_profiles WHERE tenant_id = ? AND address_city = 'Bonn'`),
		}
	}
}

func positionStaffRows(n int) []importModels.StaffImportRow {
	rows := make([]importModels.StaffImportRow, n)
	for i := range rows {
		rows[i] = importModels.StaffImportRow{
			FirstName: fmt.Sprintf("Position%d", i), LastName: "Staff", RoleName: "Betreuung",
			PersonnelNumber: fmt.Sprintf("P-%d", i), WeeklyHours: "19,5", Qualifications: "Erste Hilfe", AddressCity: "Köln",
		}
	}
	return rows
}

func staffCreateAt(db *bun.DB, mark func(*importModels.StaffImportRow)) func(*testing.T, int, int) positionOutcome {
	return func(t *testing.T, n, k int) positionOutcome {
		module, err := api.NewImportTestModule(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db))
		require.NoError(t, err)
		actor := newImporter(t, db)
		before := countIn(t, db, `SELECT count(*) FROM users.staff_school_memberships WHERE tenant_id = ?`)
		rows := positionStaffRows(n)
		mark(&rows[k-1])
		result, err := module.StaffImport.ImportBatches(uploadContext(t, db, "admin:*"), importModels.ImportRequest[importModels.StaffImportRow]{
			Rows: rows, Mode: importModels.ImportModeCreate, UserID: actor.accountID, SkipInvalidRows: true,
		}, importModels.BatchAudit{EntityType: "staff", Filename: "position.csv", AccountID: actor.accountID})
		require.Error(t, err)
		return positionOutcome{
			created: result.CreatedCount, updated: result.UpdatedCount, failures: blockingFailures(result),
			persisted: countIn(t, db, `SELECT count(*) FROM users.staff_school_memberships WHERE tenant_id = ?`) - before,
		}
	}
}

func staffUpdateAt(db *bun.DB) func(*testing.T, int, int) positionOutcome {
	return func(t *testing.T, n, k int) positionOutcome {
		module, err := api.NewImportTestModule(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db))
		require.NoError(t, err)
		actor := newImporter(t, db)
		ctx := uploadContext(t, db, "admin:*")
		rows := positionStaffRows(n)
		created, err := module.StaffImport.ImportBatches(ctx, importModels.ImportRequest[importModels.StaffImportRow]{
			Rows: rows, Mode: importModels.ImportModeCreate, UserID: actor.accountID, SkipInvalidRows: true,
		}, importModels.BatchAudit{EntityType: "staff", Filename: "position.csv", AccountID: actor.accountID})
		require.NoError(t, err)
		require.Equal(t, n, created.CreatedCount)
		for i := range rows {
			rows[i].AddressCity = "Bonn"
		}
		rows[k-1].AddressCity = "Refusedstadt"
		result, err := module.StaffImport.ImportBatches(ctx, importModels.ImportRequest[importModels.StaffImportRow]{
			Rows: rows, Mode: importModels.ImportModeUpdate, UserID: actor.accountID, SkipInvalidRows: true,
		}, importModels.BatchAudit{EntityType: "staff", Filename: "position-update.csv", AccountID: actor.accountID})
		require.Error(t, err)
		return positionOutcome{
			created: result.CreatedCount, updated: result.UpdatedCount, failures: blockingFailures(result),
			persisted: countIn(t, db, `SELECT count(*) FROM users.staff_master_data WHERE tenant_id = ? AND address_city = 'Bonn'`),
		}
	}
}

func classListAt(db *bun.DB) func(*testing.T, int, int) positionOutcome {
	return func(t *testing.T, n, k int) positionOutcome {
		module, err := api.NewImportTestModule(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db))
		require.NoError(t, err)
		actor := newImporter(t, db)
		rows := make([]importModels.ClassListEntryImportRow, n)
		for i := range rows {
			rows[i] = importModels.ClassListEntryImportRow{FirstName: fmt.Sprintf("Kind%d", i), LastName: "Position", SchoolClass: "1a"}
		}
		rows[k-1].LastName = "Refused"
		result, err := module.ClassListImport.ImportBatches(uploadContext(t, db), importModels.ImportRequest[importModels.ClassListEntryImportRow]{
			Rows: rows, Mode: importModels.ImportModeCreate, UserID: actor.staffID, SkipInvalidRows: true,
		}, importModels.BatchAudit{EntityType: "class_list_entries", Filename: "position.csv", AccountID: actor.accountID})
		require.Error(t, err)
		return positionOutcome{
			created: result.CreatedCount, updated: result.UpdatedCount, failures: blockingFailures(result),
			persisted: countIn(t, db, `SELECT count(*) FROM users.class_list_entries WHERE tenant_id = ?`),
		}
	}
}

func openingBalanceAt(db *bun.DB) func(*testing.T, int, int) positionOutcome {
	return func(t *testing.T, n, k int) positionOutcome {
		module, err := api.NewImportTestModuleWithOptions(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db), api.ImportTestOptions{
			Clock: func() time.Time { return time.Date(2026, time.September, 12, 12, 0, 0, 0, time.UTC) },
		})
		require.NoError(t, err)
		actor := newImporter(t, db)
		rows := make([]importModels.OpeningBalanceImportRow, n)
		for i := range rows {
			first := fmt.Sprintf("Position%d", i)
			testpkg.CreateTestStaff(t, db, first, "Salden")
			rows[i] = importModels.OpeningBalanceImportRow{FirstName: first, LastName: "Salden", HoursBalance: "-3,25", VacationEntitled: "30", VacationCarryover: "4", VacationRemaining: "12,5"}
		}
		rows[k-1].HoursBalance = "-2,5"
		svc := module.OpeningBalanceImport(testpkg.Date(2026, time.September, 1), "Übernahme", actor.staffID)
		result, err := svc.ImportBatches(uploadContext(t, db), importModels.ImportRequest[importModels.OpeningBalanceImportRow]{
			Rows: rows, Mode: importModels.ImportModeCreate, UserID: actor.accountID, SkipInvalidRows: true,
		}, importModels.BatchAudit{EntityType: "opening_balance", Filename: "position.csv", AccountID: actor.accountID, Options: "2026-09-01"})
		require.Error(t, err)
		return positionOutcome{
			created: result.CreatedCount, updated: result.UpdatedCount, failures: blockingFailures(result),
			persisted: countIn(t, db, `SELECT count(*) FROM active.staff_balance_adjustments WHERE tenant_id = ?`),
		}
	}
}
