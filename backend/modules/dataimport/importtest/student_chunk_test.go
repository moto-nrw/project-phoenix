package integration

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/api"
	importModels "github.com/moto-nrw/project-phoenix/modules/dataimport"
	"github.com/moto-nrw/project-phoenix/modules/dataimport/ports"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// The student create on the chunk executor (#5552, ADR 0067 addendum of
// 2026-11-08, step 5): the persons of a segment are created in one statement,
// everything after the person still row by row. Every test compares with the
// forced row path (api.ImportTestOptions.RowPathOnly) on a tenant of its own.

type studentUpload struct {
	result *importModels.ImportResult[importModels.StudentImportRow]
	err    error
}

// uploadStudents runs a create upload in the unit of work of pool, the pool
// the module was composed over.
func uploadStudents(t *testing.T, module api.ImportTestModule, pool *bun.DB, actor importer, rows []importModels.StudentImportRow, filename string) <-chan studentUpload {
	done := make(chan studentUpload, 1)
	ctx := tenant.WithUnitOfWork(testpkg.Ctx(t), testpkg.TenantRuntime(t, pool))
	go func() {
		result, err := module.Import.ImportBatches(ctx, importModels.ImportRequest[importModels.StudentImportRow]{
			Rows: rows, Mode: importModels.ImportModeCreate, UserID: actor.staffID, SkipInvalidRows: true,
		}, importModels.BatchAudit{EntityType: "student", Filename: filename, AccountID: actor.accountID})
		done <- studentUpload{result, err}
	}()
	return done
}

func awaitStudents(t *testing.T, done <-chan studentUpload) studentUpload {
	t.Helper()
	select {
	case upload := <-done:
		return upload
	case <-time.After(60 * time.Second):
		t.Fatal("the import did not answer")
		return studentUpload{}
	}
}

// studentOutcome is an upload's result and the owner rows of its tenant
// without IDs, foreign keys and timestamps (a timestamp or actor column only
// says whether it is set), for comparison across tenants.
type studentOutcome struct {
	Created, Updated, Errors, Warnings int
	RowErrors                          []string
	Rows                               map[string][]string
}

// studentOwnerTables are the tables a student create writes, each with the
// child it belongs to, so rows stay attributable without their IDs.
var studentOwnerTables = map[string]string{
	"users.persons":                             `SELECT x, p.first_name || ' ' || p.last_name FROM users.persons x JOIN users.student_profiles s ON s.person_id = x.id JOIN users.persons p ON p.id = x.id`,
	"users.student_profiles":                    `SELECT x, p.first_name || ' ' || p.last_name FROM users.student_profiles x JOIN users.persons p ON p.id = x.person_id`,
	"users.student_school_memberships":          `SELECT x, p.first_name || ' ' || p.last_name FROM users.student_school_memberships x JOIN users.student_profiles s ON s.id = x.student_profile_id JOIN users.persons p ON p.id = s.person_id`,
	"users.student_care_profiles":               `SELECT x, p.first_name || ' ' || p.last_name FROM users.student_care_profiles x JOIN users.student_school_memberships m ON m.id = x.membership_id JOIN users.student_profiles s ON s.id = m.student_profile_id JOIN users.persons p ON p.id = s.person_id`,
	"users.guardian_profiles":                   `SELECT x, x.email FROM users.guardian_profiles x`,
	"users.guardian_phone_numbers":              `SELECT x, g.email FROM users.guardian_phone_numbers x JOIN users.guardian_profiles g ON g.id = x.guardian_profile_id`,
	"users.student_guardian_relationships":      `SELECT x, p.first_name || ' ' || p.last_name || ' > ' || g.email FROM users.student_guardian_relationships x JOIN users.guardian_profiles g ON g.id = x.guardian_profile_id JOIN users.student_profiles s ON s.id = x.student_id JOIN users.persons p ON p.id = s.person_id`,
	"users.student_guardian_pickup_permissions": `SELECT x, p.first_name || ' ' || p.last_name || ' > ' || g.email FROM users.student_guardian_pickup_permissions x JOIN users.student_guardian_relationships r ON r.id = x.relationship_id JOIN users.guardian_profiles g ON g.id = r.guardian_profile_id JOIN users.student_profiles s ON s.id = r.student_id JOIN users.persons p ON p.id = s.person_id`,
	"auth.guardian_student_access":              `SELECT x, p.first_name || ' ' || p.last_name || ' > ' || g.email FROM auth.guardian_student_access x JOIN users.student_guardian_relationships r ON r.id = x.relationship_id JOIN users.guardian_profiles g ON g.id = r.guardian_profile_id JOIN users.student_profiles s ON s.id = r.student_id JOIN users.persons p ON p.id = s.person_id`,
	"users.privacy_consents":                    `SELECT x, p.first_name || ' ' || p.last_name FROM users.privacy_consents x JOIN users.student_profiles s ON s.id = x.student_id JOIN users.persons p ON p.id = s.person_id`,
	"schedule.student_pickup_schedules":         `SELECT x, p.first_name || ' ' || p.last_name FROM schedule.student_pickup_schedules x JOIN users.student_profiles s ON s.id = x.student_id JOIN users.persons p ON p.id = s.person_id`,
	"schedule.student_arrival_schedules":        `SELECT x, p.first_name || ' ' || p.last_name FROM schedule.student_arrival_schedules x JOIN users.student_profiles s ON s.id = x.student_id JOIN users.persons p ON p.id = s.person_id`,
	"audit.student_consent_changes":             `SELECT x, p.first_name || ' ' || p.last_name FROM audit.student_consent_changes x JOIN users.student_profiles s ON s.id = x.student_id JOIN users.persons p ON p.id = s.person_id`,
}

