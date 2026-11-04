package compose

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// studentClassGateKey is School Membership's class-writes gate key.
const studentClassGateKey = int32(0x636c6173)

// TestStudentRowLockWaitsForTheClassGateFirst pins the order the single
// locking statement keeps (#5278): it takes the shared class-writes gate
// before the student row. While a grade transition holds the gate
// exclusively, the waiting lock holds no row, so the transition can still
// lock the child.
func TestStudentRowLockWaitsForTheClassGateFirst(t *testing.T) {
	t.Parallel()
	fixtures := testpkg.SetupTestDB(t)
	// The gate holder, the waiting lock and the probe hold connections at
	// once, so they run on a pool of the test's own (#5269).
	db := testpkg.SetupClosableTestDB(t)
	t.Cleanup(func() { assert.NoError(t, db.Close()) })
	ctx := testpkg.Ctx(t)
	gate := func(ctx context.Context) (*bun.SelectQuery, error) {
		return db.NewSelect().ColumnExpr("pg_advisory_xact_lock_shared(?, ?)", studentClassGateKey, int32(tenant.FromContext(ctx))), nil
	}
	module, err := New(Dependencies{DB: db, StudentDirectory: testpkg.StudentDirectory{}, StudentOwners: lockTestOwners{},
		StudentClassWriteGateQuery: gate, Observe: func(Observation) {}}, nil)
	require.NoError(t, err)
	student := testpkg.CreateTestStudent(t, fixtures, "Gate", "Order", "1a")

	release, transitionPID := holdExclusiveClassGate(t, db)
	locked := make(chan error, 1)
	go func() {
		locked <- testpkg.WithTenantTx(t, ctx, db, testpkg.Tenant(t), func(txCtx context.Context, _ bun.Tx) error {
			_, err := module.FindStudentRecordForMutation(txCtx, student.ID)
			return err
		})
	}()
	awaitBlockedBy(t, db, transitionPID)

	require.NoError(t, testpkg.WithTenantTx(t, ctx, db, testpkg.Tenant(t), func(txCtx context.Context, tx bun.Tx) error {
		var id int64
		return tx.NewRaw("SELECT id FROM users.student_profiles WHERE id = ? FOR UPDATE NOWAIT", student.ID).Scan(txCtx, &id)
	}), "the waiting lock must not hold the student row")
	release()

	select {
	case err := <-locked:
		require.NoError(t, err)
	case <-time.After(10 * time.Second):
		t.Fatal("the student lock did not finish after the gate was released")
	}
}

// holdExclusiveClassGate holds the class-writes gate exclusively in its own
// tenant transaction, as a grade transition does, until release is called or
// the test ends.
func holdExclusiveClassGate(t *testing.T, db *bun.DB) (release func(), pid int) {
	t.Helper()
	stop := make(chan struct{})
	var once sync.Once
	release = func() { once.Do(func() { close(stop) }) }
	done := make(chan error, 1)
	ready := make(chan int, 1)
	go func() {
		done <- testpkg.WithTenantTx(t, testpkg.Ctx(t), db, testpkg.Tenant(t), func(txCtx context.Context, tx bun.Tx) error {
			var backend int
			if err := tx.NewRaw("SELECT pg_backend_pid()").Scan(txCtx, &backend); err != nil {
				return err
			}
			if _, err := tx.NewRaw("SELECT pg_advisory_xact_lock(?, ?)", studentClassGateKey, int32(testpkg.Tenant(t))).Exec(txCtx); err != nil {
				return err
			}
			ready <- backend
			<-stop
			return nil
		})
	}()
	select {
	case pid = <-ready:
	case err := <-done:
		t.Fatalf("gate holder did not start: %v", err)
	}
	t.Cleanup(func() {
		release()
		select {
		case err := <-done:
			assert.NoError(t, err)
		case <-time.After(10 * time.Second):
			t.Error("gate holder did not commit")
		}
	})
	return release, pid
}

// awaitBlockedBy waits until another backend waits on a lock holderPID holds.
func awaitBlockedBy(t *testing.T, db *bun.DB, holderPID int) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		var waiting bool
		require.NoError(t, db.NewRaw("SELECT EXISTS (SELECT 1 FROM pg_stat_activity WHERE ?::int = ANY (pg_blocking_pids(pid)))", holderPID).
			Scan(context.Background(), &waiting))
		if waiting {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("no backend waited on the gate holder")
}
