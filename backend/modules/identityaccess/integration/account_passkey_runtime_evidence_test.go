package integration

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/gofrs/uuid"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	identityCompose "github.com/moto-nrw/project-phoenix/modules/identityaccess/compose"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// TestAccountPasskeyRuntimeEvidence records the runtime evidence for the
// school-portal passkey records cutover (#2724): statement counts, latency,
// affected rows, pool waits, unit-of-work outcomes (including the rollback of
// an injected registration failure), deadlocks and the duplicate-prevention
// conflicts per operation on an isolated clone. It mirrors the sequences the
// retained passkey service runs over the public capability; the raw JSON is
// logged for the migration ticket.
func TestAccountPasskeyRuntimeEvidence(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	records, err := identityCompose.New(identityCompose.Dependencies{DB: db, Observe: func(identityCompose.Observation) {}})
	require.NoError(t, err)
	account := testpkg.CreateTestAccount(t, db, "passkey-"+uuid.Must(uuid.NewV4()).String()+"@example.test")
	tenantID := testpkg.Tenant(t)
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
	register := func(txCtx context.Context, sessionID string, credential identityaccess.AccountPasskeyCredential) error {
		if _, err := records.ConsumeAccountPasskeySession(txCtx, sessionID, identityaccess.PasskeySessionPurposeRegistration, time.Now()); err != nil {
			return err
		}
		_, err := records.CreateAccountPasskey(txCtx, credential)
		return err
	}

	for iteration := range 35 {
		// Registration options: existing credentials build the WebAuthn user,
		// then the ceremony is stored.
		var registration identityaccess.AccountPasskeySession
		measure("begin_registration", iteration, func() error {
			if _, err := records.ListActiveAccountPasskeys(ctx, account.ID); err != nil {
				return err
			}
			var err error
			registration, err = records.CreateAccountPasskeySession(ctx,
				newAccountPasskeySession(t, db, &account.ID, &tenantID, identityaccess.PasskeySessionPurposeRegistration, time.Now().Add(5*time.Minute)))
			return err
		})
		credential := newAccountPasskey(account.ID, handle)
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
		if _, err := records.ConsumeAccountPasskeySession(ctx, registration.ID, identityaccess.PasskeySessionPurposeRegistration, time.Now()); errors.Is(err, identityaccess.ErrAccountPasskeySessionNotFound) {
			conflicts["replayed_ceremony"]++
		}
		// A second registration of the same authenticator is refused.
		if _, err := records.CreateAccountPasskey(ctx, credential); err != nil &&
			strings.Contains(err.Error(), "uniq_passkey_credentials_credential_id") {
			conflicts["duplicate_credential"]++
		}
		var login identityaccess.AccountPasskeySession
		measure("begin_login", iteration, func() error {
			var err error
			login, err = records.CreateAccountPasskeySession(ctx,
				newAccountPasskeySession(t, db, nil, &tenantID, identityaccess.PasskeySessionPurposeLogin, time.Now().Add(5*time.Minute)))
			return err
		})
		// Login, in the administrative transaction the service opens: consume
		// the ceremony, resolve the credential and the account's other
		// credentials, store the new signature state.
		measure("finish_login", iteration, func() error {
			return testpkg.WithAdminTx(t, ctx, db, func(txCtx context.Context, _ bun.Tx) error {
				if _, err := records.ConsumeAccountPasskeySession(txCtx, login.ID, identityaccess.PasskeySessionPurposeLogin, time.Now()); err != nil {
					return err
				}
				found, err := records.FindActiveAccountPasskey(txCtx, credential.CredentialID, handle)
				if err != nil {
					return err
				}
				if _, err := records.ListActiveAccountPasskeys(txCtx, account.ID); err != nil {
					return err
				}
				return records.RecordAccountPasskeyUse(txCtx, found.ID, json.RawMessage(`{"id":"used"}`), time.Now())
			})
		})
		measure("list_passkeys", iteration, func() error {
			_, err := records.ListActiveAccountPasskeys(ctx, account.ID)
			return err
		})
		measure("revoke_passkey", iteration, func() error {
			found, err := records.FindActiveAccountPasskey(ctx, credential.CredentialID, handle)
			if err != nil {
				return err
			}
			return records.RevokeAccountPasskey(ctx, account.ID, found.ID, time.Now())
		})
	}
	raw, err := json.Marshal(map[string]any{
		"postgres": version, "warmup": 5, "samples_per_operation": 30, "concurrency": 1, "samples": samples,
		"unit_of_work_events_including_warmup": events(), "deadlocks": deadlocks() - beforeDeadlocks,
		"duplicate_prevention_conflicts_including_warmup": conflicts,
	})
	require.NoError(t, err)
	t.Logf("account-passkey-runtime %s", raw)
	require.Empty(t, failures, "every measured sample must succeed")
	require.Equal(t, 35, conflicts["replayed_ceremony"], "every replay must be refused")
	require.Equal(t, 35, conflicts["duplicate_credential"], "every duplicate registration must be refused")
}
