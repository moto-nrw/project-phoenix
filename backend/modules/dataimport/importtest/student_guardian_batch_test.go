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
	"github.com/uptrace/bun/dialect/pgdialect"
	"github.com/uptrace/bun/driver/pgdriver"

	"github.com/moto-nrw/project-phoenix/api"
	importModels "github.com/moto-nrw/project-phoenix/modules/dataimport"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// Step 5c of #5552 (PR #6514): in a batch segment the guardians of a row
// whose guardians all lack an e-mail, with at most one primary and no later
// primary phone, are written after the rows in one statement per table:
// profiles, phones, relationships with the retention reset, pickup
// permissions, portal accesses. They touch only rows of the upload's own
// transaction and the school row the persons statement already holds.

const guardianBatchCommand = "create_new_student_guardians"

// variedGuardianFile has seven rows without guardian e-mail and one without
// guardians, with every value the single commands derive: trimmed names, a
// language in capitals, an emergency priority of 0, a role preset and the
// default role, pickup notes, and phones with a duplicate, an empty number,
// a label with spaces, an empty and an unknown type.
func variedGuardianFile() []importModels.StudentImportRow {
	rows := make([]importModels.StudentImportRow, 0, 8)
	for i := range 7 {
		name := fmt.Sprintf("Vielfalt%d", i)
		second := importModels.GuardianImportData{
			FirstName: "Ben", LastName: name + "b", RelationshipType: "Vater", IsEmergencyContact: true, EmergencyPriority: 2,
			PhoneNumbers: []importModels.PhoneImportData{{PhoneNumber: fmt.Sprintf("0170 444%02d", i), PhoneType: "work"}},
		}
		if i%2 == 1 {
			second.GuardianRole = "pickup_only"
			second.CanPickup = true
		}
		rows = append(rows, importModels.StudentImportRow{
			FirstName: name, LastName: "Batch", SchoolClass: "4A", Birthday: "2017-04-05", DataRetentionDays: 30, PrivacyAccepted: true,
			Guardians: []importModels.GuardianImportData{{
				FirstName: " Anna ", LastName: name, RelationshipType: "Mutter", IsPrimary: true, IsPrimarySet: true, CanPickup: true,
				PickupNotes: " nur dienstags ", AddressStreet: "Weg 1", AddressCity: " Köln ", AddressPostalCode: "50667",
				Notes: "Notiz", LanguagePreference: "EN",
				PhoneNumbers: []importModels.PhoneImportData{
					{PhoneNumber: fmt.Sprintf("0151 111%02d", i), PhoneType: "mobile"},
					{PhoneNumber: fmt.Sprintf("0151 111%02d", i), PhoneType: "home"},
					{PhoneNumber: "", PhoneType: "home"},
					{PhoneNumber: fmt.Sprintf("0221 222%02d", i), PhoneType: "", Label: " Büro "},
					{PhoneNumber: fmt.Sprintf("0331 333%02d", i), PhoneType: "fax"},
				},
			}, second},
			PickupSchedules: []importModels.PickupScheduleImportData{{Weekday: 2, PickupTime: "15:30"}},
		})
	}
	rows = append(rows, importModels.StudentImportRow{FirstName: "Ohne", LastName: "Eltern", SchoolClass: "4A", DataRetentionDays: 30})
	return rows
}

func runGuardianFile(t *testing.T, db *bun.DB, options api.ImportTestOptions, rows []importModels.StudentImportRow) (studentOutcome, importModels.ImportObservation) {
	t.Helper()
	testpkg.OwnTenant(t)
	module, err := api.NewImportTestModuleWithOptions(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db), options)
	require.NoError(t, err)
	actor := newImporter(t, db)
	upload := awaitStudents(t, uploadStudents(t, module, db, actor, rows, "guardians.csv"))
	require.NoError(t, upload.err)
	require.Len(t, *module.Observations, 1)
	return studentOutcomeOf(t, db, upload.result), (*module.Observations)[0]
}

