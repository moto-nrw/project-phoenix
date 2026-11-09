package integration

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/api"
	importModels "github.com/moto-nrw/project-phoenix/modules/dataimport"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// Decision E13 (09.11.2026, ADR 0067 addendum of 2026-11-08): every bundled
// write of the chunk executor runs as a bounded attempt. PostgreSQL sends
// nothing while a statement waits for a lock, and pgdriver gives up a read
// after its read timeout (10 s, not overridden in production). One INSERT
// over several rows waits for each conflicting transaction in turn in one
// read; one INSERT per row waits for one of them per read. Scaled down as in
// #6239 and #6261: a 1 s read timeout, two holders of 0.7 s each, a bound of
// 0.8 s.

// shortReadTimeoutPool opens a second pool on the test's isolated clone whose
// driver gives up a read after readTimeout.
func shortReadTimeoutPool(t *testing.T, readTimeout time.Duration) *bun.DB {
	t.Helper()
	dsn, err := url.Parse(testpkg.IsolatedTestDSN(t))
	require.NoError(t, err)
	query := dsn.Query()
	query.Set("read_timeout", readTimeout.String())
	dsn.RawQuery = query.Encode()
	pool := testpkg.AdoptPrivatePool(t, testpkg.NewBunDB(testpkg.OpenPostgresSQL(dsn.String())))
	t.Cleanup(func() { _ = pool.Close() })
	return pool
}

func waitForLockWaiters(t *testing.T, db *bun.DB) {
	t.Helper()
	require.Eventually(t, func() bool {
		var waiting int
		err := db.NewRaw(`SELECT count(*) FROM pg_stat_activity
			WHERE datname = current_database() AND wait_event_type = 'Lock'`).Scan(context.Background(), &waiting)
		return err == nil && waiting > 0
	}, 10*time.Second, 5*time.Millisecond)
}

// holdClassListKey inserts an uncommitted entry with the row's key; an import
// inserting the same key waits for it until the holder rolls back.
func holdClassListKey(t *testing.T, db *bun.DB, row importModels.ClassListEntryImportRow) bun.Tx {
	t.Helper()
	tx, err := db.BeginTx(context.Background(), nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = tx.Rollback() })
	_, err = tx.ExecContext(context.Background(), `INSERT INTO users.class_list_entries (tenant_id, first_name, last_name, school_class) VALUES (?, ?, ?, ?)`,
		testpkg.Tenant(t), row.FirstName, row.LastName, row.SchoolClass)
	require.NoError(t, err)
	return tx
}

type classListUpload struct {
	result *importModels.ImportResult[importModels.ClassListEntryImportRow]
	err    error
}

// uploadClassList runs the upload in the unit of work of pool, the pool the
// module was composed over.
func uploadClassList(t *testing.T, module api.ImportTestModule, pool *bun.DB, actor importer, rows []importModels.ClassListEntryImportRow, filename string) <-chan classListUpload {
	done := make(chan classListUpload, 1)
	ctx := tenant.WithUnitOfWork(testpkg.Ctx(t), testpkg.TenantRuntime(t, pool))
	go func() {
		result, err := module.ClassListImport.ImportBatches(ctx, importModels.ImportRequest[importModels.ClassListEntryImportRow]{
			Rows: rows, Mode: importModels.ImportModeCreate, UserID: actor.staffID, SkipInvalidRows: true,
		}, importModels.BatchAudit{EntityType: "class_list_entries", Filename: filename, AccountID: actor.accountID})
		done <- classListUpload{result, err}
	}()
	return done
}

func awaitUpload(t *testing.T, done <-chan classListUpload) classListUpload {
	t.Helper()
	select {
	case upload := <-done:
		return upload
	case <-time.After(60 * time.Second):
		t.Fatal("the import did not answer")
		return classListUpload{}
	}
}

