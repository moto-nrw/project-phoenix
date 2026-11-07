//go:build testhooks

package compose_test

import (
	"context"
	"testing"
	"time"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func TestCleanupQueueOwnerConflictOrderAndOuterCommit(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"folder", "attachments"} {
		t.Run(mode, func(t *testing.T) {
			testpkg.OwnTenant(t)
			db := testpkg.SetupIsolatedTestDB(t)
			fixture := cleanupOwnerFixture(t, db, mode)
			ctx, cancel := context.WithTimeout(testpkg.Ctx(t), 10*time.Second)
			defer cancel()
			tenantID := testpkg.Tenant(t)
			otherTenant, _ := testpkg.CreateTestTenant(t, db)
			testpkg.OwnTenantRows(t, db, otherTenant)
			stamp := time.Date(2020, 1, 2, 3, 4, 5, 0, time.UTC)
			require.NoError(t, testpkg.WithTenantTx(t, testpkg.ContextForTenant(ctx, otherTenant), db, otherTenant, func(txCtx context.Context, tx bun.Tx) error {
				return cleanupProbeInsertIntent(txCtx, tx, mode, otherTenant, fixture.ownerID, fixture.names[0], stamp)
			}))
			foreignBefore, err := cleanupProbeIntentRows(ctx, db, mode, otherTenant, fixture.names)
			require.NoError(t, err)
			foreign := cleanupBeginTransaction(t, ctx, db, otherTenant)
			cleanupLockName(t, foreign, mode, fixture.names[0])
			var otherKind *cleanupHeldTransaction
			var kindBefore []string
			if mode == "folder" {
				_, err := db.NewRaw(`INSERT INTO documents.file_cleanup (tenant_id, object_kind, owner_id, filename_stored, retry_after, cleaned_at)
VALUES (?, 'student-photos', ?, ?, ?, ?)`, tenantID, fixture.ownerID, fixture.names[0], stamp, stamp).Exec(ctx)
				require.NoError(t, err)
				require.NoError(t, db.NewRaw(`SELECT row_to_json(i)::text FROM documents.file_cleanup AS i WHERE tenant_id = ? AND object_kind = 'student-photos' ORDER BY id`, tenantID).Scan(ctx, &kindBefore))
				holder := cleanupBeginTransaction(t, ctx, db, tenantID)
				var id int64
				require.NoError(t, holder.tx.NewRaw(`SELECT id FROM documents.file_cleanup WHERE tenant_id = ? AND object_kind = 'student-photos' AND filename_stored = ? FOR UPDATE`, tenantID, fixture.names[0]).Scan(holder.ctx, &id))
				otherKind = &holder
			}
			first := cleanupBeginTransaction(t, ctx, db, tenantID)
			second := cleanupBeginTransaction(t, ctx, db, tenantID)
			cleanupLockName(t, first, mode, fixture.names[0])
			cleanupLockName(t, second, mode, fixture.names[1])
			contender := cleanupBeginTransaction(t, ctx, db, tenantID)
			start := cleanupProtocolSequence(t, ctx, db, mode)
			done := make(chan error, 1)
			go func() { done <- fixture.run(contender.ctx) }()
			cleanupAwaitBlocker(t, ctx, db, contender.pid, first.pid)
			require.Equal(t, start+1, cleanupProtocolSequence(t, ctx, db, mode))
			var blockers []int
			require.NoError(t, db.NewRaw("SELECT unnest(pg_blocking_pids(?))", contender.pid).Scan(ctx, &blockers))
			require.Equal(t, []int{first.pid}, blockers, "other tenant/kind locks do not block the operation")
			require.NoError(t, first.tx.Commit())
			cleanupAwaitBlocker(t, ctx, db, contender.pid, second.pid)
			require.Equal(t, start+2, cleanupProtocolSequence(t, ctx, db, mode))
			waiter := cleanupBeginTransaction(t, ctx, db, tenantID)
			waited := make(chan error, 1)
			go func() {
				var id int64
				query := "SELECT id FROM " + cleanupProtocolTable(mode) + " WHERE tenant_id = ? AND filename_stored = ?"
				if mode == "folder" {
					query += " AND object_kind = 'files'"
				}
				waited <- waiter.tx.NewRaw(query+" FOR UPDATE", tenantID, fixture.names[0]).Scan(waiter.ctx, &id)
			}()
			cleanupAwaitBlocker(t, ctx, db, waiter.pid, contender.pid)
			require.NoError(t, second.tx.Commit())
			require.NoError(t, cleanupReceive(t, ctx, done))
			require.Equal(t, start+3, cleanupProtocolSequence(t, ctx, db, mode))
			cleanupAwaitBlocker(t, ctx, db, waiter.pid, contender.pid)
			select {
			case err := <-waited:
				t.Fatalf("first intent unlocked before outer commit: %v", err)
			default:
			}
			require.NoError(t, contender.tx.Commit())
			require.NoError(t, cleanupReceive(t, ctx, waited))
			require.NoError(t, waiter.tx.Commit())
			require.NoError(t, foreign.tx.Commit())
			foreignAfter, err := cleanupProbeIntentRows(ctx, db, mode, otherTenant, fixture.names)
			require.NoError(t, err)
			require.Equal(t, foreignBefore, foreignAfter)
			if otherKind != nil {
				require.NoError(t, otherKind.tx.Commit())
				var kindAfter []string
				require.NoError(t, db.NewRaw(`SELECT row_to_json(i)::text FROM documents.file_cleanup AS i WHERE tenant_id = ? AND object_kind = 'student-photos' ORDER BY id`, tenantID).Scan(ctx, &kindAfter))
				require.Equal(t, kindBefore, kindAfter)
			}
		})
	}
}
