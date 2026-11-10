package integration

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/api"
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
