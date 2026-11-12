package integration

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/driver/pgdriver"

	"github.com/moto-ogs/project-phoenix/api"
	importModels "github.com/moto-ogs/project-phoenix/modules/dataimport"
	importPorts "github.com/moto-ogs/project-phoenix/modules/dataimport/ports"
	testpkg "github.com/moto-ogs/project-phoenix/test"

	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
)

// Step 7 of #5552 (PR #6558): in a batch segment the profiles of the
// children without group or permanent hint are created in one statement
// before the rows, each membership where the row path creates it, and the
// care profiles in one statement after the rows.

const (
	enrollmentProfilesCommand = "create_enrollment_profiles"
	enrollmentCaresCommand    = "save_enrollment_cares"
	// classWriteGateKey is School Membership's class-writes advisory lock.
	classWriteGateKey = int32(0x636c6173)
)

// holdClassWriteGate takes the class-writes gate exclusively in a session of
// its own, which every child creation of the school waits for.
func holdClassWriteGate(t *testing.T, ctx context.Context, db *bun.DB) (bun.Tx, int) {
	t.Helper()
	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = tx.Rollback() })
	pid := sessionPID(t, ctx, tx)
	_, err = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(?, ?)`, classWriteGateKey, int32(testpkg.Tenant(t)))
	require.NoError(t, err)
	return tx, pid
}

// membershipStatuses are the membership statuses of the tenant's children by
// first name.
func membershipStatuses(t *testing.T, db *bun.DB) map[string]string {
	t.Helper()
	var rows []struct {
		Name   string `bun:"first_name"`
		Status string `bun:"status"`
	}
	require.NoError(t, db.NewRaw(`SELECT p.first_name, m.status FROM users.student_school_memberships m
		JOIN users.student_profiles s ON s.id = m.student_profile_id JOIN users.persons p ON p.id = s.person_id
		WHERE m.tenant_id = ?`, testpkg.Tenant(t)).Scan(context.Background(), &rows))
	statuses := make(map[string]string, len(rows))
	for _, row := range rows {
		statuses[row.Name] = row.Status
	}
	return statuses
}

// TestStudentEnrollmentBatchTakesEachStatusWhereTheRowPathDoes (Befundklasse
// 5, table review R2 finding 1): the row path takes a child's status from the
// calendar day right before it creates the child; only the first creation
// of a chunk then waits for the class gate, every later one comes after that
// wait. A holder keeps the gate while the import's clock shows day D, the
// upload waits for it (its own session, pg_blocking_pids), the clock turns
// to D+1, the holder releases. The first two children start on D+1: the
// first took its status on D (pending), the second on D+1 (active), on the
// batch path as on the row path, even if the first child has a group or a
// permanent hint and keeps its enrollment in place.
func TestStudentEnrollmentBatchTakesEachStatusWhereTheRowPathDoes(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	day := calendar.NewDate(2026, time.November, 10)
	for _, run := range []struct {
		name       string
		options    api.ImportTestOptions
		leadingRow string
	}{
		{"batch path", api.ImportTestOptions{}, ""},
		{"row path", api.ImportTestOptions{RowPathOnly: true}, ""},
		{"batch path/leading group", api.ImportTestOptions{}, "group"},
		{"row path/leading group", api.ImportTestOptions{RowPathOnly: true}, "group"},
		{"batch path/leading hint", api.ImportTestOptions{}, "hint"},
		{"row path/leading hint", api.ImportTestOptions{RowPathOnly: true}, "hint"},
	} {
		t.Run(run.name, func(t *testing.T) {
			testpkg.OwnTenant(t)
			var turned atomic.Bool
			options := run.options
			options.Today = func() calendar.Date {
				if turned.Load() {
					return day.AddDays(1)
				}
				return day
			}
			pool, name := namedImportPool(t, testpkg.PackageTestDSN(t), 0)
			module, err := api.NewImportTestModuleWithOptions(testpkg.NewDatabase(t, pool), testpkg.TenantRuntime(t, pool), options)
			require.NoError(t, err)
			actor := newImporter(t, db)
			rows := namedStudentBatch("Mitternacht", studentChunkRows)
			rows[0].EnrolledFrom = day.AddDays(1).String()
			rows[1].EnrolledFrom = day.AddDays(1).String()
			switch run.leadingRow {
			case "group":
				group := testpkg.CreateTestEducationGroup(t, db, fmt.Sprintf("Gruppe %d", testpkg.Tenant(t)))
				rows[0].GroupName = group.Name
			case "hint":
				rows[0].SupervisorNotes = "Dauerhinweis"
			}
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			holder, pid := holdClassWriteGate(t, ctx, db)
			done := uploadStudents(t, module, pool, actor, rows, "midnight.csv")
			waitBlockedBy(t, db, name, pid)
			turned.Store(true)
			require.NoError(t, holder.Rollback())

			upload := awaitStudents(t, done)
			require.NoError(t, upload.err)
			require.Equal(t, studentChunkRows, upload.result.CreatedCount)
			observation := (*module.Observations)[0]
			require.Zero(t, observation.BatchFallbacks)
			if run.options.RowPathOnly {
				require.Zero(t, commandCount(observation, enrollmentProfilesCommand))
			} else {
				require.Equal(t, 1, commandCount(observation, enrollmentProfilesCommand), "the batch path ran")
				inPlace := 0
				if run.leadingRow != "" {
					inPlace = 1
				}
				require.Equal(t, inPlace, commandCount(observation, "create_enrollment_student"), "group and hint rows keep their enrollment in place")
				require.Equal(t, studentChunkRows-inPlace, commandCount(observation, "enroll_created_student"))
			}
			statuses := membershipStatuses(t, db)
			require.Equal(t, "pending", statuses["Mitternacht0"], "the first child took its status before the gate, on day D")
			require.Equal(t, "active", statuses["Mitternacht1"], "the second child took its status after the gate, on day D+1")
			for i := 2; i < studentChunkRows; i++ {
				require.Equal(t, "active", statuses[rows[i].FirstName])
			}
		})
	}
}

// variedEnrollmentFile has eight children with values that differ per
// child in every column the profile and the care profile take: address,
// extra info, health information, consent dates, departure plan, class and
// enrollment window (one starting in thirty days: pending).
func variedEnrollmentFile(prefix string) []importModels.StudentImportRow {
	rows := namedStudentBatch(prefix, studentChunkRows)
	plans := []map[string]string{{"mon": "bus"}, {"tue": "pickup"}, {"wed": "alone"}, {"thu": "bus", "fri": "pickup"}}
	for i := range rows {
		row := &rows[i]
		row.AddressStreet = fmt.Sprintf("Weg %d", i+1)
		row.AddressCity = fmt.Sprintf("Stadt%d", i)
		row.AddressPostalCode = fmt.Sprintf("5066%d", i)
		row.ExtraInfo = fmt.Sprintf("Info %d", i)
		row.HealthInfo = fmt.Sprintf("Allergie %d", i)
		row.AGBAcceptedAt = fmt.Sprintf("2026-09-%02d", i+1)
		row.DataProcessingAcceptedAt = fmt.Sprintf("2026-08-%02d", i+1)
		row.EmailContactAcceptedAt = fmt.Sprintf("2026-07-%02d", i+1)
		row.PhotoConsentGivenAt = fmt.Sprintf("2026-06-%02d", i+1)
		row.DepartureDays = plans[i%len(plans)]
		row.SchoolClass = []string{"4A", "4B", "3C"}[i%3]
		row.EnrolledFrom = fmt.Sprintf("2026-0%d-01", 1+i%8)
		row.EnrolledUntil = fmt.Sprintf("2031-07-%02d", 10+i)
	}
	rows[5].EnrolledFrom = calendar.TodayDate().AddDays(30).String()
	return rows
}

// openedHealth reads every child's health information through People
// Directory, which opens it with the child's membership as the associated
// data: a care profile written for another membership does not open.
func openedHealth(t *testing.T, db *bun.DB, persons importPorts.StudentDirectory) map[string]string {
	t.Helper()
	var students []struct {
		ID   int64  `bun:"id"`
		Name string `bun:"first_name"`
	}
	require.NoError(t, db.NewRaw(`SELECT s.id, p.first_name FROM users.student_profiles s JOIN users.persons p ON p.id = s.person_id
		WHERE s.tenant_id = ?`, testpkg.Tenant(t)).Scan(context.Background(), &students))
	opened := make(map[string]string, len(students))
	for _, student := range students {
		record, err := persons.ReadEnrollmentStudent(testpkg.Ctx(t), student.ID, "")
		require.NoError(t, err, "the health field of %s opens with its membership", student.Name)
		if record.HealthInfo != nil {
			opened[student.Name] = *record.HealthInfo
		}
	}
	return opened
}

// TestStudentEnrollmentBatchMatchesTheRowPath (table C, Befundklasse 17): the
// three enrollment tables hold exactly what the row path writes, with values
// fixed per child by the file; the health information opens per child with
// its own membership and reads as the file has it.
func TestStudentEnrollmentBatchMatchesTheRowPath(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	run := func(t *testing.T, options api.ImportTestOptions) (studentOutcome, importModels.ImportObservation, map[string]string) {
		testpkg.OwnTenant(t)
		module, persons, _, err := api.NewImportTestModuleWithOwners(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db), options)
		require.NoError(t, err)
		actor := newImporter(t, db)
		upload := awaitStudents(t, uploadStudents(t, module, db, actor, variedEnrollmentFile("Werte"), "values.csv"))
		require.NoError(t, upload.err)
		require.Len(t, *module.Observations, 1)
		return studentOutcomeOf(t, db, upload.result), (*module.Observations)[0], openedHealth(t, db, persons)
	}
	var batch, rowPath studentOutcome
	var batchHealth, rowHealth map[string]string
	t.Run("batch path", func(t *testing.T) {
		var observation importModels.ImportObservation
		batch, observation, batchHealth = run(t, api.ImportTestOptions{})
		require.Zero(t, observation.BatchFallbacks)
		require.Equal(t, 1, commandCount(observation, enrollmentProfilesCommand))
		require.Equal(t, studentChunkRows, commandCount(observation, "enroll_created_student"), "each membership where the row path writes it")
		require.Equal(t, 1, commandCount(observation, enrollmentCaresCommand))
		require.Zero(t, commandCount(observation, "create_enrollment_student"))
		statuses := membershipStatuses(t, db)
		require.Equal(t, "pending", statuses["Werte5"], "a start in thirty days is pending")
		require.Equal(t, "active", statuses["Werte4"])
		require.Equal(t, studentChunkRows, countIn(t, db, `SELECT count(DISTINCT person_id) FROM users.student_profiles WHERE tenant_id = ?`))
		require.Equal(t, studentChunkRows, countIn(t, db, `SELECT count(DISTINCT membership_id) FROM users.student_care_profiles WHERE tenant_id = ?`))
	})
	t.Run("row path", func(t *testing.T) {
		var observation importModels.ImportObservation
		rowPath, observation, rowHealth = run(t, api.ImportTestOptions{RowPathOnly: true})
		require.Zero(t, commandCount(observation, enrollmentProfilesCommand))
	})
	require.Equal(t, rowPath, batch)
	want := make(map[string]string, studentChunkRows)
	for i := range studentChunkRows {
		want[fmt.Sprintf("Werte%d", i)] = fmt.Sprintf("Allergie %d", i)
	}
	require.Equal(t, want, batchHealth, "each child's own health information, opened with its own membership")
	require.Equal(t, want, rowHealth)
}

// TestStudentEnrollmentBatchKeepsGroupAndHintRowsInPlace: a child with a
// group and one with a permanent hint are created in place with
// CreateEnrollmentStudent; the other six go into the batch. The result is the
// row path's.
// TestStudentEnrollmentBatchNeedsTheMinimumOfBatchRows: a segment of eight
// children with seven permanent hints has one batch row, below ChunkMinRows
// (six); every child is then created in place, with the row path's
// outcome. With two hints the six batch rows take the batch.
func TestStudentEnrollmentBatchNeedsTheMinimumOfBatchRows(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	for _, tc := range []struct {
		name    string
		hints   int
		inPlace int
		batches int
	}{
		{"one batch row", 7, 8, 0},
		{"six batch rows", 2, 2, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			run := func(t *testing.T, options api.ImportTestOptions) (studentOutcome, importModels.ImportObservation) {
				testpkg.OwnTenant(t)
				module, err := api.NewImportTestModuleWithOptions(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db), options)
				require.NoError(t, err)
				actor := newImporter(t, db)
				rows := variedEnrollmentFile("Mindest")[:8]
				for i := 0; i < tc.hints; i++ {
					rows[i].SupervisorNotes = "Dauerhinweis"
				}
				upload := awaitStudents(t, uploadStudents(t, module, db, actor, rows, "minimum.csv"))
				require.NoError(t, upload.err)
				return studentOutcomeOf(t, db, upload.result), (*module.Observations)[0]
			}
			var batch, rowPath studentOutcome
			t.Run("batch path", func(t *testing.T) {
				var observation importModels.ImportObservation
				batch, observation = run(t, api.ImportTestOptions{})
				require.Zero(t, observation.BatchFallbacks)
				require.Equal(t, tc.batches, commandCount(observation, enrollmentProfilesCommand))
				require.Equal(t, tc.inPlace, commandCount(observation, "create_enrollment_student"))
				require.Equal(t, 8-tc.inPlace, commandCount(observation, "enroll_created_student"))
			})
			t.Run("row path", func(t *testing.T) { rowPath, _ = run(t, api.ImportTestOptions{RowPathOnly: true}) })
			require.Equal(t, rowPath, batch)
		})
	}
}

// TestStudentEnrollmentBelowTheMinimumTakesEachStatusInTheRowLoop: a
// segment of eight children with seven permanent hints stays below
// ChunkMinRows, so every child takes its status where the row path does,
// in the row loop after the reset of the first bound. The clock turns from
// D to D+1 exactly when that reset runs; the first two children start on
// D+1 and are therefore active.
func TestStudentEnrollmentBelowTheMinimumTakesEachStatusInTheRowLoop(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	testpkg.OwnTenant(t)
	day := calendar.NewDate(2026, time.November, 10)
	var armed, turned atomic.Bool
	pool := testpkg.AdoptPrivatePool(t, testpkg.NewBunDB(sql.OpenDB(faultConnector{
		Connector: pgdriver.NewConnector(pgdriver.WithDSN(testpkg.PackageTestDSN(t))),
		decide: func(query string) error {
			if armed.Load() && strings.HasPrefix(query, boundReset) {
				turned.Store(true)
			}
			return nil
		},
	})))
	t.Cleanup(func() { _ = pool.Close() })
	options := api.ImportTestOptions{Today: func() calendar.Date {
		if turned.Load() {
			return day.AddDays(1)
		}
		return day
	}}
	module, err := api.NewImportTestModuleWithOptions(testpkg.NewDatabase(t, pool), testpkg.TenantRuntime(t, pool), options)
	require.NoError(t, err)
	actor := newImporter(t, db)
	rows := namedStudentBatch("Grenze", 8)
	for i := 0; i < 7; i++ {
		rows[i].SupervisorNotes = "Dauerhinweis"
	}
	rows[0].EnrolledFrom = day.AddDays(1).String()
	rows[1].EnrolledFrom = day.AddDays(1).String()
	armed.Store(true)
	upload := awaitStudents(t, uploadStudents(t, module, pool, actor, rows, "grenze.csv"))
	require.NoError(t, upload.err)
	require.True(t, turned.Load(), "the first bound was reset")
	require.Equal(t, 8, upload.result.CreatedCount)
	observation := (*module.Observations)[0]
	require.Zero(t, observation.BatchFallbacks)
	require.Equal(t, 1, commandCount(observation, "create_persons"), "the segment took the batch path")
	require.Zero(t, commandCount(observation, enrollmentProfilesCommand))
	require.Equal(t, 8, commandCount(observation, "create_enrollment_student"))
	statuses := membershipStatuses(t, db)
	require.Equal(t, "active", statuses["Grenze0"], "the first child took its status after the bound reset, on day D+1")
	require.Equal(t, "active", statuses["Grenze1"])
}

func TestStudentEnrollmentBatchKeepsGroupAndHintRowsInPlace(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	run := func(t *testing.T, options api.ImportTestOptions) (studentOutcome, importModels.ImportObservation) {
		testpkg.OwnTenant(t)
		group := testpkg.CreateTestEducationGroup(t, db, fmt.Sprintf("Gruppe %d", testpkg.Tenant(t)))
		module, err := api.NewImportTestModuleWithOptions(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db), options)
		require.NoError(t, err)
		actor := newImporter(t, db)
		rows := variedEnrollmentFile("Ort")
		rows[2].GroupName = group.Name
		rows[6].SupervisorNotes = "Dauerhinweis"
		upload := awaitStudents(t, uploadStudents(t, module, db, actor, rows, "inplace.csv"))
		require.NoError(t, upload.err)
		return studentOutcomeOf(t, db, upload.result), (*module.Observations)[0]
	}
	var batch, rowPath studentOutcome
	t.Run("batch path", func(t *testing.T) {
		var observation importModels.ImportObservation
		batch, observation = run(t, api.ImportTestOptions{})
		require.Zero(t, observation.BatchFallbacks)
		require.Equal(t, 2, commandCount(observation, "create_enrollment_student"), "the group row and the hint row are created in place")
		require.Equal(t, studentChunkRows-2, commandCount(observation, "enroll_created_student"))
	})
	t.Run("row path", func(t *testing.T) { rowPath, _ = run(t, api.ImportTestOptions{RowPathOnly: true}) })
	require.Equal(t, rowPath, batch)
}

// TestStudentEnrollmentBatchGateHolder (Befundklasse 1 and table review R1
// finding 6, a deviation E13/E13′ cover): the class-writes gate sits in the
// bounded profile statement. With a 1 s read timeout and a 0.4 s bound, a
// holder of the gate for h: below the bound the statement waits, 200; above
// it the bound cancels, the chunk replays row by row in a new transaction and
// waits for the rest, 200 with one replay; at 1.2 s, beyond the read timeout,
// the row path (and #6514) fails with an i/o timeout while the replay only
// waits the remaining 0.8 s, 200.
func TestStudentEnrollmentBatchGateHolder(t *testing.T) {
	t.Parallel()
	const readTimeout, limit = time.Second, 400 * time.Millisecond
	db := testpkg.SetupIsolatedTestDB(t)
	pool, name := namedImportPool(t, testpkg.IsolatedTestDSN(t), readTimeout)
	for _, c := range []struct {
		hold     time.Duration
		replayed int
	}{
		{200 * time.Millisecond, 0},
		{700 * time.Millisecond, 1},
		{1200 * time.Millisecond, 1},
	} {
		t.Run(c.hold.String(), func(t *testing.T) {
			testpkg.OwnTenant(t)
			module, err := api.NewImportTestModuleWithOptions(testpkg.NewDatabase(t, pool), testpkg.TenantRuntime(t, pool), api.ImportTestOptions{BatchStatementLimit: limit})
			require.NoError(t, err)
			actor := newImporter(t, db)
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			holder, pid := holdClassWriteGate(t, ctx, db)
			done := uploadStudents(t, module, pool, actor, namedStudentBatch("Tor", studentChunkRows), "gate.csv")
			waitBlockedBy(t, db, name, pid)
			// The hold time is the subject of the test.
			time.Sleep(c.hold)
			require.NoError(t, holder.Rollback())

			upload := awaitStudents(t, done)
			require.NoError(t, upload.err)
			require.Equal(t, studentChunkRows, upload.result.CreatedCount)
			observation := (*module.Observations)[0]
			require.Equal(t, c.replayed, observation.BatchFallbacks)
			require.Zero(t, observation.BatchesRetried)
			require.Equal(t, studentChunkRows, countIn(t, db, `SELECT count(*) FROM users.student_care_profiles WHERE tenant_id = ?`))
		})
	}
}

// TestStudentEnrollmentBatchCommitOutcomeUnknown (Befundklasse 16, U): the
// COMMIT of a chunk whose profiles and care profiles were bundled fails
// after the server committed: 500, no retry, outcome unknown, the chunk with
// all three enrollment tables durable.
func TestStudentEnrollmentBatchCommitOutcomeUnknown(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	dsn := testpkg.PackageTestDSN(t)
	testpkg.OwnTenant(t)
	var bundled, failed atomic.Bool
	pool := testpkg.AdoptPrivatePool(t, testpkg.NewBunDB(sql.OpenDB(faultConnector{
		Connector: pgdriver.NewConnector(pgdriver.WithDSN(dsn)),
		decide: func(query string) error {
			if strings.HasPrefix(statementKind(query), "INSERT INTO users.student_care_profiles (") {
				bundled.Store(true)
			}
			return nil
		},
		commit: func() error {
			if bundled.Load() && failed.CompareAndSwap(false, true) {
				return driver.ErrBadConn
			}
			return nil
		},
	})))
	t.Cleanup(func() { _ = pool.Close() })
	module, err := api.NewImportTestModule(testpkg.NewDatabase(t, pool), testpkg.TenantRuntime(t, pool))
	require.NoError(t, err)
	actor := newImporter(t, db)
	upload := awaitStudents(t, uploadStudents(t, module, pool, actor, namedStudentBatch("Zusage", studentChunkRows), "commit.csv"))
	require.True(t, failed.Load(), "the COMMIT fault fired")
	require.Equal(t, "500", uploadStatus(upload.err))
	var unknown interface{ CommitOutcomeUnknown() }
	require.ErrorAs(t, upload.err, &unknown, "the outcome of the COMMIT is unknown")
	observation := (*module.Observations)[0]
	require.Zero(t, observation.BatchesRetried, "no retry after an unknown COMMIT")
	require.Zero(t, observation.BatchFallbacks)
	for _, table := range []string{"users.student_profiles", "users.student_school_memberships", "users.student_care_profiles"} {
		// The tables come from the fixed list above, never input.
		require.Equal(t, studentChunkRows, countIn(t, db, `SELECT count(*) FROM `+table+` WHERE tenant_id = ?`), "the server committed %s", table)
	}
}

// TestStudentEnrollmentBatchRealCancelReplaysInNewTransaction (table A, 57014
// echt): a slow row trigger makes the profile statement, or the care
// statement, outlast the bound for real; PostgreSQL cancels it, pgdriver
// closes the connection, the chunk replays row by row in a new transaction
// without the bound (E13′, N): 200, one replay, no retry.
func TestStudentEnrollmentBatchRealCancelReplaysInNewTransaction(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	_, err := db.ExecContext(context.Background(), `CREATE FUNCTION public.slow_marked_enrollment() RETURNS trigger LANGUAGE plpgsql AS $$
		BEGIN
			IF to_jsonb(NEW) ->> TG_ARGV[0] = TG_ARGV[1] THEN PERFORM pg_sleep(0.4); END IF;
			RETURN NULL;
		END $$;
		CREATE TRIGGER slow_marked_profile AFTER INSERT ON users.student_profiles FOR EACH ROW EXECUTE FUNCTION public.slow_marked_enrollment('address_street', 'Langsamweg 1');
		CREATE TRIGGER slow_marked_care AFTER INSERT ON users.student_care_profiles FOR EACH ROW EXECUTE FUNCTION public.slow_marked_enrollment('departure_days', '{"fri": "bus"}')`)
	require.NoError(t, err)
	for _, c := range []struct {
		name string
		mark func(*importModels.StudentImportRow)
	}{
		{"profile statement", func(row *importModels.StudentImportRow) { row.AddressStreet = "Langsamweg 1" }},
		{"care statement", func(row *importModels.StudentImportRow) { row.DepartureDays = map[string]string{"fri": "bus"} }},
	} {
		t.Run(c.name, func(t *testing.T) {
			testpkg.OwnTenant(t)
			module, err := api.NewImportTestModuleWithOptions(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db), api.ImportTestOptions{
				BatchStatementLimit: 250 * time.Millisecond,
			})
			require.NoError(t, err)
			actor := newImporter(t, db)
			rows := namedStudentBatch("Abbruch", studentChunkRows)
			c.mark(&rows[3])
			upload := awaitStudents(t, uploadStudents(t, module, db, actor, rows, "cancel.csv"))
			require.NoError(t, upload.err)
			require.Equal(t, studentChunkRows, upload.result.CreatedCount)
			observation := (*module.Observations)[0]
			require.Equal(t, 1, observation.BatchFallbacks, "the cancelled batch replayed")
			require.Zero(t, observation.BatchesRetried)
			require.Equal(t, studentChunkRows, countIn(t, db, `SELECT count(*) FROM users.student_care_profiles WHERE tenant_id = ?`))
		})
	}
}

// TestStudentEnrollmentBatchWritesEachMembershipInPlace (Befundklasse 5): the
// membership of row k, with its status and quota count, is written after row
// k−1's pickup schedule, as on the row path; only the profiles go before the
// rows and the care profiles after them.
func TestStudentEnrollmentBatchWritesEachMembershipInPlace(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module, err := api.NewImportTestModule(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db))
	require.NoError(t, err)
	actor := newImporter(t, db)
	counter := testpkg.CaptureQueriesForContext(t, db)
	ctx := counter.Context(testpkg.Ctx(t))
	result, err := module.Import.ImportBatches(ctx, importModels.ImportRequest[importModels.StudentImportRow]{
		Rows: namedStudentBatch("Reihe", studentChunkRows), Mode: importModels.ImportModeCreate, UserID: actor.staffID, SkipInvalidRows: true,
	}, importModels.BatchAudit{EntityType: "student", Filename: "order.csv", AccountID: actor.accountID})
	require.NoError(t, err)
	require.Equal(t, studentChunkRows, result.CreatedCount)
	require.Equal(t, 1, commandCount((*module.Observations)[0], enrollmentProfilesCommand), "the batch path ran")
	var sequence strings.Builder
	for _, query := range counter.Queries() {
		switch kind := statementKind(query); {
		case strings.Contains(kind, "INSERT INTO users.student_school_memberships"):
			sequence.WriteString("M")
		case strings.HasPrefix(kind, "INSERT INTO schedule.student_pickup_schedules"):
			sequence.WriteString("S")
		}
	}
	require.Equal(t, strings.Repeat("MS", studentChunkRows), sequence.String(), "each membership right before its own row's schedule, after the previous row's")
}
