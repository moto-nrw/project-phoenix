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
	importModels "github.com/moto-nrw/project-phoenix/modules/dataimport"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// Fault matrix of the student batch command (#5552 step 5a; tables A, B and
// C of the PR). A recorded upload names every statement the chunk issues
// from its savepoint on: the protected name read of ResolveChunk, the person
// statement under the bound and every owner write after it, inner
// savepoints included; a new statement joins the matrix by itself.
//
// Part 1: each statement fails once with each kind of main error while every
// cleanup succeeds. The batch path must end as the row path does when the
// same statement fails there: with the same status and the same persistence.
// Where the batch command replays the chunk (a failed write that is neither a
// broken connection nor a failed savepoint statement), the replay is a
// row-path run after the fault, so the batch path ends as the row path
// without it.
//
// Part 2: the whole cross of main error × cleanup point × cleanup error for
// the first and the last statement of each equivalence class, and for every
// other statement the main errors 40P01, 57014 and one without SQLSTATE ×
// every cleanup point of its class × every cleanup error. The executor
// reads two things of a failing statement only: whether it ran in the
// bounded call, and which savepoints enclose it, which fixes the cleanup
// statements after it. Its SQL enters no decision. The classes are those two
// properties.

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

// outerRollback is the cleanup point of the chunk transaction's ROLLBACK.
const outerRollback = -1

// matrixFault fails the statement at index at (counted from the upload's
// first statement) with err. When cleanup is set, the cleanup-th savepoint
// cleanup statement after it fails with err2, or the transaction ROLLBACK
// after it for outerRollback.
type matrixFault struct {
	at      int
	err     error
	cleanup int
	err2    error
}

// matrixUpload is what one upload of the matrix file left behind.
type matrixUpload struct {
	status           string
	outcome, before  studentOutcome
	retries, replays int
	deadlocks        int
	statements       []string
	primary, cleaned bool // the faults fired
}

func (u matrixUpload) persisted() map[string][]string { return u.outcome.Rows }

// matrixFile is the fixture: six children in one segment, each with consent
// dates (consent history), arrival and pickup plans, departure days, a
// privacy consent and a guardian with two phones. The first child's guardian
// exists already and is reused by e-mail; the third and the fifth share a
// new guardian, which the fifth finds as the row path does.
func matrixFile(t *testing.T, db *bun.DB) []importModels.StudentImportRow {
	t.Helper()
	testpkg.CreateTestGuardianProfileWithExactEmail(t, db, testpkg.Tenant(t), testpkg.GuardianProfileWithEmail{
		FirstName: "Eltern", LastName: "Vorhanden", Email: "matrix.vorhanden@example.test"})
	rows := make([]importModels.StudentImportRow, matrixRows)
	for i := range rows {
		name := fmt.Sprintf("Matrix%d", i)
		email := strings.ToLower(name) + "@example.test"
		switch i {
		case 0:
			email = "matrix.vorhanden@example.test"
		case 2, 4:
			email = "matrix.familie@example.test"
		}
		rows[i] = importModels.StudentImportRow{
			FirstName: name, LastName: "Matrix", SchoolClass: "4A", Birthday: "2017-04-05", DataRetentionDays: 30,
			PrivacyAccepted: true, AGBAcceptedAt: "2026-09-01", DataProcessingAcceptedAt: "2026-09-02",
			DepartureDays: map[string]string{"mon": "bus", "tue": "pickup"},
			Guardians: []importModels.GuardianImportData{{
				FirstName: "Eltern", LastName: name, Email: email, RelationshipType: "Mutter",
				PhoneNumbers: []importModels.PhoneImportData{
					{PhoneNumber: fmt.Sprintf("0151 70%04d", i), PhoneType: "mobile"},
					{PhoneNumber: fmt.Sprintf("0221 80%04d", i), PhoneType: "home"},
				},
			}},
			PickupSchedules:  []importModels.PickupScheduleImportData{{Weekday: 2, PickupTime: "15:30"}},
			ArrivalSchedules: []importModels.ArrivalScheduleImportData{{Weekday: 1, ExpectedArrival: "07:45"}},
		}
	}
	return rows
}

