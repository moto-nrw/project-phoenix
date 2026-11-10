package integration

import (
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"unicode/utf8"

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

// matrixFile is the fixture: thirteen children in two batch segments of one
// chunk, split by a child with a card, which takes the row path as a segment
// of its own. Each child has consent dates (consent history), arrival and
// pickup plans, departure days, a privacy consent and a guardian with two
// phones. The six rows M0…M5 carry guardians with e-mail, written in place:
// M0's guardian exists already and is reused by e-mail; M2 and M4 share a new
// guardian, which M4 finds as the row path does. The seven rows O0…O6 and the
// card row carry a guardian without e-mail; in the batch segments the O rows'
// guardians go into the bundled guardian statements (#5552 step 5c), the
// card row writes its own in place. Order: M0 M1 M2 M3 O0 O1 O2 | card | M4
// M5 O3 O4 O5 O6, so both batch segments reach ChunkMinRows.
func matrixFile(t *testing.T, db *bun.DB) []importModels.StudentImportRow {
	t.Helper()
	testpkg.CreateTestGuardianProfileWithExactEmail(t, db, testpkg.Tenant(t), testpkg.GuardianProfileWithEmail{
		FirstName: "Eltern", LastName: "Vorhanden", Email: "matrix.vorhanden@example.test"})
	card := testpkg.CreateTestRFIDCard(t, db, "MATRIX")
	row := func(name, email, secondType string, i int) importModels.StudentImportRow {
		return importModels.StudentImportRow{
			FirstName: name, LastName: "Matrix", SchoolClass: "4A", Birthday: "2017-04-05", DataRetentionDays: 30,
			PrivacyAccepted: true, AGBAcceptedAt: "2026-09-01", DataProcessingAcceptedAt: "2026-09-02",
			DepartureDays: map[string]string{"mon": "bus", "tue": "pickup"},
			Guardians: []importModels.GuardianImportData{{
				FirstName: "Eltern", LastName: name, Email: email, RelationshipType: "Mutter",
				PhoneNumbers: []importModels.PhoneImportData{
					{PhoneNumber: fmt.Sprintf("0151 70%04d", i), PhoneType: "mobile"},
					{PhoneNumber: fmt.Sprintf("0221 80%04d", i), PhoneType: secondType},
				},
			}},
			PickupSchedules:  []importModels.PickupScheduleImportData{{Weekday: 2, PickupTime: "15:30"}},
			ArrivalSchedules: []importModels.ArrivalScheduleImportData{{Weekday: 1, ExpectedArrival: "07:45"}},
		}
	}
	withEmail := func(i int) importModels.StudentImportRow {
		name := fmt.Sprintf("Matrix%d", i)
		email := strings.ToLower(name) + "@example.test"
		switch i {
		case 0:
			email = "matrix.vorhanden@example.test"
		case 2, 4:
			email = "matrix.familie@example.test"
		}
		return row(name, email, "home", i)
	}
	// An empty phone type is stored as "other" (mapPhoneType), on both paths.
	withoutEmail := func(i int) importModels.StudentImportRow {
		return row(fmt.Sprintf("Ohne%d", i), "", "", 100+i)
	}
	cardRow := row("Karte", "", "", 200)
	cardRow.TagID = card.ID
	return []importModels.StudentImportRow{
		withEmail(0), withEmail(1), withEmail(2), withEmail(3), withoutEmail(0), withoutEmail(1), withoutEmail(2),
		cardRow,
		withEmail(4), withEmail(5), withoutEmail(3), withoutEmail(4), withoutEmail(5), withoutEmail(6),
	}
}

// runMatrixUpload imports the matrix file on its own tenant of the package
// clone over a pool that records every statement and injects fault.
func runMatrixUpload(t *testing.T, db *bun.DB, dsn string, options api.ImportTestOptions, fault *matrixFault) matrixUpload {
	return runMatrixUploadOf(t, db, dsn, options, fault, matrixFile)
}

// runMatrixUploadOf is runMatrixUpload over the file fixture builds.
func runMatrixUploadOf(t *testing.T, db *bun.DB, dsn string, options api.ImportTestOptions, fault *matrixFault, fixture func(*testing.T, *bun.DB) []importModels.StudentImportRow) matrixUpload {
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
	rows := fixture(t, db)
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
	classBounded matrixClass = "K1 bounded"
	classChunk   matrixClass = "K2 in the chunk savepoint"
	// The owner savepoints of the enrollment and of a guardian link enclose
	// different statements; each is a class of its own (#5552 step 5c, table
	// review R1 point 4).
	classOwnerEnroll        matrixClass = "K3a in the enrollment savepoint"
	classOwnerLink          matrixClass = "K3b in a guardian link savepoint"
	classOwnerEnrollControl matrixClass = "K4a enrollment savepoint statement"
	classOwnerLinkControl   matrixClass = "K4b guardian link savepoint statement"
	// The inner savepoint of School Membership's counting write when the
	// school has a child quota (#5552 step 7, table review R1 finding 3):
	// in a batch row it sits in the chunk savepoint, in a row created in
	// place inside the enrollment savepoint.
	classQuota                matrixClass = "K6 in the quota savepoint"
	classQuotaControl         matrixClass = "K6c quota savepoint statement"
	classQuotaInEnroll        matrixClass = "K3a/K6 in the quota savepoint of an enrollment"
	classQuotaInEnrollControl matrixClass = "K3a/K6c quota savepoint statement of an enrollment"
	classResolverRead         matrixClass = "K5 resolver read"
	classResolverControl      matrixClass = "K5 resolver savepoint statement"
	// classBoundControl sets or resets a bound: not the batch command, so a
	// failure behaves as the row path's first write (table A: R for 40P01
	// and 40001, otherwise 500) and a failed cleanup after it ends the upload
	// (E19). Each set and reset of each bound of each segment is a statement
	// of its own in the recording.
	classBoundControl matrixClass = "bound set or reset"
)

// cleanupPoints are the savepoint cleanup statements a failure of the class
// runs through, numbered in order, and the chunk transaction's ROLLBACK.
var cleanupPoints = map[matrixClass][]int{
	classBounded:              {1, 2, outerRollback}, // chunk ROLLBACK TO, RELEASE
	classChunk:                {1, 2, outerRollback},
	classOwnerEnroll:          {1, 2, 3, 4, outerRollback}, // inner ROLLBACK TO, RELEASE, then the chunk's
	classOwnerLink:            {1, 2, 3, 4, outerRollback},
	classOwnerEnrollControl:   {1, 2, outerRollback}, // the chunk's
	classOwnerLinkControl:     {1, 2, outerRollback},
	classQuota:                {1, 2, 3, 4, outerRollback},       // inner ROLLBACK TO, RELEASE, then the chunk's
	classQuotaControl:         {1, 2, outerRollback},             // the chunk's
	classQuotaInEnroll:        {1, 2, 3, 4, 5, 6, outerRollback}, // quota, enrollment, chunk
	classQuotaInEnrollControl: {1, 2, 3, 4, outerRollback},       // enrollment, chunk
	classResolverRead:         {1, 2, outerRollback},             // the read's
	classResolverControl:      {1, 2, outerRollback},             // the chunk's
	classBoundControl:         {1, 2, outerRollback},             // the chunk's
}

// matrixStatement is one statement of the chunk in the recording.
type matrixStatement struct {
	kind  string
	index int // in the batch-path recording
	row   int // index of the same statement in the row-path recording, -1 for none
	class matrixClass
	// segment and phase place the statement: the batch segment (1, 2) and
	// where in it (resolve, bound 1, loop, bound 2, after it).
	segment int
	phase   string
}

// matrixLabelBytes bounds the statement text in a subtest name. A statement
// reaches 2,000 bytes; with it, the test framing lines of `go test -json`
// exceeded PIPE_BUF (512 bytes on macOS), so a concurrent write to stderr
// split them and the run lost its "run" event (#6488).
const matrixLabelBytes = 60

// label names the statement in a subtest name: its segment, phase, class,
// batch-path index and the start of its kind. logStatementOnFailure logs the whole kind.
func (s matrixStatement) label() string {
	kind := s.kind
	if len(kind) > matrixLabelBytes {
		cut := matrixLabelBytes
		for cut > 0 && !utf8.RuneStart(kind[cut]) {
			cut--
		}
		kind = kind[:cut] + "…"
	}
	return fmt.Sprintf("%d %s %s %d %s", s.segment, s.phase, s.class, s.index, kind)
}

// logStatementOnFailure names the whole statement of a failed case, which
// the subtest name only starts.
func logStatementOnFailure(t *testing.T, statement matrixStatement) {
	t.Cleanup(func() {
		if t.Failed() {
			t.Logf("statement %d: %s", statement.index, statement.kind)
		}
	})
}

// savepointOwner names the savepoint a SAVEPOINT opens from the statement
// after it: the resolver's name read, the enrollment or a guardian link.
func savepointOwner(next string) string {
	switch {
	case strings.HasPrefix(next, `SELECT "person"`):
		return "resolver"
	case strings.HasPrefix(next, enrollmentWrite) && strings.Contains(next, "INSERT INTO users.student_school_memberships"):
		return "quota"
	case strings.HasPrefix(next, enrollmentWrite):
		return "enroll"
	case strings.HasPrefix(next, "WITH linked_guardian"):
		return "link"
	default:
		return "other"
	}
}

// chunkStatements lists the statements of the chunk in the batch recording
// by position: one per kind for each place (phase, class), and for the
// resolver and every bound set and reset one per segment, so both bounds and
// each set and reset stay apart while the same statement of another row or
// segment in the same place, which the executor cannot tell apart, is
// represented once (the last). Every one but the
// resolver's and the bounds' is mapped to the same statement on the row
// path: the person statement to the first person insert, a bundled statement
// to the first single statement of its table, every other one to its
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
		segment     int
		phase, kind string
		class       matrixClass
	}
	last := map[key]matrixStatement{}
	occurrence := map[string]int{}
	var order []key
	record := func(k key, statement matrixStatement) {
		if _, seen := last[k]; !seen {
			order = append(order, k)
		}
		last[k] = statement
	}
	var owners []string // the open savepoints inside the chunk's
	segment, phase, started, done := 0, "", false, false
	for i := 0; i < len(batch) && !done; i++ {
		query := strings.TrimSpace(batch[i])
		kind := statementKind(query)
		if !started {
			if strings.HasPrefix(kind, "SAVEPOINT ") {
				started = true // the chunk savepoint opened
			}
			continue
		}
		var class matrixClass
		switch {
		case strings.HasPrefix(kind, "SAVEPOINT "):
			next := ""
			if i+1 < len(batch) {
				next = strings.TrimSpace(batch[i+1])
			}
			owner := savepointOwner(next)
			if owner == "resolver" && len(owners) == 0 {
				segment, phase = segment+1, "resolve"
			}
			owners = append(owners, owner)
			class = controlClass(t, owners, kind)
		case strings.HasPrefix(kind, "RELEASE SAVEPOINT ") || strings.HasPrefix(kind, "ROLLBACK TO SAVEPOINT "):
			if len(owners) == 0 {
				done = true // the chunk's own release ends it
				continue
			}
			owner := owners[len(owners)-1]
			class = controlClass(t, owners, kind)
			owners = owners[:len(owners)-1]
			if owner == "resolver" {
				phase = "resolved"
			}
		case strings.HasPrefix(query, boundSet):
			phase = map[string]string{"resolved": "bound 1", "loop": "bound 2"}[phase]
			require.NotEmpty(t, phase, "a bound is set after the resolver or the loop (statement %d)", i)
			record(key{segment, phase + " set", kind, classBoundControl}, matrixStatement{kind: kind, index: i, class: classBoundControl, row: -1, segment: segment, phase: phase + " set"})
			continue
		case strings.HasPrefix(query, boundReset):
			record(key{segment, phase + " reset", kind, classBoundControl}, matrixStatement{kind: kind, index: i, class: classBoundControl, row: -1, segment: segment, phase: phase + " reset"})
			phase = map[string]string{"bound 1": "loop", "bound 2": "after"}[phase]
			continue
		case len(owners) > 0 && owners[len(owners)-1] == "resolver":
			class = classResolverRead
		case phase == "bound 1" || phase == "bound 2":
			class = classBounded
		case len(owners) > 1 && owners[len(owners)-1] == "quota" && owners[len(owners)-2] == "enroll":
			class = classQuotaInEnroll
		case len(owners) > 0 && owners[len(owners)-1] == "quota":
			class = classQuota
		case len(owners) > 0 && owners[len(owners)-1] == "enroll":
			class = classOwnerEnroll
		case len(owners) > 0 && owners[len(owners)-1] == "link":
			class = classOwnerLink
		case len(owners) > 0:
			t.Fatalf("statement %d runs in a savepoint the matrix does not know: %s", i, kind)
		default:
			class = classChunk
		}
		if class != classResolverRead && class != classResolverControl {
			occurrence[kind]++
		}
		// Only the resolver and the bounds differ per segment for the executor;
		// any other statement of the same place, class and kind stands for
		// both segments (the last occurrence, so segment 2 when it has one).
		keySegment, keyKind := 0, kind
		if class == classResolverRead || class == classResolverControl {
			keySegment = segment
		}
		if phase == "after" {
			// The card row between the batch segments is written on the row
			// path: the executor tells its statements apart by class only.
			keyKind = ""
		}
		record(key{keySegment, phase, keyKind, class}, matrixStatement{kind: kind, index: i, class: class, row: occurrence[kind], segment: segment, phase: phase})
	}
	require.True(t, done, "the recording holds a whole batch attempt")
	statements := make([]matrixStatement, 0, len(order))
	for _, k := range order {
		statement := last[k]
		switch k.class {
		case classResolverRead, classResolverControl, classBoundControl:
			statement.row = -1 // the row path has no resolver read and no bound
		default:
			statement.row = rowPathIndex(t, rowPath, firstPerson, statement.kind, statement.row)
		}
		statements = append(statements, statement)
	}
	// The representatives stand in the order of their positions, so the
	// first and the last of a class are its smallest and largest index in
	// the recording, not its first key and its last overwrite.
	sort.Slice(statements, func(i, j int) bool { return statements[i].index < statements[j].index })
	return statements
}