// TestClassListBatchWaitsAreBoundedAndReplayed is test (a) of E13. Unbounded,
// the batch INSERT waits for both holders in one read and the driver gives up
// (i/o timeout, a 500). Bounded at 0.8 s, the attempt is cancelled, the chunk
// replays row by row, each row waits for one holder within the read timeout,
// and the import ends like the row path.
func TestClassListBatchWaitsAreBoundedAndReplayed(t *testing.T) {
	t.Parallel()
	const readTimeout, hold, limit = time.Second, 700 * time.Millisecond, 800 * time.Millisecond
	db := testpkg.SetupIsolatedTestDB(t)
	pool := shortReadTimeoutPool(t, readTimeout)
	// Two connections: a connection the bound's 57014 discarded must be
	// replaced, or the second upload below would starve.
	pool.SetMaxOpenConns(2)
	for _, bounded := range []bool{true, false} {
		t.Run(fmt.Sprintf("bounded=%t", bounded), func(t *testing.T) {
			testpkg.OwnTenant(t)
			module, err := api.NewImportTestModuleWithOptions(testpkg.NewDatabase(t, pool), testpkg.TenantRuntime(t, pool), api.ImportTestOptions{
				BatchStatementLimit: limit, UnboundedBatches: !bounded,
			})
			require.NoError(t, err)
			actor := newImporter(t, db)
			rows := []importModels.ClassListEntryImportRow{
				{FirstName: "Halter", LastName: "Eins", SchoolClass: "1a"},
				{FirstName: "Halter", LastName: "Zwei", SchoolClass: "1a"},
			}
			first, second := holdClassListKey(t, db, rows[0]), holdClassListKey(t, db, rows[1])
			done := uploadClassList(t, module, pool, actor, rows, "bounded.csv")
			waitForLockWaiters(t, db)
			started := time.Now()
			// The hold times are the subject of the test.
			time.Sleep(hold)
			require.NoError(t, first.Rollback())
			waitForLockWaiters(t, db)
			time.Sleep(hold)
			require.NoError(t, second.Rollback())
			upload := awaitUpload(t, done)
			require.Greater(t, time.Since(started), readTimeout, "the waits together exceed the read timeout")

			entries := countIn(t, db, `SELECT count(*) FROM users.class_list_entries WHERE tenant_id = ?`)
			if !bounded {
				require.Error(t, upload.err)
				var netErr net.Error
				require.True(t, errors.As(upload.err, &netErr) || strings.Contains(upload.err.Error(), "timeout"), "%v", upload.err)
				require.Zero(t, entries, "the cut connection committed nothing")
				return
			}
			require.NoError(t, upload.err)
			require.Equal(t, 2, upload.result.CreatedCount)
			require.Zero(t, upload.result.ErrorCount)
			require.Equal(t, 2, entries)
			require.Len(t, *module.Observations, 1)
			require.Equal(t, 1, (*module.Observations)[0].BatchFallbacks, "the bound cancelled the attempt and the chunk replayed")
			require.Zero(t, (*module.Observations)[0].BatchesRetried, "the replay in a new transaction consumes no retry")

			// The pool replaced the discarded connection: a second upload runs
			// on it, and the pool never grew past its two connections.
			more := awaitUpload(t, uploadClassList(t, module, pool, actor, []importModels.ClassListEntryImportRow{
				{FirstName: "Danach", LastName: "Eins", SchoolClass: "1a"}, {FirstName: "Danach", LastName: "Zwei", SchoolClass: "1a"},
			}, "after.csv"))
			require.NoError(t, more.err)
			require.Equal(t, 2, more.result.CreatedCount)
			require.LessOrEqual(t, pool.Stats().OpenConnections, 2)
		})
	}
}

// classListOutcome is an upload's result and persistence without IDs and
// timestamps, for byte comparison across tenants.
type classListOutcome struct {
	Created, Updated, Errors, Warnings int
	RowErrors                          []string
	Entries, Changes                   []string
}

func classListOutcomeOf(t *testing.T, db *bun.DB, result *importModels.ImportResult[importModels.ClassListEntryImportRow]) classListOutcome {
	t.Helper()
	outcome := classListOutcome{Created: result.CreatedCount, Updated: result.UpdatedCount, Errors: result.ErrorCount, Warnings: result.WarningCount}
	for _, row := range result.Errors {
		for _, e := range row.Errors {
			outcome.RowErrors = append(outcome.RowErrors, fmt.Sprintf("%d %s %s %s %s", row.RowNumber, e.Severity, e.Field, e.Code, e.Message))
		}
	}
	require.NoError(t, db.NewRaw(`SELECT first_name || '|' || last_name || '|' || school_class || '|' || (created_by IS NOT NULL)::text
		FROM users.class_list_entries WHERE tenant_id = ? ORDER BY 1`, testpkg.Tenant(t)).Scan(context.Background(), &outcome.Entries))
	require.NoError(t, db.NewRaw(`SELECT action || '|' || old_value || '|' || new_value || '|' || (changed_by IS NOT NULL)::text
		FROM audit.class_list_entry_changes WHERE tenant_id = ? ORDER BY 1`, testpkg.Tenant(t)).Scan(context.Background(), &outcome.Changes))
	sort.Strings(outcome.RowErrors)
	return outcome
}