// runMatrixUpload imports the matrix file on its own tenant of the package
// clone over a pool that records every statement and injects fault.
func runMatrixUpload(t *testing.T, db *bun.DB, dsn string, options api.ImportTestOptions, fault *matrixFault) matrixUpload {
	t.Helper()
	testpkg.OwnTenant(t)
	var (
		mu         sync.Mutex
		armed      atomic.Bool
		count      int
		cleanups   int
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
			case upload.primary && !upload.cleaned && fault.cleanup > 0 && isSavepointCleanup(query):
				cleanups++
				if cleanups == fault.cleanup {
					upload.cleaned = true
					return fault.err2
				}
			}
			return nil
		},
		rollback: func() error {
			if !armed.Load() || fault == nil || fault.cleanup != outerRollback {
				return nil
			}
			mu.Lock()
			defer mu.Unlock()
			if !upload.primary || upload.cleaned {
				return nil
			}
			upload.cleaned = true
			return fault.err2
		},
	})))
	t.Cleanup(func() { _ = pool.Close() })
	module, err := api.NewImportTestModuleWithOptions(testpkg.NewDatabase(t, pool), testpkg.TenantRuntime(t, pool), options)
	require.NoError(t, err)
	actor := newImporter(t, db)
	rows := matrixFile(t, db)
	upload.before = studentOutcomeOf(t, db, nil)
	armed.Store(true)
	done := awaitStudents(t, uploadStudents(t, module, pool, actor, rows, "matrix.csv"))
	armed.Store(false)
	mu.Lock()
	upload.statements = statements
	mu.Unlock()
	upload.status = uploadStatus(done.err)
	upload.outcome = studentOutcomeOf(t, db, done.result)
	observation := (*module.Observations)[0]
	upload.retries, upload.replays, upload.deadlocks = observation.BatchesRetried, observation.BatchFallbacks, observation.Deadlocks
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

// matrixClass is an equivalence class of statements (see above).
type matrixClass string

const (
	classBounded         matrixClass = "K1 bounded"
	classChunk           matrixClass = "K2 in the chunk savepoint"
	classOwner           matrixClass = "K3 in an owner savepoint"
	classOwnerControl    matrixClass = "K4 owner savepoint statement"
	classResolverRead    matrixClass = "K5 resolver read"
	classResolverControl matrixClass = "K5 resolver savepoint statement"
)

// cleanupPoints are the savepoint cleanup statements a failure of the class
// runs through, numbered in order, and the chunk transaction's ROLLBACK.
var cleanupPoints = map[matrixClass][]int{
	classBounded:         {1, 2, outerRollback}, // chunk ROLLBACK TO, RELEASE
	classChunk:           {1, 2, outerRollback},
	classOwner:           {1, 2, 3, 4, outerRollback}, // inner ROLLBACK TO, RELEASE, then the chunk's
	classOwnerControl:    {1, 2, outerRollback},       // the chunk's
	classResolverRead:    {1, 2, outerRollback},       // the read's
	classResolverControl: {1, 2, outerRollback},       // the chunk's
}

// matrixStatement is one statement of the chunk in the recording.
type matrixStatement struct {
	kind  string
	index int // in the batch-path recording
	row   int // index of the same statement in the row-path recording, -1 for none
	class matrixClass
	// group splits a class by the owner savepoint around the statement.
	group string
}