// classExtremes are, per class, the smallest and the largest recording index
// among the class's statements: the first and last representatives part 2
// crosses fully.
func classExtremes(statements []matrixStatement) map[matrixClass][2]int {
	extremes := map[matrixClass][2]int{}
	for _, statement := range statements {
		current, seen := extremes[statement.class]
		if !seen {
			extremes[statement.class] = [2]int{statement.index, statement.index}
			continue
		}
		extremes[statement.class] = [2]int{min(current[0], statement.index), max(current[1], statement.index)}
	}
	return extremes
}

// controlClass is the class of a savepoint statement by the savepoint it
// belongs to.
func controlClass(t *testing.T, owners []string, kind string) matrixClass {
	t.Helper()
	owner := owners[len(owners)-1]
	if owner == "quota" {
		if len(owners) > 1 && owners[len(owners)-2] == "enroll" {
			return classQuotaInEnrollControl
		}
		return classQuotaControl
	}
	switch owner {
	case "resolver":
		return classResolverControl
	case "enroll":
		return classOwnerEnrollControl
	case "link":
		return classOwnerLinkControl
	}
	t.Fatalf("savepoint statement of an unknown savepoint: %s", kind)
	return ""
}

func rowPathIndex(t *testing.T, rowPath []string, firstPerson int, kind string, occurrence int) int {
	t.Helper()
	if strings.HasPrefix(kind, personBatch) {
		return firstPerson
	}
	if table, bundled := bundledLeafTable(kind); bundled {
		// A bundled statement stands for the first single statement of its
		// table on the row path (#5552, steps 5b and 5c).
		for i := firstPerson; i < len(rowPath); i++ {
			if strings.HasPrefix(statementKind(rowPath[i]), table) {
				return i
			}
		}
		t.Fatalf("the row path issues no statement %q", table)
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

// bundledStatement is one statement the batch command bundles after the
// rows (#5552, steps 5b and 5c): how to recognize it in the batch recording
// and the single statement of its table on the row path.
type bundledStatement struct {
	matches func(kind string) bool
	rowPath string
}

func multiRow(prefix string) func(string) bool {
	return func(kind string) bool { return strings.HasPrefix(kind, prefix) && strings.Contains(kind, "), (") }
}

func bundledPrefix(prefix, contains string) func(string) bool {
	return func(kind string) bool { return strings.HasPrefix(kind, prefix) && strings.Contains(kind, contains) }
}

var bundledStatements = []bundledStatement{
	{multiRow("INSERT INTO users.privacy_consents "), "INSERT INTO users.privacy_consents "},
	{multiRow("INSERT INTO audit.student_consent_changes "), "INSERT INTO audit.student_consent_changes "},
	{bundledPrefix("MERGE INTO users.guardian_profiles ", ""), "INSERT INTO users.guardian_profiles "},
	{multiRow("INSERT INTO users.guardian_phone_numbers ("), "INSERT INTO users.guardian_phone_numbers "},
	{bundledPrefix("WITH linked_guardian AS ", "= ANY("), "WITH linked_guardian AS "},
	{bundledPrefix("INSERT INTO users.student_guardian_pickup_permissions ", " SELECT "), "INSERT INTO users.student_guardian_pickup_permissions "},
	{bundledPrefix("INSERT INTO auth.guardian_student_access ", " SELECT "), "INSERT INTO auth.guardian_student_access "},
	// The enrollment batch of step 7: the profiles before the rows, the care
	// profiles after them.
	{bundledPrefix(enrollmentWrite, " source AS ("), enrollmentWrite + " (SELECT pg_advisory_xact_lock_shared(#, #)), enrolled_person"},
	{multiRow("INSERT INTO users.student_care_profiles ("), "INSERT INTO users.student_care_profiles AS care "},
}

// bundledLeafTable names the row-path statement a bundled statement stands
// for.
func bundledLeafTable(kind string) (string, bool) {
	for _, statement := range bundledStatements {
		if statement.matches(kind) {
			return statement.rowPath, true
		}
	}
	return "", false
}

// replays says whether a failure of the statement makes the batch command
// replay the chunk (table A: P, or N for the bound): a write of the batch
// command that fails with anything but a broken connection. A failed
// savepoint statement ends the upload (500, no retry, no replay). A row that
// forms a segment of its own between the batch segments (the card row,
// phase "after") is written on the row path: its failure is the row path's.
func replays(statement matrixStatement, err error) bool {
	return statement.phase != "after" && !errors.Is(err, driver.ErrBadConn) && !isSavepointControl(statement.kind)
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
	runStudentFaultMatrix(t, matrixFile, false)
}

// TestStudentQuotaSavepointFaultMatrix is the matrix over the file with a
// child quota (#5552 step 7, table review R1 finding 3): School Membership's
// counting write then opens its inner savepoint, in the chunk savepoint for
// a batch row and inside the enrollment savepoint for a row created in place
// (the card row and a row with a group). Only the statements of those quota
// classes are crossed; the rest is the matrix above.
func TestStudentQuotaSavepointFaultMatrix(t *testing.T) {
	t.Parallel()
	runStudentFaultMatrix(t, matrixQuotaFile, true)
}

// matrixQuotaFile is matrixFile in a school with a child quota of 100 and
// O3 in a group, so its child is created in place.
func matrixQuotaFile(t *testing.T, db *bun.DB) []importModels.StudentImportRow {
	t.Helper()
	testpkg.SetTestChildQuota(t, db, 1, 100)
	group := testpkg.CreateTestEducationGroup(t, db, fmt.Sprintf("Kontingent %d", testpkg.Tenant(t)))
	rows := matrixFile(t, db)
	rows[10].GroupName = group.Name
	return rows
}

// quotaClasses are the classes only a school with a child quota records.
var quotaClasses = map[matrixClass]bool{classQuota: true, classQuotaControl: true, classQuotaInEnroll: true, classQuotaInEnrollControl: true}

// runStudentFaultMatrix runs the matrix over the file fixture builds; with
// quota it crosses the quota classes only.
func runStudentFaultMatrix(t *testing.T, fixture func(*testing.T, *bun.DB) []importModels.StudentImportRow, quota bool) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	dsn := testpkg.PackageTestDSN(t)
	var batchRecording, rowRecording, clean matrixUpload
	t.Run("recording", func(t *testing.T) {
		batchRecording = runMatrixUploadOf(t, db, dsn, api.ImportTestOptions{}, nil, fixture)
		require.Equal(t, "200", batchRecording.status)
		require.Equal(t, 2, countKind(batchRecording.statements, personBatch), "both batch segments ran")
		require.Zero(t, batchRecording.replays)
	})
	t.Run("row path recording", func(t *testing.T) {
		rowRecording = runMatrixUploadOf(t, db, dsn, api.ImportTestOptions{RowPathOnly: true}, nil, fixture)
		clean = rowRecording
	})
	require.Equal(t, "200", clean.status)
	requireSameEnd(t, clean, batchRecording)
	// The O rows' guardians went into the bundled statements; the M rows and
	// the card row wrote theirs in place (#5552 step 5c): five new profiles
	// in place (M0 and M4 reuse one) against twelve on the row path.
	require.Equal(t, 5, countKind(batchRecording.statements, "INSERT INTO users.guardian_profiles AS"), "only the in-place guardians are single inserts")
	require.Equal(t, 12, countKind(rowRecording.statements, "INSERT INTO users.guardian_profiles AS"))
	statements := chunkStatements(t, batchRecording.statements, rowRecording.statements)
	classes := map[matrixClass]bool{}
	bundled := map[int]int{}
	bounds := 0
	for _, statement := range statements {
		classes[statement.class] = true
		if _, ok := bundledLeafTable(statement.kind); ok {
			bundled[statement.segment]++
			require.Equal(t, classBounded, statement.class, "a bundled statement runs under the bound (E13): %s", statement.kind)
		}
		if statement.class == classBoundControl {
			bounds++
		}
	}
	require.GreaterOrEqual(t, bundled[1]+bundled[2], len(bundledStatements), "every bundled kind is in the matrix")
	perKind := map[string]int{}
	for _, query := range batchRecording.statements {
		if table, ok := bundledLeafTable(statementKind(query)); ok {
			perKind[table]++
		}
	}
	for _, statement := range bundledStatements {
		require.Equal(t, 2, perKind[statement.rowPath], "each segment bundles %s once", statement.rowPath)
	}
	require.Equal(t, 8, bounds, "two segments × two bounds × set and reset")
	for class := range cleanupPoints {
		require.Equal(t, !quotaClasses[class] || quota, classes[class], "class %s in the recording", class)
	}
	if quota {
		// Cross the quota classes only; every other statement is the
		// matrix over the file without a quota.
		kept := statements[:0]
		for _, statement := range statements {
			if quotaClasses[statement.class] {
				kept = append(kept, statement)
			}
		}
		statements = kept
	}

	// Part 1: every statement × every main error, cleanups succeed.
	var mainOnly sync.Map // "index err" -> matrixUpload of the batch path
	t.Run("main error", func(t *testing.T) {
		for _, statement := range statements {
			for _, err := range matrixFailures {
				t.Run(fmt.Sprintf("%s × %v", statement.label(), err), func(t *testing.T) {
					t.Parallel()
					logStatementOnFailure(t, statement)
					// Each upload runs in a subtest of its own: OwnTenant gives it a
					// tenant of its own.
					var batch, rowPath matrixUpload
					t.Run("batch", func(t *testing.T) {
						batch = runMatrixUploadOf(t, db, dsn, api.ImportTestOptions{}, &matrixFault{at: statement.index, err: err}, fixture)
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
					case classBoundControl:
						// As the row path's first write: 40P01 and 40001 retry the
						// chunk transaction, which then succeeds; anything else ends
						// the upload. Never a replay.
						require.Zero(t, batch.replays, "a bound statement is not the batch command")
						if errors.Is(err, injectedSQLState("40P01")) || errors.Is(err, injectedSQLState("40001")) {
							requireSameEnd(t, clean, batch)
							require.Equal(t, 1, batch.retries)
							return
						}
						requireLost(t, batch)
						return
					}
					t.Run("row path", func(t *testing.T) {
						rowPath = runMatrixUploadOf(t, db, dsn, api.ImportTestOptions{RowPathOnly: true}, &matrixFault{at: statement.row, err: err}, fixture)
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
		class := string(statement.class)
		if _, seen := members[class]; !seen {
			classOrder = append(classOrder, class)
		}
		members[class] = append(members[class], i)
	}
	extremes := classExtremes(statements)
	for _, class := range classOrder {
		indices := members[class]
		first, last := statements[indices[0]], statements[indices[len(indices)-1]]
		require.Equal(t, extremes[first.class], [2]int{first.index, last.index}, "the whole cross takes the first and the last statement of %s by position", class)
	}
	t.Run("cleanup", func(t *testing.T) {
		for _, class := range classOrder {
			indices := members[class]
			for position, at := range indices {
				statement := statements[at]
				mains, cleanups := minimumCleanupFailures, minimumCleanupErrors
				if position == 0 || position == len(indices)-1 || statement.class == classBoundControl {
					// First and last of each class, and every set and reset of
					// every bound, get the whole cross.
					mains, cleanups = matrixFailures, matrixFailures
				}
				if quota {
					// The quota classes take every main error with the two
					// cleanup errors that stand for the rest (a failed
					// cleanup ends the upload whatever its error), so the
					// package stays inside CI's ten minutes.
					cleanups = minimumCleanupErrors
				}
				for _, err := range mains {
					for _, point := range cleanupPoints[statement.class] {
						for _, err2 := range cleanups {
							cases++
							t.Run(fmt.Sprintf("%s × %v, cleanup %d × %v", statement.label(), err, point, err2), func(t *testing.T) {
								t.Parallel()
								runCleanupCase(t, db, dsn, clean, &mainOnly, statement, err, point, err2, fixture)
							})
						}
					}
				}
			}
		}
	})
	t.Logf("%d statements in %d classes; %d main-error cases, %d cleanup cases", len(statements), len(classOrder), len(statements)*len(matrixFailures), cases)
}

// minimumCleanupErrors are the cleanup errors every statement that does not
// stand first or last for its class meets at each cleanup point: a failed
// cleanup ends the upload whatever its error (E19), except E13′, which rests
// on the main error; a retryable SQLSTATE and a broken connection stand for
// the rest (#6514, to keep the package inside CI's ten minutes).
var minimumCleanupErrors = []error{injectedSQLState("40P01"), driver.ErrBadConn}

// minimumCleanupFailures are the main errors every statement that does not
// stand first or last for its class is crossed with.
var minimumCleanupFailures = []error{
	injectedSQLState("40P01"), injectedSQLState("57014"), errors.New("injected failure without SQLSTATE"),
}

// runCleanupCase is one cell of part 2 (tables A and B of the PR).
func runCleanupCase(t *testing.T, db *bun.DB, dsn string, clean matrixUpload, mainOnly *sync.Map, statement matrixStatement, err error, point int, err2 error, fixture func(*testing.T, *bun.DB) []importModels.StudentImportRow) {
	t.Helper()
	logStatementOnFailure(t, statement)
	got := runMatrixUploadOf(t, db, dsn, api.ImportTestOptions{}, &matrixFault{at: statement.index, err: err, cleanup: point, err2: err2}, fixture)
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
	// The quota is reached at row k of six: places and existing children
	// fixed per case, so the refused row and its details are known in
	// advance (#5552 step 7: the profiles are created before the rows, the
	// memberships still per row).
	for _, c := range []struct {
		name             string
		places, existing int
		refused          int
	}{
		{"first row", 1, 1, 1},
		{"row 4", 3, 0, 4},
		{"last row", 5, 0, 6},
	} {
		t.Run(c.name, func(t *testing.T) {
			run := func(t *testing.T, options api.ImportTestOptions) (string, studentOutcome, error) {
				testpkg.OwnTenant(t)
				testpkg.SetTestChildQuota(t, db, 1, c.places)
				for i := range c.existing {
					testpkg.CreateTestStudent(t, db, fmt.Sprintf("Schon%d", i), "Da", "4A")
				}
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
			require.Contains(t, fmt.Sprint(rowDetails.ErrorDetails()), fmt.Sprintf("%d", c.places), "the details name the booked places")
			require.Equal(t, []string{fmt.Sprintf("%d error", c.refused+1)}, quotaRowErrors(batch), "the refused row")
		})
	}
}

// quotaRowErrors are the row number and severity of an outcome's blocking
// row errors.
func quotaRowErrors(outcome studentOutcome) []string {
	var rows []string
	for _, e := range outcome.RowErrors {
		if parts := strings.SplitN(e, " ", 3); len(parts) >= 2 && parts[1] == "error" {
			rows = append(rows, parts[0]+" "+parts[1])
		}
	}
	return rows
}
