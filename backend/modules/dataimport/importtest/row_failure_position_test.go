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
	// The consents and the consent history of a student segment go to their
	// tables in one bundled statement each (#5552 step 5b); the row triggers
	// still fire per row, so row k fails inside that statement.
	"users.privacy_consents":        {"data_retention_days", "17"},
	"audit.student_consent_changes": {"consent_key", "photo"},
	// A guardian without e-mail of a student segment goes to its five tables
	// in one bundled statement each (#5552 step 5c).
	"users.guardian_profiles":                   {"last_name", "Refusedeltern"},
	"users.guardian_phone_numbers":              {"label", "Refusedlabel"},
	"users.student_guardian_relationships":      {"emergency_priority", "7"},
	"users.student_guardian_pickup_permissions": {"pickup_notes", "Refusednotiz"},
	"auth.guardian_student_access":              {"permissions", "{}"}, // pickup_only grants no permission
	// The enrollment batch of step 7: the profiles in one statement before the
	// rows (marked by the profile row's address above), each membership in
	// the row loop, the care profiles in one statement after the rows.
	"users.student_school_memberships": {"school_class", "9Z"},
	"users.student_care_profiles":      {"departure_days", `{"fri": "bus"}`},
	"users.staff_master_data":          {"address_city", "Refusedstadt"},
	"users.staff_qualifications":       {"name", "Refusedkurs"},
	"users.class_list_entries":         {"last_name", "Refused"},
	"active.staff_balance_adjustments": {"minutes_delta", "-150"},
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