// chunkStatements lists the statements of the chunk in the batch recording,
// classified, with the last occurrence of each kind per class; every one but
// the resolver's is mapped to the same statement on the row path: the
// person statement to the first person insert, every other statement to its
// occurrence after that insert. Bound statements are not the batch command.
func chunkStatements(t *testing.T, batch, rowPath []string) []matrixStatement {
	t.Helper()
	firstPerson := -1
	for i, query := range rowPath {
		if strings.HasPrefix(strings.TrimSpace(query), `WITH "staff_person_merge_gate"`) {
			firstPerson = i
			break
		}
	}
	require.Positive(t, firstPerson, "the row path recording creates a person")
	type key struct {
		class matrixClass
		kind  string
	}
	last := map[key]matrixStatement{}
	occurrence := map[string]int{}
	var order []key
	depth, phase, group := 0, "", ""
	for i, query := range batch {
		query = strings.TrimSpace(query)
		kind := statementKind(query)
		if strings.HasPrefix(kind, "RELEASE SAVEPOINT ") || strings.HasPrefix(kind, "ROLLBACK TO SAVEPOINT ") {
			depth--
		}
		at := depth
		if strings.HasPrefix(kind, "SAVEPOINT ") || strings.HasPrefix(kind, "ROLLBACK TO SAVEPOINT ") {
			depth++
		}
		switch {
		case phase == "" && at == 0 && strings.HasPrefix(kind, "SAVEPOINT "):
			phase = "resolve" // the chunk savepoint opened
			continue
		case phase == "":
			continue
		case at == 0:
			phase = "done" // the chunk's own release ends it
		case strings.HasPrefix(query, boundSet):
			phase = "bound"
			continue
		case strings.HasPrefix(query, boundReset):
			phase = "command"
			continue
		}
		if phase == "done" {
			break
		}
		if at == 1 && strings.HasPrefix(kind, "SAVEPOINT ") && phase == "command" {
			group = ""
		} else if at == 2 && group == "" {
			group = kind
			if len(group) > 40 {
				group = group[:40]
			}
		}
		var class matrixClass
		switch {
		case phase == "resolve" && isSavepointControl(kind):
			class = classResolverControl
		case phase == "resolve":
			class = classResolverRead
		case phase == "bound":
			class = classBounded
		case isSavepointControl(kind):
			class = classOwnerControl
		case at >= 2:
			class = classOwner
		default:
			class = classChunk
		}
		if phase != "resolve" {
			occurrence[kind]++
		}
		k := key{class, kind}
		if _, seen := last[k]; !seen {
			order = append(order, k)
		}
		statement := matrixStatement{kind: kind, index: i, class: class, row: occurrence[kind]}
		if class == classOwner {
			statement.group = group
		}
		last[k] = statement
	}
	require.Equal(t, "done", phase, "the recording holds a whole batch attempt")
	statements := make([]matrixStatement, 0, len(order))
	for _, k := range order {
		statement := last[k]
		switch k.class {
		case classResolverRead, classResolverControl:
			statement.row = -1 // the row path has no resolver read
		default:
			statement.row = rowPathIndex(t, rowPath, firstPerson, statement.kind, statement.row)
		}
		statements = append(statements, statement)
	}
	return statements
}

