package integration

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/gofrs/uuid"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	identityCompose "github.com/moto-nrw/project-phoenix/modules/identityaccess/compose"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// TestOperatorRuntimeEvidence records the flow-specific runtime evidence for
// the operator login, refresh and revocation cutover (#2720): statement
// counts, latency, affected rows, pool waits and deadlocks per operation on
// an isolated clone. It mirrors the operator auth service's sequences over
// the public capability; the raw JSON is logged for the migration ticket.
func TestOperatorRuntimeEvidence(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	access, err := identityCompose.New(identityCompose.Dependencies{DB: db, Observe: func(identityCompose.Observation) {}})
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
	measure := func(operation string, iteration int, fn func() error) {
		counter.Reset()
		before := db.Stats()
		started := time.Now()
		err := fn()
		elapsed := time.Since(started)
		after := db.Stats()
		require.NoError(t, err)
		if iteration < 5 {
			return
		}
		writes := counter.WriteRows()
		rows, statements := counter.Rows()
		samples[operation] = append(samples[operation], testpkg.RuntimeCheckpointSample{
			DurationMS: float64(elapsed) / float64(time.Millisecond), Queries: counter.Total(),
			WriteRowsAffected: &writes, RowsAffected: rows, StatementsWithRows: statements,
			PoolWaitCount: after.WaitCount - before.WaitCount, PoolWaitMS: float64(after.WaitDuration-before.WaitDuration) / float64(time.Millisecond),
		})
	}
	for iteration := range 35 {
		familyID := uuid.Must(uuid.NewV4()).String()
		var session identityaccess.OperatorSession
		// Login: credential lookup, session mint, login stamp.
		measure("login", iteration, func() error {
			if _, err := access.FindOperatorByEmail(ctx, operator.Email); err != nil {
				return err
			}
			var err error
			session, err = access.CreateOperatorSession(ctx, newSession(operator.ID, familyID, 0))
			if err != nil {
				return err
			}
			return access.RecordOperatorLogin(ctx, operator.ID)
		})
		// Refresh: the rotation the operator auth service runs inside one
		// administrative transaction.
		measure("refresh", iteration, func() error {
			return testpkg.WithAdminTx(t, ctx, db, func(txCtx context.Context, _ bun.Tx) error {
				presented, err := access.FindOperatorSessionForUpdate(txCtx, session.Token)
				if err != nil {
					return err
				}
				if _, err := access.LatestOperatorSessionInFamily(txCtx, presented.FamilyID); err != nil {
					return err
				}
				if _, err := access.FindOperator(txCtx, presented.OperatorID); err != nil {
					return err
				}
				successor, err := access.CreateOperatorSession(txCtx, newSession(operator.ID, presented.FamilyID, presented.Generation+1))
				if err != nil {
					return err
				}
				if err := access.MarkOperatorSessionRotated(txCtx, presented.ID, successor.Token, []byte{1}, time.Now()); err != nil {
					return err
				}
				return access.DeleteExpiredRotatedOperatorSessions(txCtx, presented.FamilyID, time.Now())
			})
		})
		// Revoke: password change and logout delete the operator's sessions.
		measure("revoke", iteration, func() error {
			return testpkg.WithAdminTx(t, ctx, db, func(txCtx context.Context, _ bun.Tx) error {
				_, err := access.RevokeOperatorSessions(txCtx, operator.ID)
				return err
			})
		})
	}
	raw, err := json.Marshal(map[string]any{"postgres": version, "warmup": 5, "samples_per_operation": 30, "concurrency": 1, "samples": samples, "unit_of_work_events_including_warmup": events(), "deadlocks": deadlocks() - beforeDeadlocks})
	require.NoError(t, err)
	t.Logf("operator-runtime %s", raw)
}