func studentOutcomeOf(t *testing.T, db *bun.DB, result *importModels.ImportResult[importModels.StudentImportRow]) studentOutcome {
	t.Helper()
	outcome := studentOutcome{Rows: map[string][]string{}}
	if result != nil {
		outcome.Created, outcome.Updated, outcome.Errors, outcome.Warnings = result.CreatedCount, result.UpdatedCount, result.ErrorCount, result.WarningCount
		for _, row := range result.Errors {
			for _, e := range row.Errors {
				outcome.RowErrors = append(outcome.RowErrors, fmt.Sprintf("%d %s %s %s %s", row.RowNumber, e.Severity, e.Field, e.Code, e.Message))
			}
		}
	}
	for table, query := range studentOwnerTables {
		var rows []string
		// Drop identities, keys and the tenant; a timestamp or actor only says
		// whether it is set. The owner label keeps the row attributable.
		require.NoError(t, db.NewRaw(`SELECT owner || ' ' || (
				SELECT jsonb_object_agg(key, CASE WHEN key LIKE '%\_at' OR key LIKE '%\_since' OR key LIKE '%\_by' THEN to_jsonb(value <> 'null'::jsonb) ELSE value END)
				FROM jsonb_each(to_jsonb(x))
				WHERE key <> 'id' AND key NOT LIKE '%\_id' AND key NOT IN ('created_at', 'updated_at')
			)::text FROM (`+query+` WHERE x.tenant_id = ?) AS t(x, owner) ORDER BY 1`, testpkg.Tenant(t)).Scan(context.Background(), &rows), table)
		outcome.Rows[table] = rows
	}
	sort.Strings(outcome.RowErrors)
	return outcome
}