func rowPathIndex(t *testing.T, rowPath []string, firstPerson int, kind string, occurrence int) int {
	t.Helper()
	if strings.HasPrefix(kind, personBatch) {
		return firstPerson
	}
	if table, bundled := bundledLeafTable(kind); bundled {
		// A bundled leaf statement stands for the first single insert of its
		// table on the row path (#5552, step 5b).
		for i := firstPerson; i < len(rowPath); i++ {
			if strings.HasPrefix(statementKind(rowPath[i]), table) {
				return i
			}
		}
		t.Fatalf("the row path issues no insert into %q", table)
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

// bundledLeafTables are the leaf inserts the batch command bundles after the
// rows (#5552, step 5b).
var bundledLeafTables = []string{"INSERT INTO users.privacy_consents ", "INSERT INTO audit.student_consent_changes "}

// bundledLeafTable names the table of a bundled leaf insert: one of
// bundledLeafTables with more than one VALUES row.
func bundledLeafTable(kind string) (string, bool) {
	for _, table := range bundledLeafTables {
		if strings.HasPrefix(kind, table) && strings.Contains(kind, "), (") {
			return table, true
		}
	}
	return "", false
}

// replays says whether a failure of the statement makes the batch command
// replay the chunk (table A: P, or N for the bound): a write that fails with
// anything but a broken connection. A failed savepoint statement ends the
// upload (500, no retry, no replay).
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

// requireLost is F of table A: 500, no retry, no replay, nothing of the
// chunk persisted.
func requireLost(t *testing.T, got matrixUpload) {
	t.Helper()
	require.Equal(t, "500", got.status)
	require.Zero(t, got.retries, "no retry")
	require.Zero(t, got.replays, "no replay")
	require.Equal(t, got.before.Rows, got.persisted(), "nothing of the chunk persisted")
}

var matrixFailures = []error{
	injectedSQLState("57014"), injectedSQLState("40P01"), injectedSQLState("40001"), injectedSQLState("23514"),
	errors.New("injected failure without SQLSTATE"), driver.ErrBadConn,
}

// TestStudentBatchCommandFaultMatrix runs the matrix described above on the
// package clone: each upload has a tenant of its own.
func TestStudentBatchCommandFaultMatrix(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	dsn := testpkg.PackageTestDSN(t)
	var batchRecording, rowRecording, clean matrixUpload
	t.Run("recording", func(t *testing.T) {
		batchRecording = runMatrixUpload(t, db, dsn, api.ImportTestOptions{}, nil)
		require.Equal(t, "200", batchRecording.status)
		require.Equal(t, 1, countKind(batchRecording.statements, personBatch), "the batch path ran")
		require.Zero(t, batchRecording.replays)
	})
	t.Run("row path recording", func(t *testing.T) {
		rowRecording = runMatrixUpload(t, db, dsn, api.ImportTestOptions{RowPathOnly: true}, nil)
		clean = rowRecording
	})
	require.Equal(t, "200", clean.status)
	requireSameEnd(t, clean, batchRecording)
	statements := chunkStatements(t, batchRecording.statements, rowRecording.statements)
	classes := map[matrixClass]bool{}
	bundled := 0
	for _, statement := range statements {
		classes[statement.class] = true
		if _, ok := bundledLeafTable(statement.kind); ok {
			bundled++
			require.Equal(t, classBounded, statement.class, "a bundled leaf statement runs under the bound (E13): %s", statement.kind)
		}
	}
	require.Equal(t, len(bundledLeafTables), bundled, "the consents and the consent history are bundled")
	require.Len(t, classes, len(cleanupPoints), "every class is in the recording")

	// Part 1: every statement × every main error, cleanups succeed.
	var mainOnly sync.Map // "index err" -> matrixUpload of the batch path
	t.Run("main error", func(t *testing.T) {
		for _, statement := range statements {
			for _, err := range matrixFailures {
				t.Run(fmt.Sprintf("%s %s × %v", statement.class, statement.kind, err), func(t *testing.T) {
					t.Parallel()
					// Each upload runs in a subtest of its own: OwnTenant gives it a
					// tenant of its own.
					var batch, rowPath matrixUpload
					t.Run("batch", func(t *testing.T) {
						batch = runMatrixUpload(t, db, dsn, api.ImportTestOptions{}, &matrixFault{at: statement.index, err: err})
					})
					require.True(t, batch.primary, "the fault fired")
					mainOnly.Store(fmt.Sprint(statement.index, err), batch)
					switch statement.class {
					case classResolverRead:
						// L: the read falls back to FindExisting per row.
						requireSameEnd(t, clean, batch)
						require.Zero(t, batch.retries)
						require.Zero(t, batch.replays)
						return
					case classResolverControl:
						requireLost(t, batch)
						return
					}
					t.Run("row path", func(t *testing.T) {
						rowPath = runMatrixUpload(t, db, dsn, api.ImportTestOptions{RowPathOnly: true}, &matrixFault{at: statement.row, err: err})
					})
					require.True(t, rowPath.primary, "the fault fired on the row path")
					want := rowPath
					if replays(statement, err) && rowPath.status != "200" {
						want = clean
						require.Equal(t, 1, batch.replays, "the batch command replayed the chunk")
					}
					requireSameEnd(t, want, batch)
					if batch.status == "500" {
						require.Zero(t, batch.retries, "no retry after a failure the row path does not retry")
						require.Zero(t, batch.replays, "no replay")
					}
				})
			}
		}
	})

	// Part 2: main error × cleanup point × cleanup error, per class: the
	// first and the last statement of each class (and owner savepoint) with
	// every main error, every other one with the minimum main errors; each
	// against every cleanup point of its class and every cleanup error.
	cases := 0
	members := map[string][]int{}
	var classOrder []string
	for i, statement := range statements {
		class := string(statement.class) + " " + statement.group
		if _, seen := members[class]; !seen {
			classOrder = append(classOrder, class)
		}
		members[class] = append(members[class], i)
	}
	t.Run("cleanup", func(t *testing.T) {
		for _, class := range classOrder {
			indices := members[class]
			for position, at := range indices {
				statement := statements[at]
				mains := minimumCleanupFailures
				if position == 0 || position == len(indices)-1 {
					mains = matrixFailures
				}
				for _, err := range mains {
					for _, point := range cleanupPoints[statement.class] {
						for _, err2 := range matrixFailures {
							cases++
							t.Run(fmt.Sprintf("%s %s × %v, cleanup %d × %v", statement.class, statement.kind, err, point, err2), func(t *testing.T) {
								t.Parallel()
								runCleanupCase(t, db, dsn, clean, &mainOnly, statement, err, point, err2)
							})
						}
					}
				}
			}
		}
	})
	t.Logf("%d statements in %d classes; %d main-error cases, %d cleanup cases", len(statements), len(classOrder), len(statements)*len(matrixFailures), cases)
}

// minimumCleanupFailures are the main errors every statement that does not
// stand first or last for its class is crossed with.
var minimumCleanupFailures = []error{
	injectedSQLState("40P01"), injectedSQLState("57014"), errors.New("injected failure without SQLSTATE"),
}

// runCleanupCase is one cell of part 2 (tables A and B of the PR).
func runCleanupCase(t *testing.T, db *bun.DB, dsn string, clean matrixUpload, mainOnly *sync.Map, statement matrixStatement, err error, point int, err2 error) {
	t.Helper()
	got := runMatrixUpload(t, db, dsn, api.ImportTestOptions{}, &matrixFault{at: statement.index, err: err, cleanup: point, err2: err2})
	require.True(t, got.primary, "the fault fired")
	if err != injectedSQLState("40P01") {
		require.Zero(t, got.deadlocks, "a cleanup's 40P01 is no deadlock of the upload")
	}
	if point == outerRollback {
		// The transaction's ROLLBACK is diagnostics: the upload ends as with
		// the main error alone (#6316).
		value, ok := mainOnly.Load(fmt.Sprint(statement.index, err))
		require.True(t, ok)
		want := value.(matrixUpload)
		if want.status != "200" || want.retries > 0 {
			require.True(t, got.cleaned, "the ROLLBACK fault fired")
		}
		requireSameEnd(t, want, got)
		require.Equal(t, want.retries, got.retries)
		require.Equal(t, want.replays, got.replays)
		return
	}
	require.True(t, got.cleaned, "the cleanup fault fired")
	if statement.class == classBounded && err == injectedSQLState("57014") {
		// E13′: the bound cancelled the bundled statement, whose connection
		// pgdriver closes; the chunk replays in a new transaction whatever its
		// rollback returned.
		requireSameEnd(t, clean, got)
		require.Equal(t, 1, got.replays)
		require.Zero(t, got.retries)
		return
	}
	requireLost(t, got) // E19
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