// TestStudentGuardianBatchMatchesTheRowPath (table C, NULL against default):
// the bundled guardian statements leave exactly the rows the single commands
// leave in all five tables, every column but ids and timestamps compared.
func TestStudentGuardianBatchMatchesTheRowPath(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	var batch, rowPath studentOutcome
	t.Run("batch path", func(t *testing.T) {
		var observation importModels.ImportObservation
		batch, observation = runGuardianFile(t, db, api.ImportTestOptions{}, variedGuardianFile())
		require.Zero(t, observation.BatchFallbacks)
		require.Equal(t, 1, commandCount(observation, guardianBatchCommand), "the guardians went into the bundled statements")
		require.Zero(t, commandCount(observation, "create_guardian"))
		require.Zero(t, commandCount(observation, "add_guardian_phone"))
		require.Zero(t, commandCount(observation, "link_guardian_to_student"))
		var unmarked, other int
		require.NoError(t, db.NewRaw(`SELECT count(*) FILTER (WHERE childless_since IS NULL), (SELECT count(*) FROM users.guardian_phone_numbers WHERE tenant_id = ? AND phone_type = 'other')
			FROM users.guardian_profiles WHERE tenant_id = ?`, testpkg.Tenant(t), testpkg.Tenant(t)).Scan(context.Background(), &unmarked, &other))
		require.Equal(t, 14, unmarked, "every linked guardian's retention mark is cleared")
		// The outcome comparison drops every *_id column, account_id
		// included; a new guardian has no account, on the profile and on the
		// portal access (#6514 review).
		var withoutAccount, accessWithoutAccount int
		require.NoError(t, db.NewRaw(`SELECT count(*) FILTER (WHERE account_id IS NULL AND NOT has_account),
			(SELECT count(*) FILTER (WHERE account_id IS NULL) FROM auth.guardian_student_access WHERE tenant_id = ?)
			FROM users.guardian_profiles WHERE tenant_id = ?`, testpkg.Tenant(t), testpkg.Tenant(t)).Scan(context.Background(), &withoutAccount, &accessWithoutAccount))
		require.Equal(t, 14, withoutAccount, "no new guardian profile names an account")
		require.Equal(t, 14, accessWithoutAccount, "no portal access of a new guardian names an account")
		require.Equal(t, 14, other, "an empty and an unknown phone type are stored as other")
	})
	t.Run("row path", func(t *testing.T) {
		var observation importModels.ImportObservation
		rowPath, observation = runGuardianFile(t, db, api.ImportTestOptions{RowPathOnly: true}, variedGuardianFile())
		require.Zero(t, commandCount(observation, guardianBatchCommand))
	})
	require.Equal(t, rowPath, batch)
	require.Len(t, batch.Rows["users.guardian_phone_numbers"], 7*4, "duplicates and empty numbers are skipped")
}

// TestStudentGuardianBatchKeepsOtherRowsInPlace: a row with an e-mail
// guardian, one with two primary guardians and one whose later phone asks to
// be primary write their guardians in place with the single commands; the
// other rows' guardians are bundled. The result is the row path's.
func TestStudentGuardianBatchKeepsOtherRowsInPlace(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	file := func() []importModels.StudentImportRow {
		rows := variedGuardianFile()
		rows[1].Guardians[1].Email = "vielfalt1b@example.test"
		rows[3].Guardians[1].IsPrimary = true
		rows[5].Guardians[0].PhoneNumbers[3].IsPrimary = true
		return rows
	}
	var batch, rowPath studentOutcome
	t.Run("batch path", func(t *testing.T) {
		var observation importModels.ImportObservation
		batch, observation = runGuardianFile(t, db, api.ImportTestOptions{}, file())
		require.Zero(t, observation.BatchFallbacks)
		require.Equal(t, 1, commandCount(observation, guardianBatchCommand))
		require.Equal(t, 6, commandCount(observation, "create_guardian"), "rows 1, 3 and 5 keep their two guardians in place")
		require.Equal(t, 6, commandCount(observation, "link_guardian_to_student"))
	})
	t.Run("row path", func(t *testing.T) {
		rowPath, _ = runGuardianFile(t, db, api.ImportTestOptions{RowPathOnly: true}, file())
	})
	require.Equal(t, rowPath, batch)
}

