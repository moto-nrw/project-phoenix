package integration

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/api"
	importModels "github.com/moto-nrw/project-phoenix/modules/dataimport"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	testpkg "github.com/moto-nrw/project-phoenix/test"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
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
// batch path as on the row path.
func TestStudentEnrollmentBatchTakesEachStatusWhereTheRowPathDoes(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	day := calendar.NewDate(2026, time.November, 10)
	for _, run := range []struct {
		name    string
		options api.ImportTestOptions
	}{
		{"batch path", api.ImportTestOptions{}},
		{"row path", api.ImportTestOptions{RowPathOnly: true}},
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
func openedHealth(t *testing.T, db *bun.DB, persons peopledirectory.Capability) map[string]string {
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
