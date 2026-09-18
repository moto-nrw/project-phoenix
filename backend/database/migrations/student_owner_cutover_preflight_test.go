package migrations

import (
	"context"
	"errors"
	"fmt"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func TestStudentOwnerCutoverPreconditionPassesOnReconciledData(t *testing.T) {
	t.Parallel()
	db := setupStudentStorageBeforeCutover(t)
	studentOwnerCutoverFixture(t, db, 3)
	require.NoError(t, studentOwnerCutoverPrecondition(t.Context(), db))
}

func TestStudentOwnerCutoverPreconditionReportsUnreconciledGuardianValue(t *testing.T) {
	t.Parallel()
	db := setupStudentStorageBeforeCutover(t)
	tenantID, ids := studentOwnerCutoverFixture(t, db, 2)
	_, err := db.NewRaw(`UPDATE users.students SET guardian_email = 'nobody@example.org' WHERE tenant_id = ? AND id = ?`,
		tenantID, ids[0]).Exec(t.Context())
	require.NoError(t, err)

	err = studentOwnerCutoverPrecondition(t.Context(), db)
	require.ErrorContains(t, err, fmt.Sprintf("tenant %d: 1 unreconciled guardian values", tenantID))
	require.ErrorContains(t, err, "docs/operations/student-owner-storage-backfill.md")
}

// The environment this has to protect is the one that is several releases
// behind: it reaches the backfill and the cutover in a single `migrate` run, so
// at preflight time no checkpoint exists to read a verdict from. The check reads
// the source tables instead, which is what makes it answer there at all.
func TestStudentOwnerCutoverPreconditionReportsBeforeTheBackfillHasEverRun(t *testing.T) {
	t.Parallel()
	db := setupStudentStorageBeforeCutover(t)
	tenantID := testpkg.Tenant(t)
	ids := studentOwnerFixture(t, db, tenantID, 2)
	_, err := db.NewRaw(`UPDATE users.students SET guardian_phone = '0170 1234567' WHERE tenant_id = ? AND id = ?`,
		tenantID, ids[0]).Exec(t.Context())
	require.NoError(t, err)

	var checkpoints int
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM platform.storage_backfill_checkpoints WHERE backfill = ?`,
		StudentOwnerBackfillName).Scan(t.Context(), &checkpoints))
	require.Zero(t, checkpoints, "this case exists precisely because no checkpoint has been written yet")

	require.ErrorContains(t, studentOwnerCutoverPrecondition(t.Context(), db),
		fmt.Sprintf("tenant %d: 1 unreconciled guardian values", tenantID))
}

func TestStudentOwnerCutoverPreconditionReportsAbsenceFlagWithoutStatusDay(t *testing.T) {
	t.Parallel()
	db := setupStudentStorageBeforeCutover(t)
	tenantID, ids := studentOwnerCutoverFixture(t, db, 2)
	_, err := db.NewRaw(`UPDATE users.students SET sick = true WHERE tenant_id = ? AND id = ?`,
		tenantID, ids[0]).Exec(t.Context())
	require.NoError(t, err)

	require.ErrorContains(t, studentOwnerCutoverPrecondition(t.Context(), db),
		fmt.Sprintf("tenant %d: 1 absence flags without a status day", tenantID))
}

// After the switch users.students is the compatibility view and the legacy
// columns live in the archive. The entry can still be pending, waiting only for
// its foreign-key validation to resume, and that needs no data correction.
func TestStudentOwnerCutoverPreconditionPassesAfterTheSwitch(t *testing.T) {
	t.Parallel()
	db := setupStudentStorageBeforeCutover(t)
	tenantID, ids := studentOwnerCutoverFixture(t, db, 2)
	require.NoError(t, finalizeStudentOwnerStorage(t.Context(), db, installStudentOwnerCompatibility))
	_, err := db.NewRaw(`UPDATE users.students_legacy SET guardian_email = 'nobody@example.org' WHERE tenant_id = ? AND id = ?`,
		tenantID, ids[0]).Exec(t.Context())
	require.NoError(t, err)

	require.NoError(t, studentOwnerCutoverPrecondition(t.Context(), db))
}

// The precondition must never be the thing that changes the data it inspects.
func TestStudentOwnerCutoverPreconditionWritesNothing(t *testing.T) {
	t.Parallel()
	db := setupStudentStorageBeforeCutover(t)
	tenantID, _ := studentOwnerCutoverFixture(t, db, 3)
	before := studentRowsJSON(t, db, tenantID)
	require.NoError(t, studentOwnerCutoverPrecondition(t.Context(), db))
	require.JSONEq(t, before, studentRowsJSON(t, db, tenantID))
}

// The preflight runs before every pending migration, so an environment below
// the migration that creates one of these relations must get "no verdict yet",
// not a "relation does not exist" that aborts the release for a reason nobody
// can correct. The rename is rolled back with the transaction.
func TestStudentOwnerCutoverPreconditionYieldsBelowTheSchemaFloor(t *testing.T) {
	t.Parallel()
	db := setupStudentStorageBeforeCutover(t)
	studentOwnerCutoverFixture(t, db, 1)
	require.ErrorIs(t, db.RunInTx(t.Context(), nil, func(ctx context.Context, tx bun.Tx) error {
		ready, err := studentOwnerPreflightReady(ctx, tx)
		require.NoError(t, err)
		require.True(t, ready, "the fixture is above the floor")

		_, err = tx.ExecContext(ctx, `ALTER TABLE active.student_status_days RENAME TO student_status_days_absent`)
		require.NoError(t, err)
		ready, err = studentOwnerPreflightReady(ctx, tx)
		require.NoError(t, err)
		require.False(t, ready, "a missing relation is not a data verdict")
		return errRollbackPreflightFixture
	}), errRollbackPreflightFixture, "the rename is rolled back with the transaction")
}

// The cutover also refuses a school with students the backfill has never
// completed a pass over. Once any school has a checkpoint the backfill has run,
// so a school without one is drift a release cannot recover from.
func TestStudentOwnerCutoverPreconditionReportsSchoolTheBackfillNeverVisited(t *testing.T) {
	t.Parallel()
	db := setupStudentStorageBeforeCutover(t)
	studentOwnerCutoverFixture(t, db, 1)

	// A school onboarded after the last pass: students, no checkpoint.
	newcomer := testpkg.Tenant(t)
	studentOwnerFixture(t, db, newcomer, 2)
	_, err := db.NewRaw(`DELETE FROM platform.storage_backfill_checkpoints WHERE backfill = ? AND tenant_id = ?`,
		StudentOwnerBackfillName, newcomer).Exec(t.Context())
	require.NoError(t, err)

	require.ErrorContains(t, studentOwnerCutoverPrecondition(t.Context(), db),
		fmt.Sprintf("tenant %d: students without a completed backfill pass", newcomer))
}

// Production reaches the backfill and the cutover in one migrate run. Asking
// about checkpoints there would fail a release that is about to write them.
func TestStudentOwnerCutoverPreconditionIgnoresCheckpointsBeforeAnyBackfillRan(t *testing.T) {
	t.Parallel()
	db := setupStudentStorageBeforeCutover(t)
	tenantID := testpkg.Tenant(t)
	studentOwnerFixture(t, db, tenantID, 2)
	_, err := db.NewRaw(`DELETE FROM platform.storage_backfill_checkpoints WHERE backfill = ?`,
		StudentOwnerBackfillName).Exec(t.Context())
	require.NoError(t, err)

	require.NoError(t, studentOwnerCutoverPrecondition(t.Context(), db),
		"no checkpoint anywhere means the backfill is pending in this same run")
}

var errRollbackPreflightFixture = errors.New("rollback preflight fixture")

func TestDescribeStudentOwnerUnreconciledNamesBothVerdicts(t *testing.T) {
	t.Parallel()
	require.Equal(t,
		"student owner cutover would refuse: tenant 4: 2 unreconciled guardian values; "+
			"tenant 5: 1 absence flags without a status day; "+
			"tenant 7: 3 unreconciled guardian values, 4 absence flags without a status day",
		describeStudentOwnerUnreconciled([]studentOwnerUnreconciled{
			{TenantID: 4, Guardian: 2},
			{TenantID: 5, CareState: 1},
			{TenantID: 7, Guardian: 3, CareState: 4},
		}))
}

// A nil database is a wiring mistake, not a passing precondition.
func TestStudentOwnerCutoverPreconditionRequiresADatabase(t *testing.T) {
	t.Parallel()
	var db *bun.DB
	require.ErrorContains(t, studentOwnerCutoverPrecondition(context.Background(), db), "database is required")
}
