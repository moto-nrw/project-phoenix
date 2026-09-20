package migrations

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestStudentOwnerContractObservedLockWaitAndTimeout(t *testing.T) {
	t.Parallel()
	for _, holdUntilTimeout := range []bool{false, true} {
		name := "released"
		if holdUntilTimeout {
			name = "timeout"
		}
		t.Run(name, func(t *testing.T) {
			db, _ := studentContractFixture(t)
			before := studentContractOwnerRows(t, db)
			holder, err := db.BeginTx(t.Context(), nil)
			require.NoError(t, err)
			defer func() { _ = holder.Rollback() }()
			_, err = holder.ExecContext(t.Context(), `LOCK TABLE users.student_profiles IN SHARE MODE`)
			require.NoError(t, err)
			var holderPID int
			require.NoError(t, holder.NewRaw(`SELECT pg_backend_pid()`).Scan(t.Context(), &holderPID))
			ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
			defer cancel()
			done := make(chan error, 1)
			started := time.Now()
			go func() { done <- contractStudentOwnerStorage(ctx, db) }()
			waiting := func() bool {
				var blocked bool
				err := db.NewRaw(`SELECT EXISTS (SELECT FROM pg_stat_activity
					WHERE datname = current_database() AND wait_event_type = 'Lock'
					AND ? = ANY(pg_blocking_pids(pid))
					AND query LIKE '%LOCK TABLE users.students IN ACCESS EXCLUSIVE MODE%')`, holderPID).Scan(t.Context(), &blocked)
				return err == nil && blocked
			}
			require.Eventually(t, waiting, 3*time.Second, 5*time.Millisecond)
			observed := time.Now()
			time.Sleep(150 * time.Millisecond)
			require.True(t, waiting(), "PostgreSQL must still report the same blocker, not merely a slow query")
			observedWait := time.Since(observed)
			if !holdUntilTimeout {
				require.NoError(t, holder.Commit())
			}
			select {
			case result := <-done:
				if holdUntilTimeout {
					require.ErrorContains(t, result, "lock timeout")
				} else {
					require.NoError(t, result)
				}
			case <-time.After(10 * time.Second):
				t.Fatal("Contract did not finish after lock release or timeout")
			}
			if holdUntilTimeout {
				require.NoError(t, holder.Rollback())
			}
			require.Equal(t, before, studentContractOwnerRows(t, db))
			var retained bool
			require.NoError(t, db.NewRaw(`SELECT to_regclass('users.students') IS NOT NULL AND to_regclass('users.students_legacy') IS NOT NULL`).Scan(t.Context(), &retained))
			require.Equal(t, holdUntilTimeout, retained)
			t.Logf("Contract lock case=%s pg_blocking_pids_observed=true observed_wait_lower_bound=%s total_duration=%s snapshots_equal=true", name, observedWait, time.Since(started))
		})
	}
}