// TestImportBatchesRefusedRowKeepsTheRowPathOutcome refuses row k of n in every
// owner table. The first table of each import kind runs every position
// (k = 1, 5, 10 of 10 and k = 1, 55, 101, 110 of 110); where the refused row
// sits is decided by the chunk executor, not by the table, so every other
// table runs one position of each file size in rotation. The full cross held
// an isolated clone for 140 uploads, 80 of them with 110 rows (#6784).
func TestImportBatchesRefusedRowKeepsTheRowPathOutcome(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	installRefusedMarkers(t, db)
	type position struct{ n, k int }
	small := []position{{10, 1}, {10, 5}, {10, 10}}
	large := []position{{110, 1}, {110, 55}, {110, 101}, {110, 110}}
	cases := []struct {
		name, table, code string
		run               func(t *testing.T, n, k int) positionOutcome
	}{
		{"student create", "users.persons", "creation_failed", studentCreateAt(db, func(row *importModels.StudentImportRow) { row.FirstName = "Refused" })},
		{"student create", "users.student_profiles", "creation_failed", studentCreateAt(db, func(row *importModels.StudentImportRow) { row.AddressStreet = "Refusedweg 1" })},
		{"student create", "schedule.student_pickup_schedules", "creation_failed", studentCreateAt(db, func(row *importModels.StudentImportRow) {
			row.PickupSchedules = []importModels.PickupScheduleImportData{{Weekday: 2, PickupTime: "17:17"}}
		})},
		{"student create", "users.privacy_consents", "creation_failed", studentCreateBundledAt(db, func(row *importModels.StudentImportRow) { row.DataRetentionDays = 17 })},
		// Only row k carries a photo consent date, so only it records a
		// "photo" transition.
		{"student create", "audit.student_consent_changes", "creation_failed", studentCreateBundledAt(db, func(row *importModels.StudentImportRow) { row.PhotoConsentGivenAt = "2026-07-17" })},
		{"student create", "users.guardian_profiles", "creation_failed", studentCreateGuardianBundledAt(db, func(g *importModels.GuardianImportData) { g.LastName = "Refusedeltern" })},
		{"student create", "users.guardian_phone_numbers", "creation_failed", studentCreateGuardianBundledAt(db, func(g *importModels.GuardianImportData) { g.PhoneNumbers[0].Label = "Refusedlabel" })},
		{"student create", "users.student_guardian_relationships", "creation_failed", studentCreateGuardianBundledAt(db, func(g *importModels.GuardianImportData) { g.EmergencyPriority = 7 })},
		{"student create", "users.student_guardian_pickup_permissions", "creation_failed", studentCreateGuardianBundledAt(db, func(g *importModels.GuardianImportData) { g.PickupNotes = "Refusednotiz" })},
		{"student create", "auth.guardian_student_access", "creation_failed", studentCreateGuardianBundledAt(db, func(g *importModels.GuardianImportData) { g.GuardianRole = "pickup_only" })},
		{"enrollment batch", "users.student_profiles", "creation_failed", studentCreateEnrollmentAt(db, func(row *importModels.StudentImportRow) { row.AddressStreet = "Refusedweg 1" })},
		{"enrollment batch", "users.student_school_memberships", "creation_failed", studentCreateEnrollmentAt(db, func(row *importModels.StudentImportRow) { row.SchoolClass = "9Z" })},
		{"enrollment batch", "users.student_care_profiles", "creation_failed", studentCreateEnrollmentAt(db, func(row *importModels.StudentImportRow) { row.DepartureDays = map[string]string{"fri": "bus"} })},
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
	fullCross := map[string]bool{}
	for i, c := range cases {
		positions := []position{small[i%len(small)], large[i%len(large)]}
		if !fullCross[c.name] {
			fullCross[c.name] = true
			positions = append(append([]position{}, small...), large...)
		}
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

// studentCreateBundledAt is studentCreateAt for a write the batch command
// bundles (#5552 step 5b): row k fails inside the bundled statement, the
// chunk rolls back to its savepoint and replays row by row, and the replay
// names row k as the row path does.
func studentCreateBundledAt(db *bun.DB, mark func(*importModels.StudentImportRow)) func(*testing.T, int, int) positionOutcome {
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
		observation := (*module.Observations)[0]
		require.Equal(t, 1, observation.BatchFallbacks, "the bundled statement failed and the chunk replayed")
		require.Zero(t, observation.BatchesRetried)
		return positionOutcome{
			created: result.CreatedCount, updated: result.UpdatedCount, failures: blockingFailures(result),
			persisted: countIn(t, db, `SELECT count(*) FROM users.student_profiles WHERE tenant_id = ?`),
		}
	}
}

// studentCreateGuardianBundledAt is studentCreateBundledAt for the bundled
// guardian statements (#5552 step 5c): every guardian of the file has no
// e-mail, row k's guardian carries the marker. Besides the children, every
// guardian table keeps exactly the guardians of the committed chunks.
func studentCreateGuardianBundledAt(db *bun.DB, mark func(*importModels.GuardianImportData)) func(*testing.T, int, int) positionOutcome {
	return func(t *testing.T, n, k int) positionOutcome {
		module, err := api.NewImportTestModule(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db))
		require.NoError(t, err)
		actor := newImporter(t, db)
		rows := studentBatch(0, n)
		for i := range rows {
			rows[i].Guardians[0].Email = ""
		}
		mark(&rows[k-1].Guardians[0])
		result, err := module.Import.ImportBatches(uploadContext(t, db), importModels.ImportRequest[importModels.StudentImportRow]{
			Rows: rows, Mode: importModels.ImportModeCreate, UserID: actor.staffID, SkipInvalidRows: true,
		}, importModels.BatchAudit{EntityType: "student", Filename: "position.csv", AccountID: actor.accountID})
		require.Error(t, err)
		observation := (*module.Observations)[0]
		require.Equal(t, 1, observation.BatchFallbacks, "the bundled guardian statement failed and the chunk replayed")
		require.Zero(t, observation.BatchesRetried)
		persisted := countIn(t, db, `SELECT count(*) FROM users.student_profiles WHERE tenant_id = ?`)
		for _, table := range []string{"users.guardian_profiles", "users.guardian_phone_numbers", "users.student_guardian_relationships",
			"users.student_guardian_pickup_permissions", "auth.guardian_student_access"} {
			// The tables come from the fixed list above, never input.
			require.Equal(t, persisted, countIn(t, db, `SELECT count(*) FROM `+table+` WHERE tenant_id = ?`), "%s keeps the committed chunks' guardians only", table)
		}
		return positionOutcome{
			created: result.CreatedCount, updated: result.UpdatedCount, failures: blockingFailures(result), persisted: persisted,
		}
	}
}

// studentCreateEnrollmentAt is studentCreateBundledAt for the enrollment
// batch (#5552 step 7): row k fails in the profile statement, in its
// membership or in the care statement; the chunk replays row by row and the
// replay names row k. The three enrollment tables keep exactly the children
// of the committed chunks, each once.
func studentCreateEnrollmentAt(db *bun.DB, mark func(*importModels.StudentImportRow)) func(*testing.T, int, int) positionOutcome {
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
		observation := (*module.Observations)[0]
		require.Equal(t, 1, observation.BatchFallbacks, "the enrollment batch failed and the chunk replayed")
		require.Zero(t, observation.BatchesRetried)
		persisted := countIn(t, db, `SELECT count(*) FROM users.student_profiles WHERE tenant_id = ?`)
		for _, query := range []string{
			`SELECT count(DISTINCT person_id) FROM users.student_profiles WHERE tenant_id = ?`,
			`SELECT count(*) FROM users.student_school_memberships WHERE tenant_id = ?`,
			`SELECT count(DISTINCT student_profile_id) FROM users.student_school_memberships WHERE tenant_id = ?`,
			`SELECT count(*) FROM users.student_care_profiles WHERE tenant_id = ?`,
			`SELECT count(DISTINCT membership_id) FROM users.student_care_profiles WHERE tenant_id = ?`,
		} {
			require.Equal(t, persisted, countIn(t, db, query), "%s: the committed chunks' children, each once", query)
		}
		return positionOutcome{
			created: result.CreatedCount, updated: result.UpdatedCount, failures: blockingFailures(result), persisted: persisted,
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
