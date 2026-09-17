package integration

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/gofrs/uuid"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	identityCompose "github.com/moto-nrw/project-phoenix/modules/identityaccess/compose"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// TestOperatorPasskeyRuntimeEvidence records the runtime evidence for the
// operator passkey records cutover (#2724): statement counts, latency,
// affected rows, pool waits, unit-of-work outcomes (including the rollback of
// an injected registration failure), deadlocks and the duplicate-prevention
// conflicts per operation on an isolated clone. It mirrors the sequences the
// operator passkey service runs over the public capability; the raw JSON is
// logged for the migration ticket.
func TestOperatorPasskeyRuntimeEvidence(t *testing.T) {
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
	handle := []byte("handle-" + uuid.Must(uuid.NewV4()).String())
	register := func(txCtx context.Context, sessionID string, credential identityaccess.OperatorPasskeyCredential) error {
		if _, err := records.ConsumeOperatorPasskeySession(txCtx, sessionID, identityaccess.OperatorPasskeySessionPurposeRegistration, time.Now()); err != nil {
			return err
		}
		_, err := records.CreateOperatorPasskey(txCtx, credential)
		return err
	}

	for iteration := range 35 {
		// Registration options: existing credentials build the WebAuthn user,
		// then the ceremony is stored.
		var registration identityaccess.OperatorPasskeySession
		measure("begin_registration", iteration, func() error {
			if _, err := records.ListActiveOperatorPasskeys(ctx, operator.ID); err != nil {
				return err
			}
			var err error
			registration, err = records.CreateOperatorPasskeySession(ctx,
				newOperatorPasskeySession(t, db, &operator.ID, identityaccess.OperatorPasskeySessionPurposeRegistration, time.Now().Add(5*time.Minute)))
			return err
		})
		credential := newOperatorPasskey(operator.ID, handle)
		// Registration with an injected failure after both writes rolls back.
		measure("finish_registration_rolled_back", iteration, func() error {
			err := testpkg.WithAdminTx(t, ctx, db, func(txCtx context.Context, _ bun.Tx) error {
				if err := register(txCtx, registration.ID, credential); err != nil {
					return err
				}
				return errInjected
			})
			if errors.Is(err, errInjected) {
				return nil
			}
			return err
		})
		// The retry completes the same ceremony.
		measure("finish_registration", iteration, func() error {
			return testpkg.WithAdminTx(t, ctx, db, func(txCtx context.Context, _ bun.Tx) error {
				return register(txCtx, registration.ID, credential)
			})
		})
		// A replayed completion is refused.
		if _, err := records.ConsumeOperatorPasskeySession(ctx, registration.ID, identityaccess.OperatorPasskeySessionPurposeRegistration, time.Now()); errors.Is(err, identityaccess.ErrOperatorPasskeySessionNotFound) {
			conflicts["replayed_ceremony"]++
		}
		// A second registration of the same authenticator is refused.
		if _, err := records.CreateOperatorPasskey(ctx, credential); err != nil {
			conflicts["duplicate_credential"]++
		}
		var login identityaccess.OperatorPasskeySession
		measure("begin_login", iteration, func() error {
			var err error
			login, err = records.CreateOperatorPasskeySession(ctx,
				newOperatorPasskeySession(t, db, nil, identityaccess.OperatorPasskeySessionPurposeLogin, time.Now().Add(5*time.Minute)))
			return err
		})
		// Login: consume the ceremony, resolve the credential and the
		// operator's other credentials, store the new signature state.
		measure("finish_login", iteration, func() error {
			if _, err := records.ConsumeOperatorPasskeySession(ctx, login.ID, identityaccess.OperatorPasskeySessionPurposeLogin, time.Now()); err != nil {
				return err
			}
			found, err := records.FindActiveOperatorPasskey(ctx, credential.CredentialID, handle)
			if err != nil {
				return err
			}
			if _, err := records.ListActiveOperatorPasskeys(ctx, operator.ID); err != nil {
				return err
			}
			return records.RecordOperatorPasskeyUse(ctx, found.ID, json.RawMessage(`{"id":"used"}`), time.Now())
		})
		measure("list_passkeys", iteration, func() error {
			_, err := records.ListActiveOperatorPasskeys(ctx, operator.ID)
			return err
		})
		measure("revoke_passkey", iteration, func() error {
			found, err := records.FindActiveOperatorPasskey(ctx, credential.CredentialID, handle)
			if err != nil {
				return err
			}
			return records.RevokeOperatorPasskey(ctx, operator.ID, found.ID, time.Now())
		})
	}
	raw, err := json.Marshal(map[string]any{
		"postgres": version, "warmup": 5, "samples_per_operation": 30, "concurrency": 1, "samples": samples,
		"unit_of_work_events_including_warmup": events(), "deadlocks": deadlocks() - beforeDeadlocks,
		"duplicate_prevention_conflicts_including_warmup": conflicts,
	})
	require.NoError(t, err)
	t.Logf("operator-passkey-runtime %s", raw)
	require.Empty(t, failures, "every measured sample must succeed")
	require.Equal(t, 35, conflicts["replayed_ceremony"], "every replay must be refused")
	require.Equal(t, 35, conflicts["duplicate_credential"], "every duplicate registration must be refused")
}
