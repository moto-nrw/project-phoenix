package compose

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

const (
	conflictAckFingerprintA = "0123456789abcdef0123456789abcdef"
	conflictAckFingerprintB = "fedcba9876543210fedcba9876543210"
)

func conflictAckAccount(t *testing.T, db *bun.DB) int64 {
	t.Helper()
	account := testpkg.CreateTestAccount(t, db, fmt.Sprintf("conflict-ack-%d@example.com", time.Now().UnixNano()))
	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(),
			"DELETE FROM schedule.timetable_conflict_acks WHERE account_id = ?", account.ID)
	})
	return account.ID
}

func TestModuleOwnsConflictAckLifecycle(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	log := &observationLog{}
	module := buildModule(t, db, log.record)
	ctx := testpkg.Ctx(t)
	accountID := conflictAckAccount(t, db)

	listed, err := module.ListConflictAcks(ctx, accountID)
	require.NoError(t, err)
	assert.Empty(t, listed)

	require.NoError(t, module.AcknowledgeConflict(ctx, accountID, conflictAckFingerprintB))
	require.NoError(t, module.AcknowledgeConflict(ctx, accountID, conflictAckFingerprintA))
	listed, err = module.ListConflictAcks(ctx, accountID)
	require.NoError(t, err)
	assert.Equal(t, []string{conflictAckFingerprintA, conflictAckFingerprintB}, listed, "fingerprints are listed in stable order")
	assert.EqualValues(t, 1, observedOperation(log.seen, "list_conflict_acks").Stats.Queries)
	assert.EqualValues(t, 2, observedOperation(log.seen, "acknowledge_conflict").Stats.Queries, "an actual insert also prunes")

	// Idempotent re-ack: succeeds, does not grow the set, skips the prune.
	require.NoError(t, module.AcknowledgeConflict(ctx, accountID, conflictAckFingerprintA))
	listed, err = module.ListConflictAcks(ctx, accountID)
	require.NoError(t, err)
	assert.Len(t, listed, 2)
	repeated := log.seen[len(log.seen)-2]
	assert.Equal(t, "acknowledge_conflict", repeated.Operation)
	assert.EqualValues(t, 1, repeated.Stats.Queries, "a duplicate ack skips the prune")
	assert.EqualValues(t, 1, repeated.Stats.DuplicatePreventionConflicts)

	require.NoError(t, module.UnacknowledgeConflict(ctx, accountID, conflictAckFingerprintA))
	listed, err = module.ListConflictAcks(ctx, accountID)
	require.NoError(t, err)
	assert.Equal(t, []string{conflictAckFingerprintB}, listed)

	// Removing an unknown fingerprint is a no-op, not an error.
	require.NoError(t, module.UnacknowledgeConflict(ctx, accountID, conflictAckFingerprintA))
	listed, err = module.ListConflictAcks(ctx, accountID)
	require.NoError(t, err)
	assert.Equal(t, []string{conflictAckFingerprintB}, listed)
}

// TestModuleConflictAckCapPrunesOldest verifies the per-account bound (#2151
// review): fingerprints are opaque to the server, so the owner must not let
// one account accumulate unbounded rows. Inserts beyond
// MaxConflictAcksPerAccount evict the oldest acknowledgements first.
func TestModuleConflictAckCapPrunesOldest(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db)
	ctx := testpkg.Ctx(t)
	accountID := conflictAckAccount(t, db)

	const extra = 10
	total := domain.MaxConflictAcksPerAccount + extra
	fingerprint := func(i int) string { return fmt.Sprintf("%032x", i) }
	for i := range total {
		require.NoError(t, module.AcknowledgeConflict(ctx, accountID, fingerprint(i)))
	}

	got, err := module.ListConflictAcks(ctx, accountID)
	require.NoError(t, err)
	assert.Len(t, got, domain.MaxConflictAcksPerAccount, "cap must hold after overflow")

	kept := make(map[string]struct{}, len(got))
	for _, fp := range got {
		kept[fp] = struct{}{}
	}
	for i := range extra {
		assert.NotContains(t, kept, fingerprint(i), "oldest acks must be pruned first")
	}
	assert.Contains(t, kept, fingerprint(total-1), "the newest ack must survive")

	// Idempotent re-ack of a surviving fingerprint must not shrink the set.
	require.NoError(t, module.AcknowledgeConflict(ctx, accountID, fingerprint(total-1)))
	got, err = module.ListConflictAcks(ctx, accountID)
	require.NoError(t, err)
	assert.Len(t, got, domain.MaxConflictAcksPerAccount)
}