// seedClassListOverlap gives a tenant an entry for "Schon Eingetragen" and a
// student "Schon Kind", so the file has two already_exists rows.
func seedClassListOverlap(t *testing.T, db *bun.DB) {
	t.Helper()
	_, err := db.ExecContext(context.Background(), `INSERT INTO users.class_list_entries (tenant_id, first_name, last_name, school_class) VALUES (?, 'Schon', 'Eingetragen', '1a')`, testpkg.Tenant(t))
	require.NoError(t, err)
	testpkg.CreateTestStudent(t, db, "Schon", "Kind", "1a")
}

func overlapFile() []importModels.ClassListEntryImportRow {
	return []importModels.ClassListEntryImportRow{
		{FirstName: "Neu", LastName: "Eins", SchoolClass: "1a"},
		{FirstName: "Schon", LastName: "Eingetragen", SchoolClass: "1a"},
		{FirstName: "Neu", LastName: "Gehalten", SchoolClass: "1a"},
		{FirstName: "Schon", LastName: "Kind", SchoolClass: "1a"},
		{FirstName: "Neu", LastName: "Drei", SchoolClass: "1b"},
	}
}

// TestClassListBoundedReplayMatchesTheRowPath is test (b) of E13: the bound
// cancels the attempt, the chunk replays, and result, counters and
// persistence are byte-equal to a run on the row path.
func TestClassListBoundedReplayMatchesTheRowPath(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	run := func(t *testing.T, options api.ImportTestOptions, holder bool) (classListOutcome, importModels.ImportObservation) {
		testpkg.OwnTenant(t)
		module, err := api.NewImportTestModuleWithOptions(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db), options)
		require.NoError(t, err)
		actor := newImporter(t, db)
		seedClassListOverlap(t, db)
		rows := overlapFile()
		var held bun.Tx
		if holder {
			held = holdClassListKey(t, db, rows[2])
		}
		done := uploadClassList(t, module, db, actor, rows, "replay.csv")
		if holder {
			waitForLockWaiters(t, db)
			// The hold outlasts the bound, the subject of the test.
			time.Sleep(600 * time.Millisecond)
			require.NoError(t, held.Rollback())
		}
		upload := awaitUpload(t, done)
		require.NoError(t, upload.err)
		require.Len(t, *module.Observations, 1)
		return classListOutcomeOf(t, db, upload.result), (*module.Observations)[0]
	}
	var bounded, rowPath classListOutcome
	t.Run("bounded attempt replays", func(t *testing.T) {
		var observation importModels.ImportObservation
		bounded, observation = run(t, api.ImportTestOptions{BatchStatementLimit: 200 * time.Millisecond}, true)
		require.Equal(t, 1, observation.BatchFallbacks)
	})
	t.Run("row path", func(t *testing.T) {
		var observation importModels.ImportObservation
		rowPath, observation = run(t, api.ImportTestOptions{RowPathOnly: true}, false)
		require.Zero(t, observation.BatchFallbacks)
	})
	require.Equal(t, rowPath, bounded)
	require.Equal(t, 3, bounded.Created)
	require.Equal(t, 2, bounded.Errors)
}

