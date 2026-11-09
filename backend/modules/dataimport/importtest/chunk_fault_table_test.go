package integration

import (
	"database/sql"
	"database/sql/driver"
	"errors"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun/driver/pgdriver"

	"github.com/moto-nrw/project-phoenix/api"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// Fault table of the chunk executor (#5827 review R2; the table is in the PR).
// Only a failure of the batch command itself replays the chunk. Every other
// failure behaves as the same failure at the row path's first write: a
// 40P01/40001 costs one retry of the whole chunk transaction, anything else
// ends the upload with the error; no fallback, no write of the failed attempt.

// injectedSQLState is a database error with a SQLSTATE, as the transaction
// runtime and the executor classify them (Field('C')).
type injectedSQLState string

func (s injectedSQLState) Error() string { return "injected SQLSTATE " + string(s) }
func (s injectedSQLState) Field(field byte) string {
	if field == 'C' {
		return string(s)
	}
	return ""
}

// faultOutcome is what one injection must leave behind.
type faultOutcome struct {
	failsWith string // "" for success, a SQLSTATE, or "bad connection"
	retries   int
	fallbacks int
	entries   int
}

type faultCase struct {
	point, prefix string
	nth           int32
	err           error
	refuseRow     bool // the third row is refused, so the batch command fails
	want          faultOutcome
}

const (
	boundSet   = "SELECT current_setting('statement_timeout')"
	boundReset = "SELECT set_config('statement_timeout'"
	savepoint  = "SAVEPOINT phoenix_operation"
	release    = "RELEASE SAVEPOINT phoenix_operation"
	rollbackTo = "ROLLBACK TO SAVEPOINT phoenix_operation"
	batchMerge = "MERGE INTO users.class_list_entries"
	checkpoint = "INSERT INTO audit.data_imports"
	batchAudit = "INSERT INTO audit.class_list_entry_changes"
	matchRead  = "SELECT k.ord"
	personRead = `SELECT "person"."id"`
)

func runFaultTable(t *testing.T, cases []faultCase) {
	t.Helper()
	db := testpkg.SetupIsolatedTestDB(t)
	dsn := testpkg.IsolatedTestDSN(t)
	installRefusedMarkers(t, db)
	for _, c := range cases {
		t.Run(c.point+" × "+c.err.Error(), func(t *testing.T) {
			testpkg.OwnTenant(t)
			armed := &atomic.Bool{}
			pool := testpkg.AdoptPrivatePool(t, testpkg.NewBunDB(sql.OpenDB(faultConnector{
				Connector: pgdriver.NewConnector(pgdriver.WithDSN(dsn)), armed: armed, prefix: c.prefix,
				nth: c.nth, err: c.err, matches: &atomic.Int32{},
			})))
			t.Cleanup(func() { _ = pool.Close() })
			module, err := api.NewImportTestModule(testpkg.NewDatabase(t, pool), testpkg.TenantRuntime(t, pool))
			require.NoError(t, err)
			actor := newImporter(t, db)
			rows := overlapFile()
			if c.refuseRow {
				rows[2].LastName = "Refused"
			}
			armed.Store(true)
			upload := awaitUpload(t, uploadClassList(t, module, pool, actor, rows, "fault.csv"))
			require.False(t, armed.Load(), "the fault fired")
			observation := (*module.Observations)[0]
			got := faultOutcome{retries: observation.BatchesRetried, fallbacks: observation.BatchFallbacks,
				entries: countIn(t, db, `SELECT count(*) FROM users.class_list_entries WHERE tenant_id = ?`)}
			var state interface{ Field(byte) string }
			switch {
			case upload.err == nil:
			case errors.Is(upload.err, driver.ErrBadConn):
				got.failsWith = "bad connection"
			case errors.As(upload.err, &state):
				got.failsWith = state.Field('C')
			default:
				got.failsWith = upload.err.Error()
			}
			require.Equal(t, c.want, got, "%v", upload.err)
		})
	}
}

// TestChunkFaultsOutsideTheBatchCommandBehaveLikeTheRowPath: the statements
// around the batch command (savepoint, bound) never replay.
func TestChunkFaultsOutsideTheBatchCommandBehaveLikeTheRowPath(t *testing.T) {
	t.Parallel()
	retried := faultOutcome{retries: 1, entries: 5}
	runFaultTable(t, []faultCase{
		{point: "bound set", prefix: boundSet, err: injectedSQLState("40P01"), want: retried},
		{point: "bound set", prefix: boundSet, err: injectedSQLState("40001"), want: retried},
		{point: "bound set", prefix: boundSet, err: injectedSQLState("57014"), want: faultOutcome{failsWith: "57014"}},
		{point: "bound set", prefix: boundSet, err: injectedSQLState("55P03"), want: faultOutcome{failsWith: "55P03"}},
		{point: "bound set", prefix: boundSet, err: injectedSQLState("23514"), want: faultOutcome{failsWith: "23514"}},
		{point: "bound set", prefix: boundSet, err: driver.ErrBadConn, want: faultOutcome{failsWith: "bad connection"}},
		{point: "bound reset", prefix: boundReset, err: injectedSQLState("40P01"), want: retried},
		{point: "bound reset", prefix: boundReset, err: injectedSQLState("57014"), want: faultOutcome{failsWith: "57014"}},
		{point: "bound reset", prefix: boundReset, err: driver.ErrBadConn, want: faultOutcome{failsWith: "bad connection"}},
		{point: "chunk savepoint", prefix: savepoint, err: injectedSQLState("40P01"), want: retried},
		{point: "chunk savepoint", prefix: savepoint, err: driver.ErrBadConn, want: faultOutcome{failsWith: "bad connection"}},
		// The first RELEASE is ResolveChunk's protected read, the second the chunk's.
		{point: "chunk release", prefix: release, nth: 2, err: injectedSQLState("40P01"), want: retried},
		{point: "chunk release", prefix: release, nth: 2, err: driver.ErrBadConn, want: faultOutcome{failsWith: "bad connection"}},
		{point: "chunk rollback after a refused batch", prefix: rollbackTo, err: driver.ErrBadConn, refuseRow: true, want: faultOutcome{failsWith: "bad connection"}},
		// The first person read is ResolveChunk's, the second the student guard's.
		{point: "student guard read", prefix: personRead, nth: 2, err: injectedSQLState("40P01"), want: retried},
		{point: "student guard read", prefix: personRead, nth: 2, err: driver.ErrBadConn, want: faultOutcome{failsWith: "bad connection"}},
		// ResolveChunk is a protected read: it falls back to FindExisting per
		// row (ADR F3), with no retry and no replay.
		{point: "ResolveChunk entry read", prefix: matchRead, err: injectedSQLState("40P01"), want: faultOutcome{entries: 5}},
		{point: "ResolveChunk person read", prefix: personRead, nth: 1, err: injectedSQLState("40P01"), want: faultOutcome{entries: 5}},
		{point: "checkpoint", prefix: checkpoint, err: injectedSQLState("40P01"), want: retried},
		{point: "checkpoint", prefix: checkpoint, err: driver.ErrBadConn, want: faultOutcome{failsWith: "bad connection"}},
	})
}

// TestChunkFaultsInTheBatchCommandReplay: a failure of the batch command
// itself replays the chunk in the same transaction (57014: in a new one,
// E13′), consuming no retry; a broken connection is returned.
func TestChunkFaultsInTheBatchCommandReplay(t *testing.T) {
	t.Parallel()
	replayed := faultOutcome{fallbacks: 1, entries: 5}
	runFaultTable(t, []faultCase{
		{point: "batch command", prefix: batchMerge, err: injectedSQLState("40P01"), want: replayed},
		{point: "batch command", prefix: batchMerge, err: injectedSQLState("40001"), want: replayed},
		{point: "batch command", prefix: batchMerge, err: injectedSQLState("57014"), want: replayed},
		{point: "batch command", prefix: batchMerge, err: injectedSQLState("55P03"), want: replayed},
		{point: "batch command", prefix: batchMerge, err: injectedSQLState("23514"), want: replayed},
		{point: "batch command", prefix: batchMerge, err: driver.ErrBadConn, want: faultOutcome{failsWith: "bad connection"}},
		{point: "batch command audit", prefix: batchAudit, err: injectedSQLState("40P01"), want: replayed},
		{point: "batch command audit", prefix: batchAudit, err: driver.ErrBadConn, want: faultOutcome{failsWith: "bad connection"}},
	})
}
