package integration

import (
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/driver/pgdriver"

	"github.com/moto-nrw/project-phoenix/api"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// Fault matrix of the student batch command (#5552 review of 1bb4bde258).
// A recorded upload names every statement the batch command issues, inner
// savepoints included; a new statement joins the matrix by itself. Each
// statement fails once with each kind of error, and each statement inside an
// inner savepoint fails once more with its rollback failing after it. The
// batch path must end as the row path does when the same statement fails
// there: with the same status and the same persistence. Where the batch
// command replays the chunk (a failed bundled or row write that is neither a
// broken connection nor a failed savepoint), the replay is a row-path run
// after the fault, so the batch path ends as the row path without it.

const matrixRows = 6

var (
	sqlLiteral = regexp.MustCompile(`'(?:[^']|'')*'`)
	sqlNumber  = regexp.MustCompile(`\b\d+\b`)
	sqlSpace   = regexp.MustCompile(`\s+`)
)

// statementKind is a statement without its literals: the same statement for
// another row or school has the same kind.
func statementKind(query string) string {
	kind := sqlSpace.ReplaceAllString(strings.TrimSpace(query), " ")
	kind = sqlLiteral.ReplaceAllString(kind, "'?'")
	return sqlNumber.ReplaceAllString(kind, "#")
}

func isSavepointControl(kind string) bool {
	return strings.HasPrefix(kind, "SAVEPOINT ") || strings.HasPrefix(kind, "RELEASE SAVEPOINT ") || strings.HasPrefix(kind, "ROLLBACK TO SAVEPOINT ")
}

// matrixFault fails the statement at index at (counted from the upload's
// first statement) with err, and then the first ROLLBACK TO SAVEPOINT after
// it with err2, if set.
type matrixFault struct {
	at        int
	err, err2 error
}

// matrixUpload is what one upload of the matrix file left behind.
type matrixUpload struct {
	status            string
	outcome           studentOutcome
	retries, replays  int
	statements        []string
	primary, rollback bool // the faults fired
}

func (u matrixUpload) persisted() map[string][]string { return u.outcome.Rows }

// runMatrixUpload imports the matrix file on its own tenant over a pool that
// records every statement and injects fault.
func runMatrixUpload(t *testing.T, db *bun.DB, dsn string, options api.ImportTestOptions, fault *matrixFault) matrixUpload {
	t.Helper()
	testpkg.OwnTenant(t)
	var (
		mu         sync.Mutex
		armed      atomic.Bool
		count      int
		statements []string
		upload     matrixUpload
	)
	pool := testpkg.AdoptPrivatePool(t, testpkg.NewBunDB(sql.OpenDB(faultConnector{
		Connector: pgdriver.NewConnector(pgdriver.WithDSN(dsn)),
		decide: func(query string) error {
			if !armed.Load() {
				return nil
			}
			mu.Lock()
			defer mu.Unlock()
			index := count
			count++
			statements = append(statements, query)
			switch {
			case fault == nil:
			case index == fault.at:
				upload.primary = true
				return fault.err
			case upload.primary && !upload.rollback && fault.err2 != nil && strings.HasPrefix(strings.TrimSpace(query), "ROLLBACK TO SAVEPOINT"):
				upload.rollback = true
				return fault.err2
			}
			return nil
		},
	})))
	t.Cleanup(func() { _ = pool.Close() })
	module, err := api.NewImportTestModuleWithOptions(testpkg.NewDatabase(t, pool), testpkg.TenantRuntime(t, pool), options)
	require.NoError(t, err)
	actor := newImporter(t, db)
	armed.Store(true)
	done := awaitStudents(t, uploadStudents(t, module, pool, actor, namedStudentBatch("Matrix", matrixRows), "matrix.csv"))
	armed.Store(false)
	mu.Lock()
	upload.statements = statements
	mu.Unlock()
	upload.status = uploadStatus(done.err)
	upload.outcome = studentOutcomeOf(t, db, done.result)
	observation := (*module.Observations)[0]
	upload.retries, upload.replays = observation.BatchesRetried, observation.BatchFallbacks
	return upload
}

// uploadStatus is the answer the HTTP adapter derives from an upload error.
func uploadStatus(err error) string {
	var refusal interface{ ErrorCode() string }
	switch {
	case err == nil:
		return "200"
	case errors.As(err, &refusal):
		return "409 " + refusal.ErrorCode()
	default:
		return "500"
	}
}

// matrixStatement is one statement of the batch command in the recording.
type matrixStatement struct {
	kind  string
	index int  // in the batch-path recording
	row   int  // index of the same statement in the row-path recording
	inner bool // runs inside a savepoint of an owner command
}

// batchCommandStatements lists the last statement of each kind the batch
// command issued, and maps it to the same statement on the row path: the
// person statement to the first person insert, every other statement to its
// occurrence after that insert. Bound statements are not the batch command.
func batchCommandStatements(t *testing.T, batch, rowPath []string) []matrixStatement {
	t.Helper()
	start := -1
	for i, query := range batch {
		if strings.HasPrefix(strings.TrimSpace(query), boundSet) {
			start = i + 1
			break
		}
	}
	require.Positive(t, start, "the recording holds a bounded attempt")
	firstPerson := -1
	for i, query := range rowPath {
		if strings.HasPrefix(strings.TrimSpace(query), `WITH "staff_person_merge_gate"`) {
			firstPerson = i
			break
		}
	}
	require.Positive(t, firstPerson, "the row path recording creates a person")
	last := map[string]matrixStatement{}
	occurrence := map[string]int{}
	var order []string
	depth := 1
	for i := start; i < len(batch); i++ {
		query := strings.TrimSpace(batch[i])
		kind := statementKind(query)
		switch {
		case strings.HasPrefix(query, boundSet), strings.HasPrefix(query, boundReset):
			continue
		case strings.HasPrefix(kind, "SAVEPOINT "):
			depth++
		case strings.HasPrefix(kind, "RELEASE SAVEPOINT "):
			depth--
		}
		if depth == 0 {
			break // the chunk's own release ends the batch command
		}
		if _, seen := last[kind]; !seen {
			order = append(order, kind)
		}
		occurrence[kind]++
		last[kind] = matrixStatement{kind: kind, index: i, inner: depth >= 2 && !isSavepointControl(kind), row: occurrence[kind]}
	}
	statements := make([]matrixStatement, 0, len(order))
	for _, kind := range order {
		statement := last[kind]
		statement.row = rowPathIndex(t, rowPath, firstPerson, kind, statement.row)
		statements = append(statements, statement)
	}
	return statements
}

func rowPathIndex(t *testing.T, rowPath []string, firstPerson int, kind string, occurrence int) int {
	t.Helper()
	if strings.HasPrefix(kind, personBatch) {
		return firstPerson
	}
	seen := 0
	for i := firstPerson; i < len(rowPath); i++ {
		if statementKind(rowPath[i]) == kind {
			seen++
			if seen == occurrence {
				return i
			}
		}
	}
	t.Fatalf("the row path issues no statement %d of kind %q", occurrence, kind)
	return -1
}

// replays says whether a failure of the statement makes the batch command
// replay the chunk (fault table: P, or N for the bound): a write that fails
// with anything but a broken connection. A failed savepoint statement ends
// the upload (500, no retry, no replay).
func replays(statement matrixStatement, err error) bool {
	return !errors.Is(err, driver.ErrBadConn) && !isSavepointControl(statement.kind)
}

func requireSameEnd(t *testing.T, want, got matrixUpload) {
	t.Helper()
	require.Equal(t, want.status, got.status)
	require.Equal(t, want.persisted(), got.persisted(), "persistence")
	require.Equal(t, want.outcome.Created, got.outcome.Created)
	require.Equal(t, want.outcome.Updated, got.outcome.Updated)
	if want.status == "200" {
		require.Equal(t, want.outcome, got.outcome, "result")
	}
}

// TestStudentBatchCommandFaultMatrix runs the matrix described above.
func TestStudentBatchCommandFaultMatrix(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	dsn := testpkg.IsolatedTestDSN(t)
	var batchRecording, rowRecording, clean matrixUpload
	t.Run("recording", func(t *testing.T) {
		batchRecording = runMatrixUpload(t, db, dsn, api.ImportTestOptions{}, nil)
		require.Equal(t, 1, countKind(batchRecording.statements, personBatch), "the batch path ran")
	})
	t.Run("row path recording", func(t *testing.T) {
		rowRecording = runMatrixUpload(t, db, dsn, api.ImportTestOptions{RowPathOnly: true}, nil)
		clean = rowRecording
	})
	require.Equal(t, "200", clean.status)
	statements := batchCommandStatements(t, batchRecording.statements, rowRecording.statements)
	require.NotEmpty(t, statements)

	failures := []error{injectedSQLState("57014"), injectedSQLState("40P01"), injectedSQLState("40001"), driver.ErrBadConn, errors.New("injected failure without SQLSTATE")}
	type matrixCase struct {
		name      string
		statement matrixStatement
		err, err2 error
	}
	var cases []matrixCase
	for _, statement := range statements {
		for _, err := range failures {
			cases = append(cases, matrixCase{name: fmt.Sprintf("%s × %v", statement.kind, err), statement: statement, err: err})
		}
		if statement.inner {
			for _, err2 := range failures {
				cases = append(cases, matrixCase{name: fmt.Sprintf("%s × no SQLSTATE, rollback × %v", statement.kind, err2),
					statement: statement, err: errors.New("injected failure without SQLSTATE"), err2: err2})
			}
		}
	}
	// The subtests run concurrently without t.Parallel: this test holds the
	// package's one isolated clone (see TestImportBatchesRefusedRowKeepsTheRowPathOutcome).
	slots := make(chan struct{}, 6)
	var running sync.WaitGroup
	for _, c := range cases {
		slots <- struct{}{}
		running.Add(1)
		go func() {
			defer running.Done()
			defer func() { <-slots }()
			t.Run(c.name, func(t *testing.T) {
				var batch, rowPath matrixUpload
				t.Run("batch", func(t *testing.T) {
					batch = runMatrixUpload(t, db, dsn, api.ImportTestOptions{}, &matrixFault{at: c.statement.index, err: c.err, err2: c.err2})
				})
				t.Run("row path", func(t *testing.T) {
					rowPath = runMatrixUpload(t, db, dsn, api.ImportTestOptions{RowPathOnly: true}, &matrixFault{at: c.statement.row, err: c.err, err2: c.err2})
				})
				require.True(t, batch.primary && rowPath.primary, "the fault fired on both paths")
				require.Equal(t, c.err2 != nil, batch.rollback, "the rollback fault fired on the batch path")
				require.Equal(t, c.err2 != nil, rowPath.rollback, "the rollback fault fired on the row path")
				want := rowPath
				if c.err2 == nil && replays(c.statement, c.err) && rowPath.status != "200" {
					want = clean
					require.Equal(t, 1, batch.replays, "the batch command replayed the chunk")
				}
				requireSameEnd(t, want, batch)
				if batch.status == "500" {
					require.Zero(t, batch.retries, "no retry after a failure the row path does not retry")
					require.Zero(t, batch.replays, "no replay")
				}
			})
		}()
	}
	running.Wait()
	t.Logf("%d statements of the batch command, %d cases", len(statements), len(cases))
}

func countKind(statements []string, prefix string) int {
	count := 0
	for _, query := range statements {
		if strings.HasPrefix(strings.TrimSpace(query), prefix) {
			count++
		}
	}
	return count
}

// TestStudentBatchQuotaRefusalMatchesTheRowPath is the business column of
// the matrix: the quota is reached at row 4. The batch command is refused,
// the replay names row 4 with the details of the row path.
func TestStudentBatchQuotaRefusalMatchesTheRowPath(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	run := func(t *testing.T, options api.ImportTestOptions) (string, studentOutcome, error) {
		testpkg.OwnTenant(t)
		testpkg.SetTestChildQuota(t, db, 1, 3)
		module, err := api.NewImportTestModuleWithOptions(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db), options)
		require.NoError(t, err)
		actor := newImporter(t, db)
		upload := awaitStudents(t, uploadStudents(t, module, db, actor, namedStudentBatch("Kontingent", matrixRows), "quota.csv"))
		return uploadStatus(upload.err), studentOutcomeOf(t, db, upload.result), upload.err
	}
	var batchStatus, rowStatus string
	var batch, rowPath studentOutcome
	var batchErr, rowErr error
	t.Run("batch", func(t *testing.T) { batchStatus, batch, batchErr = run(t, api.ImportTestOptions{}) })
	t.Run("row path", func(t *testing.T) { rowStatus, rowPath, rowErr = run(t, api.ImportTestOptions{RowPathOnly: true}) })
	require.Equal(t, "409 students.child_quota_reached", rowStatus)
	require.Equal(t, rowStatus, batchStatus)
	require.Equal(t, rowPath, batch)
	var batchDetails, rowDetails interface{ ErrorDetails() any }
	require.ErrorAs(t, batchErr, &batchDetails)
	require.ErrorAs(t, rowErr, &rowDetails)
	require.Equal(t, rowDetails.ErrorDetails(), batchDetails.ErrorDetails())
}
