//go:build testhooks

package enrollmenthttp_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	api "github.com/moto-nrw/project-phoenix/api"

	enrollment "github.com/moto-nrw/project-phoenix/modules/enrollment"
	enrollmentAPI "github.com/moto-nrw/project-phoenix/modules/enrollment/http"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// The competing writer is a real existing_students Intake, with a distinct
// email so only the phase match lock serializes it with the Restore.
func TestRestoreMatchedStudentWaitingWriterLifetime(t *testing.T) {
	t.Parallel()
	for _, finish := range []string{"commit", "rollback", "cancel_waiter"} {
		t.Run(finish, func(t *testing.T) {
			env, requestID, _, submitAgain := restoreMatchFailureFixture(t)
			require.Same(t, testpkg.SetupIsolatedTestDB(t), env.db)
			baseCtx := testpkg.Ctx(t)
			ctx, cancel := context.WithTimeout(baseCtx, 10*time.Second)
			defer cancel()
			writerCtx, cancelWriter := context.WithCancel(ctx)
			defer cancelWriter()
			otherPhase := testpkg.CreateTestEnrollmentPhase(t, env.db)
			counter := testpkg.CaptureQueries(t, env.db)
			defer counter.Stop()
			schedule := &restoreMatchSchedule{DecisionRequests: env.repos.Enrollment()}
			decision := restoreMatchDecision(env, schedule)
			pidReady := make(chan int, 1)
			writerDone := make(chan error, 1)
			started, drained := false, false
			t.Cleanup(func() {
				cancelWriter()
				if started && !drained {
					select {
					case <-writerDone:
					case <-time.After(5 * time.Second):
						t.Error("owned writer did not drain after cancellation")
					}
				}
			})
			schedule.afterCheck = func(txCtx context.Context, n int) error {
				if n != 1 {
					return nil
				}
				started = true
				go func() {
					writerDone <- testpkg.WithTenantTx(t, writerCtx, env.db, testpkg.Tenant(t), func(writeCtx context.Context, tx bun.Tx) error {
						var pid int
						if err := tx.NewRaw("SELECT pg_backend_pid()").Scan(writeCtx, &pid); err != nil {
							return err
						}
						pidReady <- pid
						return submitAgain(writeCtx)
					})
				}()
				var pid int
				select {
				case pid = <-pidReady:
				case err := <-writerDone:
					drained = true
					return err
				case <-ctx.Done():
					return ctx.Err()
				}
				// Observe a real ungranted match lock, not a timed assumption. The poll
				// has a deadline and yields through a ticker; no sleeps or fake blockers.
				ticker := time.NewTicker(time.Millisecond)
				defer ticker.Stop()
				for {
					var waiting bool
					err := env.db.NewRaw("SELECT EXISTS (SELECT 1 FROM pg_locks WHERE pid = ? AND locktype = 'advisory' AND classid = ? AND objid = ? AND NOT granted)", pid, int32(0x656e726c), env.sourcePhase.ID).Scan(ctx, &waiting)
					if err != nil {
						return err
					}
					if waiting {
						break
					}
					select {
					case err := <-writerDone:
						drained = true
						return errors.Join(errors.New("writer finished before waiting on phase match lock"), err)
					case <-ctx.Done():
						return ctx.Err()
					case <-ticker.C:
					}
				}
				// A different phase remains independent while this phase is held.
				if err := testpkg.WithTenantTx(t, ctx, env.db, testpkg.Tenant(t), func(otherCtx context.Context, _ bun.Tx) error {
					return env.repos.Enrollment().AcquireExistingStudentMatchLock(otherCtx, otherPhase.ID)
				}); err != nil {
					return err
				}
				if finish == "cancel_waiter" {
					// pgdriver does not interrupt an in-flight lock wait on
					// cancellation alone. Release the holder before draining.
					cancelWriter()
				}
				return nil
			}
			rollback := errors.New("caller rolls back restored request")
			var outcome *enrollmentAPI.RestoreOutcome
			callErr := testpkg.WithTenantTx(t, ctx, env.db, testpkg.Tenant(t), func(txCtx context.Context, tx bun.Tx) error {
				var err error
				outcome, err = decision.RestoreWithdrawn(txCtx, requestID, env.creatorID)
				if err != nil {
					return err
				}
				// Restore returning must not release the caller's transaction lock.
				if finish != "cancel_waiter" {
					select {
					case err := <-writerDone:
						drained = true
						return errors.Join(errors.New("writer escaped before caller transaction ended"), err)
					default:
					}
				}
				err = testpkg.WithTenantTx(t, ctx, env.db, testpkg.Tenant(t), func(probeCtx context.Context, probeTx bun.Tx) error {
					var acquired bool
					if err := probeTx.NewRaw("SELECT pg_try_advisory_xact_lock(?, ?)", int32(0x656e726c), env.sourcePhase.ID).Scan(probeCtx, &acquired); err != nil {
						return err
					}
					if acquired {
						return errors.New("caller transaction lost phase match lock")
					}
					return nil
				})
				if err != nil {
					return err
				}
				if finish == "rollback" {
					return rollback
				}
				return nil
			})
			var writerErr error
			if !drained {
				select {
				case writerErr = <-writerDone:
					drained = true
				case <-ctx.Done():
					t.Fatal("owned writer did not drain after transaction end")
				}
			}
			counter.Stop()
			t.Logf("SQL=%q", counter.Queries())
			t.Logf("finish=%s restore_error=%v writer_error=%v outcome=%+v", finish, callErr, writerErr, outcome)
			if finish == "rollback" {
				require.ErrorIs(t, callErr, rollback)
				require.NoError(t, writerErr)
			} else {
				require.NoError(t, callErr)
				if finish == "commit" {
					require.ErrorIs(t, writerErr, enrollment.ErrExistingStudentAlreadyRequested)
				} else {
					require.ErrorIs(t, writerErr, context.Canceled)
				}
			}
			require.NotNil(t, outcome)
			// Both transaction outcomes and cancellation leave the gate reusable.
			require.NoError(t, testpkg.WithTenantTx(t, ctx, env.db, testpkg.Tenant(t), func(retryCtx context.Context, _ bun.Tx) error {
				return env.repos.Enrollment().AcquireExistingStudentMatchLock(retryCtx, env.sourcePhase.ID)
			}))
			children, readErr := enrollmentAPI.ReadOwnerRequestChildrenForTest(ctx, env.repos.Enrollment(), requestID)
			require.NoError(t, readErr)
			request, readErr := enrollmentAPI.ReadOwnerRequestForTest(ctx, env.repos.Enrollment(), requestID)
			require.NoError(t, readErr)
			var audit []*api.EnrollmentFlowRestorationAudit
			require.NoError(t, testpkg.WithTenantTx(t, ctx, env.db, testpkg.Tenant(t), func(readCtx context.Context, tx bun.Tx) error {
				var err error
				audit, err = readRestoreMatchAudit(readCtx, tx, testpkg.Tenant(t), requestID)
				return err
			}))
			state, encodeErr := json.Marshal(struct {
				Request  any
				Children any
				Audit    any
			}{request, children, audit})
			t.Logf("finish=%s physical_state=%s encode_error=%v", finish, state, encodeErr)
			require.NoError(t, encodeErr)
			require.Len(t, children, 3)
			for _, child := range children {
				if finish == "rollback" {
					require.Equal(t, enrollment.ChildStatusWithdrawn, child.Status)
				} else {
					require.Equal(t, enrollment.ChildStatusSubmitted, child.Status)
				}
			}
			if finish == "rollback" {
				require.NotNil(t, request.WithdrawnAt)
				require.Empty(t, audit)
			} else {
				require.Nil(t, request.WithdrawnAt)
				require.Len(t, audit, 1)
			}
		})
	}
}