// mixedStudentFile has eleven rows. Rows 1 to 8 form one batch segment: a
// seeded namesake (already exists, with a capped retention warning), a
// card, a shared guardian, consent dates, arrival and pickup times. Rows 9 to
// 11 spell earlier children differently (decomposed accents, capitals, the
// Kelvin sign), so the owners' NFC- and case-folded match finds them: each
// starts a new segment and already exists.
func mixedStudentFile(t *testing.T, db *bun.DB) []importModels.StudentImportRow {
	card := testpkg.CreateTestRFIDCard(t, db, "CHUNK")
	guardian := func(name, email, phone string) []importModels.GuardianImportData {
		return []importModels.GuardianImportData{{FirstName: "Eltern", LastName: name, Email: email, RelationshipType: "Mutter",
			PhoneNumbers: []importModels.PhoneImportData{{PhoneNumber: phone, PhoneType: "mobile"}}}}
	}
	return []importModels.StudentImportRow{
		{FirstName: "José", LastName: "Gómez", SchoolClass: "4A", Birthday: "2017-04-05", DataRetentionDays: 30, PrivacyAccepted: true,
			Guardians: guardian("Gómez", "gomez@example.test", "0151 7000001"), AGBAcceptedAt: "2026-09-01", DataProcessingAcceptedAt: "2026-09-02"},
		{FirstName: "Schon", LastName: "Kind", SchoolClass: "4A", DataRetentionDays: 40},
		{FirstName: "Lena", LastName: "Neu", SchoolClass: "4A", DataRetentionDays: 40, TagID: card.ID,
			PickupSchedules: []importModels.PickupScheduleImportData{{Weekday: 2, PickupTime: "15:30"}}},
		{FirstName: "Mia", LastName: "Geschwister", SchoolClass: "4B", DataRetentionDays: 30, Guardians: guardian("Geschwister", "familie@example.test", "0151 7000002"),
			ArrivalSchedules: []importModels.ArrivalScheduleImportData{{Weekday: 1, ExpectedArrival: "07:45"}}},
		{FirstName: "Ben", LastName: "Geschwister", SchoolClass: "4B", DataRetentionDays: 30, Guardians: guardian("Geschwister", "familie@example.test", "0151 7000003"),
			DepartureDays: map[string]string{"mon": "bus", "tue": "pickup"}},
		{FirstName: "Ella", LastName: "Ohne", SchoolClass: "4B", DataRetentionDays: 30, PrivacyAccepted: true},
		{FirstName: "Kai", LastName: "Kelvin", SchoolClass: "4B", DataRetentionDays: 30},
		{FirstName: "Tom", LastName: "Acht", SchoolClass: "4B", DataRetentionDays: 30, Birthday: "2016-12-24"},
		{FirstName: "Jose\u0301", LastName: "Go\u0301mez", SchoolClass: "4A", DataRetentionDays: 30},
		{FirstName: "JOSÉ", LastName: "GÓMEZ", SchoolClass: "4a", DataRetentionDays: 30},
		{FirstName: "\u212Aai", LastName: "Kelvin", SchoolClass: "4B", DataRetentionDays: 30},
	}
}

// runMixedStudents imports the mixed file on a tenant of its own with a
// seeded namesake and returns the outcome and the upload's observation.
func runMixedStudents(t *testing.T, db *bun.DB, options api.ImportTestOptions, edit func([]importModels.StudentImportRow)) (studentOutcome, importModels.ImportObservation, error) {
	t.Helper()
	testpkg.OwnTenant(t)
	module, err := api.NewImportTestModuleWithOptions(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db), options)
	require.NoError(t, err)
	actor := newImporter(t, db)
	testpkg.CreateTestStudent(t, db, "Schon", "Kind", "4A")
	rows := mixedStudentFile(t, db)
	if edit != nil {
		edit(rows)
	}
	upload := awaitStudents(t, uploadStudents(t, module, db, actor, rows, "mixed.csv"))
	require.Len(t, *module.Observations, 1)
	return studentOutcomeOf(t, db, upload.result), (*module.Observations)[0], upload.err
}

func commandCount(observation importModels.ImportObservation, operation string) int {
	count := 0
	for _, command := range observation.Commands {
		if command.Operation == operation {
			count++
		}
	}
	return count
}

