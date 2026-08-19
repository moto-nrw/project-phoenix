package schedule_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	scheduleRepo "github.com/moto-nrw/project-phoenix/database/repositories/schedule"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestTimetableConflictAckRepository_CapPrunesOldest verifies the per-account
// bound (#2151 review): fingerprints are opaque to the server, so the
// repository must not let one account accumulate unbounded rows — inserts
// beyond MaxConflictAcksPerAccount evict the oldest acknowledgements first.
func TestTimetableConflictAckRepository_CapPrunesOldest(t *testing.T) {
	db := testpkg.SetupTestDB(t)

	ctx := testpkg.TenantContext(1)
	repo := scheduleRepo.NewTimetableConflictAckRepository(db)

	account := testpkg.CreateTestAccount(t, db, fmt.Sprintf("ack-cap-%d@example.com", time.Now().UnixNano()))
	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(),
			"DELETE FROM schedule.timetable_conflict_acks WHERE account_id = ?", account.ID)
		testpkg.CleanupTableRecords(t, db, "auth.accounts", account.ID)
	})

	const extra = 10
	total := scheduleModels.MaxConflictAcksPerAccount + extra
	fingerprint := func(i int) string { return fmt.Sprintf("%032x", i) }
	for i := 0; i < total; i++ {
		require.NoError(t, repo.Acknowledge(ctx, account.ID, fingerprint(i)))
	}

	got, err := repo.ListFingerprintsByAccount(ctx, account.ID)
	require.NoError(t, err)
	assert.Len(t, got, scheduleModels.MaxConflictAcksPerAccount, "cap must hold after overflow")

	kept := make(map[string]struct{}, len(got))
	for _, fp := range got {
		kept[fp] = struct{}{}
	}
	for i := 0; i < extra; i++ {
		assert.NotContains(t, kept, fingerprint(i), "oldest acks must be pruned first")
	}
	assert.Contains(t, kept, fingerprint(total-1), "the newest ack must survive")

	// Idempotent re-ack of a surviving fingerprint must not shrink the set —
	// duplicates skip the prune entirely.
	require.NoError(t, repo.Acknowledge(ctx, account.ID, fingerprint(total-1)))
	got, err = repo.ListFingerprintsByAccount(ctx, account.ID)
	require.NoError(t, err)
	assert.Len(t, got, scheduleModels.MaxConflictAcksPerAccount)
}