func TestModuleConflictAcksAreTenantIsolated(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db)
	ctx := testpkg.Ctx(t)
	accountID := conflictAckAccount(t, db)

	foreignTenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, foreignTenantID)
	foreignCtx := tenant.WithTenantID(testpkg.WithPackageTenantRuntime(context.Background()), foreignTenantID)

	require.NoError(t, module.AcknowledgeConflict(ctx, accountID, conflictAckFingerprintA))

	// The same account sees nothing from the owned tenant in the foreign one.
	foreign, err := module.ListConflictAcks(foreignCtx, accountID)
	require.NoError(t, err)
	assert.Empty(t, foreign)

	// A foreign-tenant removal cannot reach the owned tenant's row.
	require.NoError(t, module.UnacknowledgeConflict(foreignCtx, accountID, conflictAckFingerprintA))
	owned, err := module.ListConflictAcks(ctx, accountID)
	require.NoError(t, err)
	assert.Equal(t, []string{conflictAckFingerprintA}, owned)

	// Acknowledging in the foreign tenant is a separate row, not a duplicate.
	require.NoError(t, module.AcknowledgeConflict(foreignCtx, accountID, conflictAckFingerprintB))
	foreign, err = module.ListConflictAcks(foreignCtx, accountID)
	require.NoError(t, err)
	assert.Equal(t, []string{conflictAckFingerprintB}, foreign)
	owned, err = module.ListConflictAcks(ctx, accountID)
	require.NoError(t, err)
	assert.Equal(t, []string{conflictAckFingerprintA}, owned)
}

func TestModuleConflictAckFailuresAreNotSwallowed(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db)
	accountID := conflictAckAccount(t, db)
	ctx, cancel := context.WithCancel(testpkg.Ctx(t))
	cancel()

	_, err := module.ListConflictAcks(ctx, accountID)
	require.ErrorIs(t, err, context.Canceled)
	require.ErrorIs(t, module.AcknowledgeConflict(ctx, accountID, conflictAckFingerprintA), context.Canceled)
	require.ErrorIs(t, module.UnacknowledgeConflict(ctx, accountID, conflictAckFingerprintA), context.Canceled)
}

func TestModuleConflictAckWritesRollBackWithOuterTransaction(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db)
	ctx := testpkg.Ctx(t)
	accountID := conflictAckAccount(t, db)
	wantErr := errors.New("abort conflict ack write")

	err := tenant.WithinCurrentTenant(ctx, func(txCtx context.Context) error {
		if ackErr := module.AcknowledgeConflict(txCtx, accountID, conflictAckFingerprintA); ackErr != nil {
			return ackErr
		}
		return wantErr
	})
	require.ErrorIs(t, err, wantErr)
	listed, err := module.ListConflictAcks(ctx, accountID)
	require.NoError(t, err)
	assert.Empty(t, listed, "a rolled-back acknowledgement leaves no row")

	require.NoError(t, module.AcknowledgeConflict(ctx, accountID, conflictAckFingerprintA))
	err = tenant.WithinCurrentTenant(ctx, func(txCtx context.Context) error {
		if unackErr := module.UnacknowledgeConflict(txCtx, accountID, conflictAckFingerprintA); unackErr != nil {
			return unackErr
		}
		return wantErr
	})
	require.ErrorIs(t, err, wantErr)
	listed, err = module.ListConflictAcks(ctx, accountID)
	require.NoError(t, err)
	assert.Equal(t, []string{conflictAckFingerprintA}, listed, "a rolled-back removal keeps the row")
}