// TestStudentGuardianBatchAddsNoWait (Befundklasse 1): with a 1 s read
// timeout, a holder of the school row for 0.7 s delays the upload once, at
// the first statement that references the school, as on the row path; the
// bundled guardian statements reference only the school row the
// transaction already holds and rows of their own transaction, so they wait
// for nothing. Bounded and unbounded end with 200, nothing is cancelled,
// nothing replays.
func TestStudentGuardianBatchAddsNoWait(t *testing.T) {
	t.Parallel()
	const readTimeout, hold, limit = time.Second, 700 * time.Millisecond, 800 * time.Millisecond
	db := testpkg.SetupIsolatedTestDB(t)
	pool, name := namedImportPool(t, testpkg.IsolatedTestDSN(t), readTimeout)
	for _, bounded := range []bool{true, false} {
		t.Run(fmt.Sprintf("bounded=%t", bounded), func(t *testing.T) {
			testpkg.OwnTenant(t)
			module, err := api.NewImportTestModuleWithOptions(testpkg.NewDatabase(t, pool), testpkg.TenantRuntime(t, pool), api.ImportTestOptions{
				BatchStatementLimit: limit, UnboundedBatches: !bounded,
			})
			require.NoError(t, err)
			actor := newImporter(t, db)
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			holder, err := db.BeginTx(ctx, nil)
			require.NoError(t, err)
			t.Cleanup(func() { _ = holder.Rollback() })
			pid := sessionPID(t, ctx, holder)
			var school int64
			require.NoError(t, holder.NewRaw(`SELECT id FROM platform.schools WHERE id = ? FOR UPDATE`, testpkg.Tenant(t)).Scan(ctx, &school))
			done := uploadStudents(t, module, pool, actor, guardianBatchFile("Warte"), "wait.csv")
			waitBlockedBy(t, db, name, pid)
			// The hold time is the subject of the test.
			time.Sleep(hold)
			require.NoError(t, holder.Rollback())

			upload := awaitStudents(t, done)
			require.NoError(t, upload.err)
			require.Equal(t, studentChunkRows, upload.result.CreatedCount)
			observation := (*module.Observations)[0]
			require.Zero(t, observation.BatchFallbacks, "the bound cancelled nothing")
			require.Zero(t, observation.BatchesRetried)
			require.Equal(t, 1, commandCount(observation, guardianBatchCommand), "the batch path ran")
		})
	}
}

// TestStudentGuardianBatchWaitsAddUpNowhere (Befundklasse 1, negative
// control): three holders each lock an existing guardian of the school the
// file never names and release one after another, 0.6 s apart; the uploads
// read with a 1 s timeout and no bound. Neither the batch nor the row path
// waits for those guardians, so both end with 200. A bundled statement that
// locked them, as a batch over existing guardians would, waits for all three
// in one read and fails with an i/o timeout: the mutation in the PR turns
// this test red. The release chain starts on an event, never on a timer:
// either the upload's own session waits for a holder (pg_blocking_pids of
// its named pool), then the holders release 0.6 s apart, so the waiting read
// lasts at least 1.8 s; or the upload ends while every holder still holds,
// which proves it waited for none of them.
func TestStudentGuardianBatchWaitsAddUpNowhere(t *testing.T) {
	t.Parallel()
	const readTimeout, gap = time.Second, 600 * time.Millisecond
	db := testpkg.SetupTestDB(t)
	for _, run := range []struct {
		name    string
		options api.ImportTestOptions
	}{
		{"batch path", api.ImportTestOptions{UnboundedBatches: true}},
		{"row path", api.ImportTestOptions{RowPathOnly: true}},
	} {
		t.Run(run.name, func(t *testing.T) {
			testpkg.OwnTenant(t)
			pool, name := namedImportPool(t, testpkg.PackageTestDSN(t), readTimeout)
			module, err := api.NewImportTestModuleWithOptions(testpkg.NewDatabase(t, pool), testpkg.TenantRuntime(t, pool), run.options)
			require.NoError(t, err)
			actor := newImporter(t, db)
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			holders, pids := make([]bun.Tx, 3), make([]int, 3)
			for i := range holders {
				held := testpkg.CreateTestGuardianProfileWithExactEmail(t, db, testpkg.Tenant(t), testpkg.GuardianProfileWithEmail{
					FirstName: "Eltern", LastName: fmt.Sprintf("Gehalten%d", i), Email: fmt.Sprintf("gehalten%d.%d@example.test", i, testpkg.Tenant(t))})
				tx, err := db.BeginTx(ctx, nil)
				require.NoError(t, err)
				t.Cleanup(func() { _ = tx.Rollback() })
				var id int64
				require.NoError(t, tx.NewRaw(`SELECT id FROM users.guardian_profiles WHERE id = ? FOR UPDATE`, held.ID).Scan(ctx, &id))
				holders[i], pids[i] = tx, sessionPID(t, ctx, tx)
			}
			done := uploadStudents(t, module, pool, actor, guardianBatchFile("Kette"), "chain.csv")
			var upload studentUpload
			finished := false
			deadline := time.Now().Add(20 * time.Second)
			for !finished && !uploadWaitsForAny(t, db, name, pids) {
				require.True(t, time.Now().Before(deadline), "the upload neither ended nor waited for a holder")
				select {
				case upload = <-done:
					finished = true
				case <-time.After(5 * time.Millisecond):
				}
			}
			if finished {
				// Ended while every holder still held: it waited for none.
				for _, holder := range holders {
					require.NoError(t, holder.Rollback())
				}
			} else {
				// A statement waits for the holders: release them one after
				// another; the staggered hold times are the subject here.
				for _, holder := range holders {
					time.Sleep(gap)
					require.NoError(t, holder.Rollback())
				}
				upload = awaitStudents(t, done)
			}
			require.NoError(t, upload.err)
			require.Equal(t, studentChunkRows, upload.result.CreatedCount)
			observation := (*module.Observations)[0]
			require.Zero(t, observation.BatchFallbacks)
			require.Zero(t, observation.BatchesRetried)
		})
	}
}