// TestStatementBoundEndsWithTheBatch is test (c) of E13: the bound reaches
// no statement after the batch, neither after success nor after the
// savepoint rollback of a cancelled attempt, and the import's checkpoint
// insert after the batch is not cancelled either.
func TestStatementBoundEndsWithTheBatch(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	const limit, slow = 100 * time.Millisecond, "SELECT pg_sleep(0.3)"
	t.Run("platform", func(t *testing.T) {
		testpkg.OwnTenant(t)
		var before string
		setting := func(ctx context.Context, tx bun.Tx) string {
			var value string
			require.NoError(t, tx.NewRaw(`SELECT current_setting('statement_timeout')`).Scan(ctx, &value))
			return value
		}
		require.NoError(t, testpkg.WithTenantTx(t, testpkg.Ctx(t), db, testpkg.Tenant(t), func(ctx context.Context, tx bun.Tx) error {
			before = setting(ctx, tx)
			require.NoError(t, tenant.WithSavepoint(ctx, func(ctx context.Context) error {
				if err := tenant.WithStatementTimeout(ctx, limit, func(ctx context.Context) error {
					require.Equal(t, "100ms", setting(ctx, tx), "the bound applies inside the batch")
					return nil
				}); err != nil {
					return err
				}
				_, err := tx.ExecContext(ctx, slow)
				return err
			}), "a statement after a successful batch outlasts the bound")
			require.Equal(t, before, setting(ctx, tx), "the bound is restored after the batch")
			_, err := tx.ExecContext(ctx, slow)
			return err
		}), "audit, checkpoint and commit after a successful batch run without the bound")

		// A statement the bound cancels ends the transaction: pgdriver closes
		// the connection on 57014 (E13′).
		err := testpkg.WithTenantTx(t, testpkg.Ctx(t), db, testpkg.Tenant(t), func(ctx context.Context, tx bun.Tx) error {
			err := tenant.WithSavepoint(ctx, func(ctx context.Context) error {
				return tenant.WithStatementTimeout(ctx, limit, func(ctx context.Context) error {
					_, err := tx.ExecContext(ctx, slow)
					return err
				})
			})
			var state interface{ Field(byte) string }
			require.True(t, errors.As(err, &state), "%v", err)
			require.Equal(t, "57014", state.Field('C'), "the bound cancels the slow batch statement")
			return err
		})
		require.Error(t, err)
		require.NoError(t, testpkg.WithTenantTx(t, testpkg.Ctx(t), db, testpkg.Tenant(t), func(ctx context.Context, tx bun.Tx) error {
			require.Equal(t, before, setting(ctx, tx), "the next transaction starts without the bound")
			_, err := tx.ExecContext(ctx, slow)
			return err
		}), "a statement in the next transaction outlasts the bound")
	})
	t.Run("import", func(t *testing.T) {
		testpkg.OwnTenant(t)
		_, err := db.ExecContext(context.Background(), `
			CREATE OR REPLACE FUNCTION public.slow_import_checkpoint() RETURNS trigger LANGUAGE plpgsql AS $$
			BEGIN PERFORM pg_sleep(0.3); RETURN NULL; END $$;
			CREATE TRIGGER slow_import_checkpoint AFTER INSERT ON audit.data_imports FOR EACH ROW EXECUTE FUNCTION public.slow_import_checkpoint()`)
		require.NoError(t, err)
		t.Cleanup(func() {
			_, _ = db.ExecContext(context.Background(), `DROP TRIGGER IF EXISTS slow_import_checkpoint ON audit.data_imports`)
		})
		for _, fallback := range []bool{false, true} {
			t.Run(fmt.Sprintf("fallback=%t", fallback), func(t *testing.T) {
				testpkg.OwnTenant(t)
				module, err := api.NewImportTestModuleWithOptions(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db), api.ImportTestOptions{BatchStatementLimit: limit})
				require.NoError(t, err)
				actor := newImporter(t, db)
				rows := overlapFile()
				var held bun.Tx
				if fallback {
					held = holdClassListKey(t, db, rows[2])
				}
				done := uploadClassList(t, module, db, actor, rows, "slow-checkpoint.csv")
				if fallback {
					waitForLockWaiters(t, db)
					// The hold outlasts the bound, the subject of the test.
					time.Sleep(300 * time.Millisecond)
					require.NoError(t, held.Rollback())
				}
				upload := awaitUpload(t, done)
				require.NoError(t, upload.err, "the checkpoint after the batch or the replay is not cancelled")
				require.Equal(t, 5, upload.result.CreatedCount)
				fallbacks := 0
				if fallback {
					fallbacks = 1
				}
				require.Equal(t, fallbacks, (*module.Observations)[0].BatchFallbacks)
			})
		}
	})
}