// TestStudentChunkMatchesTheRowPath (Äquivalenz, Match-Abhängigkeiten): the
// batch path returns and persists exactly what the row path does, including
// three spellings of one child in one file and a seeded namesake.
func TestStudentChunkMatchesTheRowPath(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	var batch, rowPath studentOutcome
	t.Run("batch path", func(t *testing.T) {
		var observation importModels.ImportObservation
		var err error
		batch, observation, err = runMixedStudents(t, db, api.ImportTestOptions{}, nil)
		require.NoError(t, err)
		require.Zero(t, observation.BatchFallbacks)
		require.Positive(t, commandCount(observation, "create_persons"), "the persons came from the batch command")
	})
	t.Run("row path", func(t *testing.T) {
		var observation importModels.ImportObservation
		var err error
		rowPath, observation, err = runMixedStudents(t, db, api.ImportTestOptions{RowPathOnly: true}, nil)
		require.NoError(t, err)
		require.Zero(t, commandCount(observation, "create_persons"))
	})
	require.Equal(t, rowPath, batch)
	require.Equal(t, 7, batch.Created)
	require.Equal(t, 4, batch.Errors, "the seeded namesake and the three later spellings already exist")
	require.Len(t, batch.Rows["users.persons"], 8, "seven new children and the seeded one")
}

// TestStudentChunkRollbackRestoresTheResultState (Invariante Chunk-Rollback):
// row 2 carries a validation warning and already exists. Row 1 fails in the
// batch and in the replay, so the replay never reaches row 2: it keeps only
// its warning, as on the row path. With a batch-only failure the replay
// succeeds and equals the row path too.
func TestStudentChunkRollbackRestoresTheResultState(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	installRefusedMarkers(t, db)
	refuseRowOne := func(rows []importModels.StudentImportRow) { rows[0].FirstName = "Refused" }
	outcomes := map[string]studentOutcome{}
	for _, run := range []struct {
		name    string
		options api.ImportTestOptions
		edit    func([]importModels.StudentImportRow)
		fails   bool
	}{
		{"batch, row 1 refused", api.ImportTestOptions{}, refuseRowOne, true},
		{"row path, row 1 refused", api.ImportTestOptions{RowPathOnly: true}, refuseRowOne, true},
		{"batch fails once", api.ImportTestOptions{WrapPersonBatches: failPersonBatches(1, nil)}, nil, false},
		{"row path", api.ImportTestOptions{RowPathOnly: true}, nil, false},
	} {
		t.Run(run.name, func(t *testing.T) {
			outcome, observation, err := runMixedStudents(t, db, run.options, run.edit)
			require.Equal(t, run.fails, err != nil, "%v", err)
			if !run.options.RowPathOnly {
				require.Equal(t, 1, observation.BatchFallbacks)
			}
			outcomes[run.name] = outcome
		})
	}
	require.Equal(t, outcomes["row path, row 1 refused"], outcomes["batch, row 1 refused"])
	require.Equal(t, outcomes["row path"], outcomes["batch fails once"])
	refused := outcomes["batch, row 1 refused"]
	require.Zero(t, refused.Created)
	require.Contains(t, refused.RowErrors, "3 warning data_retention_days value_capped Aufbewahrungsdauer von 40 Tagen überschreitet Maximum. Wird auf 31 Tage gesetzt.")
	for _, line := range refused.RowErrors {
		require.NotContains(t, line, "already_exists", "the replay stops at row 1 and never reaches row 2")
	}
}

// failingPersonBatches fails the person batch command a number of times;
// before runs first, on the failing call.
type failingPersonBatches struct {
	ports.PersonBatches
	failures *atomic.Int32
	before   func()
}

func (f failingPersonBatches) CreatePersons(ctx context.Context, inputs []ports.CreatePerson) ([]ports.Person, error) {
	if f.failures.Add(-1) >= 0 {
		if f.before != nil {
			f.before()
		}
		return nil, errors.New("probe person batch failure")
	}
	return f.PersonBatches.CreatePersons(ctx, inputs)
}

