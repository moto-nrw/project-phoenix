package integration

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	identityCompose "github.com/moto-nrw/project-phoenix/modules/identityaccess/compose"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// TestAccountSessionRuntimeEvidence records the flow-specific runtime evidence
// for the tenant, parent and school login, refresh, switch and revocation
// cutover (#2720): statement counts, latency, affected rows, pool waits and
// deadlocks per operation on an isolated clone. It mirrors the session
// sequences the auth service runs inside one administrative transaction over
// the public capability; the raw JSON is logged for the migration ticket.
func TestAccountSessionRuntimeEvidence(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	access, err := identityCompose.New(identityCompose.Dependencies{DB: db, Observe: func(identityCompose.Observation) {}})
	require.NoError(t, err)
	account := testpkg.CreateTestAccount(t, db, "session-runtime")
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
			// A failed sample is recorded as an error, not an abort: the error
			// rate per operation is a measurement, so the run keeps going.
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
	mint := func(txCtx context.Context, familyID string, generation int) (identityaccess.AccountSession, error) {
		session := newAccountSession(account.ID, "tenant", familyID, generation)
		session.TenantID = tenantID
		return access.CreateAccountSession(txCtx, session)
	}
	for iteration := range 35 {
		var session identityaccess.AccountSession
		// Login: the session mint, the hand-off sweep and the portal session
		// cap inside the administrative login transaction.
		measure("login", iteration, func() error {
			return testpkg.WithAdminTx(t, ctx, db, func(txCtx context.Context, _ bun.Tx) error {
				var err error
				session, err = mint(txCtx, newFamilyID(), 0)
				if err != nil {
					return err
				}
				if err := access.DeleteExpiredRotatedAccountSessions(txCtx, account.ID, time.Now()); err != nil {
					return err
				}
				_, err = access.EnforceAccountSessionCap(txCtx, account.ID, "tenant", 5)
				return err
			})
		})
		// Refresh: the rotation the auth service runs inside one
		// administrative transaction after locking the account.
		measure("refresh", iteration, func() error {
			return testpkg.WithAdminTx(t, ctx, db, func(txCtx context.Context, _ bun.Tx) error {
				unlocked, err := access.FindAccountSession(txCtx, session.Token)
				if err != nil {
					return err
				}
				presented, err := access.FindAccountSessionForUpdate(txCtx, unlocked.Token)
				if err != nil {
					return err
				}
				if _, err := access.LatestAccountSessionInFamily(txCtx, presented.FamilyID); err != nil {
					return err
				}
				successor, err := mint(txCtx, presented.FamilyID, presented.Generation+1)
				if err != nil {
					return err
				}
				if err := access.MarkAccountSessionRotated(txCtx, presented.ID, successor.Token, []byte{1}, time.Now()); err != nil {
					return err
				}
				if err := access.DeleteExpiredRotatedAccountSessions(txCtx, account.ID, time.Now()); err != nil {
					return err
				}
				session = successor
				return nil
			})
		})
		// Switch: the presented family is retired ahead of the mint and the
		// cap, so the new session replaces it instead of adding to it.
		measure("switch", iteration, func() error {
			return testpkg.WithAdminTx(t, ctx, db, func(txCtx context.Context, _ bun.Tx) error {
				if err := access.RetireAccountSessionFamily(txCtx, account.ID, session.FamilyID, time.Now().Add(time.Minute)); err != nil {
					return err
				}
				switched, err := mint(txCtx, newFamilyID(), 0)
				if err != nil {
					return err
				}
				if err := access.DeleteExpiredRotatedAccountSessions(txCtx, account.ID, time.Now()); err != nil {
					return err
				}
				if _, err := access.EnforceAccountSessionCap(txCtx, account.ID, "tenant", 5); err != nil {
					return err
				}
				session = switched
				return nil
			})
		})
		// Revoke: logout deletes the presented family; the tenant-scoped
		// administrative revoke deletes the account's sessions at one school.
		measure("revoke", iteration, func() error {
			return testpkg.WithAdminTx(t, ctx, db, func(txCtx context.Context, _ bun.Tx) error {
				presented, err := access.FindAccountSession(txCtx, session.Token)
				if err != nil {
					return err
				}
				if _, err := access.RevokeAccountSessionFamily(txCtx, presented.FamilyID); err != nil {
					return err
				}
				_, err = access.RevokeAccountSessionsInTenant(testpkg.ContextForTenant(txCtx, tenantID), account.ID)
				return err
			})
		})
	}
	raw, err := json.Marshal(map[string]any{"postgres": version, "warmup": 5, "samples_per_operation": 30, "concurrency": 1, "samples": samples, "unit_of_work_events_including_warmup": events(), "deadlocks": deadlocks() - beforeDeadlocks})
	require.NoError(t, err)
	t.Logf("account-session-runtime %s", raw)
	require.Empty(t, failures, "every measured sample must succeed")
}