// uploadWaitsForAny reports whether a session of the upload's own pool
// (application name upload) waits for a lock one of the holder sessions
// holds.
func uploadWaitsForAny(t *testing.T, db *bun.DB, upload string, holders []int) bool {
	t.Helper()
	var blocked int
	err := db.NewRaw(`SELECT count(*) FROM pg_stat_activity WHERE application_name = ? AND pg_blocking_pids(pid) && ?::int[]`, upload, pgdialect.Array(holders)).
		Scan(context.Background(), &blocked)
	require.NoError(t, err)
	return blocked > 0
}

// guardianBatchFile is namedStudentBatch without guardian e-mails.
func guardianBatchFile(prefix string) []importModels.StudentImportRow {
	rows := namedStudentBatch(prefix, studentChunkRows)
	for i := range rows {
		rows[i].Guardians[0].Email = ""
	}
	return rows
}

// TestStudentGuardianBatchParallelUploadsFormNoDeadlock (Befundklasse 6):
// two uploads of one school, each through a module and pool of its own (one
// module would serialize them on its import lock), whose guardians are
// bundled, overlap for real: a holder of the importing staff member's
// membership stops upload A at its first schedule while A holds the child
// quota; upload B then waits for A's quota lock while it holds what its
// person statement took. Released, both commit: no deadlock, no retry, no
// replay. A guardian batch that took a lock B holds would deadlock here.
func TestStudentGuardianBatchParallelUploadsFormNoDeadlock(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	testpkg.OwnTenant(t)
	actor := newImporter(t, db)
	type side struct {
		module api.ImportTestModule
		pool   *bun.DB
		name   string
	}
	sides := make([]side, 2)
	for i := range sides {
		pool, name := namedImportPool(t, testpkg.PackageTestDSN(t), 0)
		module, err := api.NewImportTestModule(testpkg.NewDatabase(t, pool), testpkg.TenantRuntime(t, pool))
		require.NoError(t, err)
		sides[i] = side{module, pool, name}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	holder, holderPID := holdStaffMembership(t, ctx, db, actor.staffID)
	first := uploadStudents(t, sides[0].module, sides[0].pool, actor, guardianBatchFile("Erster"), "first.csv")
	waitBlockedBy(t, db, sides[0].name, holderPID)
	second := uploadStudents(t, sides[1].module, sides[1].pool, actor, guardianBatchFile("Zweiter"), "second.csv")
	waitUploadBlockedByUpload(t, db, sides[1].name, sides[0].name)
	require.NoError(t, holder.Rollback())
	for _, done := range []<-chan studentUpload{first, second} {
		upload := awaitStudents(t, done)
		require.NoError(t, upload.err)
		require.Equal(t, studentChunkRows, upload.result.CreatedCount)
	}
	for _, s := range sides {
		require.Len(t, *s.module.Observations, 1)
		observation := (*s.module.Observations)[0]
		require.Zero(t, observation.Deadlocks)
		require.Zero(t, observation.BatchesRetried)
		require.Zero(t, observation.BatchFallbacks)
		require.Equal(t, 1, commandCount(observation, guardianBatchCommand))
	}
	require.Equal(t, 2*studentChunkRows, countIn(t, db, `SELECT count(*) FROM auth.guardian_student_access WHERE tenant_id = ?`))
}

// waitUploadBlockedByUpload waits, within a hard deadline, until a session
// of upload waiter waits for a lock a session of upload holder holds; both
// are named pools of the test (Befundklasse 13).
func waitUploadBlockedByUpload(t *testing.T, db *bun.DB, waiter, holder string) {
	t.Helper()
	require.Eventually(t, func() bool {
		var blocked int
		err := db.NewRaw(`SELECT count(*) FROM pg_stat_activity w WHERE w.application_name = ?
			AND EXISTS (SELECT 1 FROM pg_stat_activity h WHERE h.application_name = ? AND h.pid = ANY(pg_blocking_pids(w.pid)))`, waiter, holder).
			Scan(context.Background(), &blocked)
		return err == nil && blocked > 0
	}, 10*time.Second, 5*time.Millisecond, "the second upload waits for the first")
}

// TestStudentGuardianBatchCommitOutcomeUnknown (Befundklasse 16, U): the
// COMMIT of a chunk that bundled its guardians fails after the server
// committed. The upload ends with 500 and no retry, the error says the
// outcome is unknown, and the chunk with its guardians is durable.
func TestStudentGuardianBatchCommitOutcomeUnknown(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	dsn := testpkg.PackageTestDSN(t)
	testpkg.OwnTenant(t)
	var bundled, failed atomic.Bool
	pool := testpkg.AdoptPrivatePool(t, testpkg.NewBunDB(sql.OpenDB(faultConnector{
		Connector: pgdriver.NewConnector(pgdriver.WithDSN(dsn)),
		decide: func(query string) error {
			if strings.HasPrefix(strings.TrimSpace(query), "MERGE INTO users.guardian_profiles") {
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
	upload := awaitStudents(t, uploadStudents(t, module, pool, actor, guardianBatchFile("Commit"), "commit.csv"))
	require.True(t, failed.Load(), "the COMMIT fault fired")
	require.Equal(t, "500", uploadStatus(upload.err))
	var unknown interface{ CommitOutcomeUnknown() }
	require.ErrorAs(t, upload.err, &unknown, "the outcome of the COMMIT is unknown")
	observation := (*module.Observations)[0]
	require.Zero(t, observation.BatchesRetried, "no retry after an unknown COMMIT")
	require.Zero(t, observation.BatchFallbacks)
	require.Equal(t, studentChunkRows, countIn(t, db, studentCount), "the server committed the chunk")
	require.Equal(t, studentChunkRows, countIn(t, db, `SELECT count(*) FROM auth.guardian_student_access WHERE tenant_id = ?`), "with its guardians")
}

// TestStudentGuardianBatchRealCancelReplaysInNewTransaction (table A,
// 57014 echt): a trigger makes the bundled guardian insert outlast the bound
// for real. PostgreSQL cancels it, pgdriver closes the connection, and the
// chunk replays row by row in a new transaction without the bound (E13′, N);
// the slow single inserts there succeed. 200, one replay, no retry.
func TestStudentGuardianBatchRealCancelReplaysInNewTransaction(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	// One slow row trigger on the profile insert (statement 1) and one on the
	// relationship insert of the retention-and-link statement (statement 3).
	_, err := db.ExecContext(context.Background(), `CREATE FUNCTION public.slow_marked_guardian() RETURNS trigger LANGUAGE plpgsql AS $$
		BEGIN
			IF to_jsonb(NEW) ->> TG_ARGV[0] = TG_ARGV[1] THEN PERFORM pg_sleep(0.4); END IF;
			RETURN NULL;
		END $$;
		CREATE TRIGGER slow_marked_guardian AFTER INSERT ON users.guardian_profiles FOR EACH ROW EXECUTE FUNCTION public.slow_marked_guardian('last_name', 'Langsam');
		CREATE TRIGGER slow_marked_link AFTER INSERT ON users.student_guardian_relationships FOR EACH ROW EXECUTE FUNCTION public.slow_marked_guardian('emergency_priority', '9')`)
	require.NoError(t, err)
	for _, c := range []struct {
		name string
		mark func(*importModels.GuardianImportData)
	}{
		{"profile statement", func(g *importModels.GuardianImportData) { g.LastName = "Langsam" }},
		{"retention and link statement", func(g *importModels.GuardianImportData) { g.EmergencyPriority = 9 }},
	} {
		t.Run(c.name, func(t *testing.T) {
			testpkg.OwnTenant(t)
			module, err := api.NewImportTestModuleWithOptions(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db), api.ImportTestOptions{
				BatchStatementLimit: 250 * time.Millisecond,
			})
			require.NoError(t, err)
			actor := newImporter(t, db)
			rows := guardianBatchFile("Abbruch")
			c.mark(&rows[3].Guardians[0])
			upload := awaitStudents(t, uploadStudents(t, module, db, actor, rows, "cancel.csv"))
			require.NoError(t, upload.err)
			require.Equal(t, studentChunkRows, upload.result.CreatedCount)
			observation := (*module.Observations)[0]
			require.Equal(t, 1, observation.BatchFallbacks, "the cancelled batch replayed")
			require.Zero(t, observation.BatchesRetried)
			require.Equal(t, studentChunkRows, countIn(t, db, `SELECT count(*) FROM auth.guardian_student_access WHERE tenant_id = ?`))
		})
	}
}
