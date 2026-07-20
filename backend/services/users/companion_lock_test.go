package users_test

// The lock protocol behind these tests: every companion writer takes student
// rows in ascending id order, which is what keeps two people editing the same
// Laufgemeinschaft from deadlocking. The order can only be honored for ids
// known before the first lock — the far ends are read from the edge table, and
// a concurrent commit can add one afterwards. Such a LATE id below the ones
// already held is the one acquisition that would invert the order, so it is
// taken without waiting and refused as retriable instead of deadlocking.
//
// These tests hold a real row lock from a second transaction, because that is
// the only way to observe the difference between "waits" and "refuses" — a mock
// would just replay whichever behavior it was told to.

import (
	"context"
	"testing"
	"time"

	usersService "github.com/moto-nrw/project-phoenix/services/users"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// holdStudentRowLock opens its own transaction, locks the student's row and
// keeps it locked until the test ends — the stand-in for "someone else is
// editing this child right now".
func holdStudentRowLock(t *testing.T, db *bun.DB, studentID int64) {
	t.Helper()

	tx, err := db.BeginTx(context.Background(), nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = tx.Rollback() })

	var locked int64
	err = tx.NewRaw(
		`SELECT id FROM users.students WHERE id = ? FOR UPDATE`, studentID,
	).Scan(context.Background(), &locked)
	require.NoError(t, err)
	require.Equal(t, studentID, locked)
}

// TestStudentService_LockStudentsForUpdateBelow_RefusesDownwardLock pins the
// escape hatch: an id BELOW what this transaction already holds must come back
// as the retriable ErrCompanionLockBusy instead of waiting. Waiting is what
// PostgreSQL would resolve with a deadlock abort — a 500 on an edit that is
// perfectly legal and succeeds on the next attempt.
func TestStudentService_LockStudentsForUpdateBelow_RefusesDownwardLock(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	ctx := testpkg.TenantContext(1)
	service := newCompanionTestService(db)

	busy := testpkg.CreateTestStudent(t, db, "LockBusyLow", "Companion", "1a")
	defer testpkg.CleanupActivityFixtures(t, db, busy.ID)

	holdStudentRowLock(t, db, busy.ID)

	start := time.Now()
	err := service.LockStudentsForUpdateBelow(ctx, []int64{busy.ID}, busy.ID+1)

	assert.ErrorIs(t, err, usersService.ErrCompanionLockBusy)
	// It must REFUSE, not wait: a downward lock that blocks is the deadlock the
	// whole protocol exists to avoid, so the refusal has to be immediate.
	assert.Less(t, time.Since(start), 2*time.Second)
}

// TestStudentService_LockStudentsForUpdateBelow_WaitsAtOrAboveBound pins the
// other half: an id that still follows the ascending order may wait for the
// other writer, because waiting in the agreed direction cannot deadlock. The
// context deadline is what ends this call — proof that it queued rather than
// bailing out with ErrCompanionLockBusy.
func TestStudentService_LockStudentsForUpdateBelow_WaitsAtOrAboveBound(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := newCompanionTestService(db)

	busy := testpkg.CreateTestStudent(t, db, "LockBusyHigh", "Companion", "1a")
	defer testpkg.CleanupActivityFixtures(t, db, busy.ID)

	holdStudentRowLock(t, db, busy.ID)

	ctx, cancel := context.WithTimeout(testpkg.TenantContext(1), 750*time.Millisecond)
	defer cancel()

	err := service.LockStudentsForUpdateBelow(ctx, []int64{busy.ID}, busy.ID)

	require.Error(t, err)
	assert.NotErrorIs(t, err, usersService.ErrCompanionLockBusy)
}

// TestStudentService_LockStudentsForUpdate_TakesFreeRows guards the ordinary
// path the refusal must not regress: with nobody holding the rows, both the
// plain entry point and the bounded one simply take their locks.
func TestStudentService_LockStudentsForUpdate_TakesFreeRows(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	ctx := testpkg.TenantContext(1)
	service := newCompanionTestService(db)

	first := testpkg.CreateTestStudent(t, db, "LockFreeOne", "Companion", "1a")
	second := testpkg.CreateTestStudent(t, db, "LockFreeTwo", "Companion", "1a")
	defer testpkg.CleanupActivityFixtures(t, db, first.ID, second.ID)

	require.NoError(t, service.LockStudentsForUpdate(ctx, []int64{second.ID, first.ID}))
	// A free row below the bound goes through the NOWAIT path and must still
	// succeed — the refusal is about contention, not about the direction alone.
	require.NoError(t, service.LockStudentsForUpdateBelow(ctx, []int64{first.ID}, second.ID))

	// An id that no longer exists is skipped, not an error: a companion can be
	// deleted between the graph read and the lock pass, and the validation that
	// follows distinguishes the two "gone" cases on its own.
	require.NoError(t, service.LockStudentsForUpdateBelow(ctx, []int64{second.ID + 9_000_000}, second.ID))
}