func failPersonBatches(times int32, before func()) func(ports.PersonBatches) ports.PersonBatches {
	return func(batches ports.PersonBatches) ports.PersonBatches {
		failures := &atomic.Int32{}
		failures.Store(times)
		return failingPersonBatches{PersonBatches: batches, failures: failures, before: before}
	}
}

// TestStudentChunkReplaySeesAParallelChange (Frisches Vorladen beim Replay):
// ResolveChunk reads the names of an ASCII file in one go and finds no
// "Runtime0-5 Evidence". A second connection then creates her and commits,
// and the batch fails. The replay finds her and reports row 7 as already
// existing: the outcome of a sequential run that starts after the change on
// the same initial state.
func TestStudentChunkReplaySeesAParallelChange(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	run := func(t *testing.T, options func(t *testing.T) api.ImportTestOptions, seed bool) (studentOutcome, importModels.ImportObservation) {
		testpkg.OwnTenant(t)
		module, err := api.NewImportTestModuleWithOptions(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db), options(t))
		require.NoError(t, err)
		actor := newImporter(t, db)
		if seed {
			testpkg.CreateTestStudent(t, db, "Runtime0-5", "Evidence", "4A")
		}
		upload := awaitStudents(t, uploadStudents(t, module, db, actor, studentBatch(0, 8), "parallel.csv"))
		require.NoError(t, upload.err)
		return studentOutcomeOf(t, db, upload.result), (*module.Observations)[0]
	}
	var replayed, sequential studentOutcome
	t.Run("replay after the change", func(t *testing.T) {
		var observation importModels.ImportObservation
		replayed, observation = run(t, func(t *testing.T) api.ImportTestOptions {
			return api.ImportTestOptions{WrapPersonBatches: failPersonBatches(1, func() {
				testpkg.CreateTestStudent(t, db, "Runtime0-5", "Evidence", "4A")
			})}
		}, false)
		require.Equal(t, 1, observation.BatchFallbacks)
	})
	t.Run("sequential run after the change", func(t *testing.T) {
		sequential, _ = run(t, func(*testing.T) api.ImportTestOptions { return api.ImportTestOptions{RowPathOnly: true} }, true)
	})
	require.Equal(t, sequential, replayed)
	require.Equal(t, 7, replayed.Created)
	require.Contains(t, strings.Join(replayed.RowErrors, "\n"), "7 error duplicate already_exists")
}

// TestStudentChunkKeysTrimLikeTheImport: the import trims every name cell
// with strings.TrimSpace before it searches and creates, so "Anna\t" and
// "Anna" are one child. Their keys must split them into two segments: the
// second row already exists, as on the row path.
func TestStudentChunkKeysTrimLikeTheImport(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	rows := namedStudentBatch("Leerraum", 7)
	rows[0].FirstName, rows[6].FirstName = "Anna\t", "Anna"
	run := func(t *testing.T, options api.ImportTestOptions) (studentOutcome, importModels.ImportObservation) {
		testpkg.OwnTenant(t)
		module, err := api.NewImportTestModuleWithOptions(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db), options)
		require.NoError(t, err)
		actor := newImporter(t, db)
		upload := awaitStudents(t, uploadStudents(t, module, db, actor, rows, "whitespace.csv"))
		require.NoError(t, upload.err)
		return studentOutcomeOf(t, db, upload.result), (*module.Observations)[0]
	}
	var batch, rowPath studentOutcome
	t.Run("batch path", func(t *testing.T) {
		var observation importModels.ImportObservation
		batch, observation = run(t, api.ImportTestOptions{})
		require.Positive(t, commandCount(observation, "create_persons"))
		require.Zero(t, observation.BatchFallbacks)
	})
	t.Run("row path", func(t *testing.T) { rowPath, _ = run(t, api.ImportTestOptions{RowPathOnly: true}) })
	require.Equal(t, rowPath, batch)
	require.Equal(t, 6, batch.Created)
	require.Contains(t, strings.Join(batch.RowErrors, "\n"), "8 error duplicate already_exists")
}
