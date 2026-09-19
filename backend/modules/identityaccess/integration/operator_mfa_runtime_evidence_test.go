package integration

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	identityCompose "github.com/moto-nrw/project-phoenix/modules/identityaccess/compose"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// TestOperatorMFARuntimeEvidence records the runtime evidence for the
// operator MFA records cutover (#2723): statement counts, latency, affected
// rows, pool waits, unit-of-work outcomes (including the rollback of an
// injected disable failure), deadlocks and the duplicate-prevention
// conflicts per operation on an isolated clone. It mirrors the sequences the
// operator MFA service runs over the public capability; the raw JSON is
// logged for the migration ticket.
func TestOperatorMFARuntimeEvidence(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	records, err := identityCompose.New(identityCompose.Dependencies{DB: db, Observe: func(identityCompose.Observation) {}})
	require.NoError(t, err)
	operator := testpkg.CreateTestOperator(t, db)
	counter := testpkg.CaptureQueriesForContext(t, db)
	testpkg.AttachLockWaitEvidence(db)
	ctx, events := testpkg.CaptureUnitOfWorkEvidence(counter.Context(testpkg.WithTenantRuntime(t, testpkg.Ctx(t), db)))
	var version string
	require.NoError(t, db.NewRaw("SHOW server_version").Scan(ctx, &version))
	deadlocks := func() int64 {
		var count int64
		require.NoError(t, db.NewRaw("SELECT deadlocks FROM pg_stat_database WHERE datname = current_database()").Scan(ctx, &count))
		return count
	}
	beforeDeadlocks := deadlocks()
	samples := map[string][]testpkg.RuntimeCheckpointSample{}
	var failures []string
	conflicts := map[string]int{}
	measure := func(operation string, iteration int, fn func() error) {
		counter.Reset()
		before := db.Stats()
		started := time.Now()
		err := fn()
		elapsed := time.Since(started)
		after := db.Stats()
		if iteration < 5 {
			require.NoError(t, err, "warmup %s", operation)
			return
		}
		sample := testpkg.RuntimeCheckpointSample{
			DurationMS: float64(elapsed) / float64(time.Millisecond), Queries: counter.Total(),
			PoolWaitCount: after.WaitCount - before.WaitCount, PoolWaitMS: float64(after.WaitDuration-before.WaitDuration) / float64(time.Millisecond),
		}
		if err != nil {
			sample.Status = 1
			sample.ErrorBody = err.Error()
			failures = append(failures, operation+": "+err.Error())
		} else {
			writes := counter.WriteRows()
			rows, statements := counter.Rows()
			sample.WriteRowsAffected = &writes
			sample.RowsAffected = rows
			sample.StatementsWithRows = statements
		}
		samples[operation] = append(samples[operation], sample)
	}
	enroll := func() error {
		_, err := records.CreateOperatorMFACredential(ctx, identityaccess.OperatorMFACredential{OperatorID: operator.ID, Method: "email"})
		return err
	}
	require.NoError(t, enroll())
	disable := func(txCtx context.Context) error {
		if err := records.DeleteOperatorMFACredentials(txCtx, operator.ID); err != nil {
			return err
		}
		if err := records.RevokeOperatorTrustedDevices(txCtx, operator.ID, time.Now()); err != nil {
			return err
		}
		return records.ResetOperatorMFAAttempts(txCtx, operator.ID)
	}

	for iteration := range 35 {
		var challenge identityaccess.OperatorMFAChallenge
		// Start challenge: rate-limit count, pending insert, activation after
		// the synchronous delivery.
		measure("start_challenge", iteration, func() error {
			if _, err := records.CountOperatorMFAChallengesSince(ctx, operator.ID, time.Now().Add(-15*time.Minute)); err != nil {
				return err
			}
			var err error
			challenge, err = records.CreateOperatorMFAChallenge(ctx, pendingChallenge(operator.ID, time.Now().Add(10*time.Minute)))
			if err != nil {
				return err
			}
			return records.ActivateOperatorMFAChallenge(ctx, challenge.ID)
		})
		// Verify: active lookup, single-use consumption, credential stamp.
		measure("verify_challenge", iteration, func() error {
			active, err := records.FindActiveOperatorMFAChallenge(ctx, operator.ID)
			if err != nil {
				return err
			}
			if err := records.ConsumeOperatorMFAChallenge(ctx, active.ID, time.Now()); err != nil {
				return err
			}
			credential, err := records.FindOperatorMFACredential(ctx, operator.ID)
			if err != nil {
				return err
			}
			return records.TouchOperatorMFACredential(ctx, credential.ID, time.Now())
		})
		// A replayed verification of the same code is refused.
		if err := records.ConsumeOperatorMFAChallenge(ctx, challenge.ID, time.Now()); errors.Is(err, identityaccess.ErrOperatorMFAChallengeStateChanged) {
			conflicts["replayed_challenge_consumption"]++
		}
		var device identityaccess.OperatorTrustedDevice
		measure("issue_trusted_device", iteration, func() error {
			var err error
			device, err = records.CreateOperatorTrustedDevice(ctx, newTrustedDevice(operator.ID, time.Now().Add(time.Hour)))
			return err
		})
		measure("verify_trusted_device", iteration, func() error {
			found, err := records.FindActiveOperatorTrustedDevice(ctx, operator.ID, device.TokenHash)
			if err != nil {
				return err
			}
			return records.TouchOperatorTrustedDevice(ctx, found.ID, time.Now())
		})
		measure("list_trusted_devices", iteration, func() error {
			_, err := records.ListActiveOperatorTrustedDevices(ctx, operator.ID)
			return err
		})
		// A duplicate enrollment is refused by the unique key.
		if err := enroll(); err != nil {
			conflicts["duplicate_enrollment"]++
		}
		// Disable with an injected failure after the last write rolls back.
		measure("disable_rolled_back", iteration, func() error {
			err := testpkg.WithAdminTx(t, ctx, db, func(txCtx context.Context, _ bun.Tx) error {
				if err := disable(txCtx); err != nil {
					return err
				}
				return errInjected
			})
			if errors.Is(err, errInjected) {
				return nil
			}
			return err
		})
		measure("disable", iteration, func() error {
			return testpkg.WithAdminTx(t, ctx, db, func(txCtx context.Context, _ bun.Tx) error {
				return disable(txCtx)
			})
		})
		require.NoError(t, enroll())
	}
	raw, err := json.Marshal(map[string]any{
		"postgres": version, "warmup": 5, "samples_per_operation": 30, "concurrency": 1, "samples": samples,
		"unit_of_work_events_including_warmup": events(), "deadlocks": deadlocks() - beforeDeadlocks,
		"duplicate_prevention_conflicts_including_warmup": conflicts,
	})
	require.NoError(t, err)
	t.Logf("operator-mfa-runtime %s", raw)
	require.Empty(t, failures, "every measured sample must succeed")
	require.Equal(t, 35, conflicts["replayed_challenge_consumption"], "every replay must be refused")
	require.Equal(t, 35, conflicts["duplicate_enrollment"], "every duplicate enrollment must be refused")
}
