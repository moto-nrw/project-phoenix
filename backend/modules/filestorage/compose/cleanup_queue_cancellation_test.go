//go:build testhooks

package compose_test

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/api"
	"github.com/moto-nrw/project-phoenix/modules/filestorage"
	"github.com/moto-nrw/project-phoenix/modules/filestorage/compose"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

type cleanupOwnerOperation struct {
	ownerID int64
	names   []string
	objects *memoryObjects
	run     func(context.Context) error
}

func cleanupOwnerFixture(t *testing.T, db *bun.DB, mode string) cleanupOwnerOperation {
	t.Helper()
	ctx := testpkg.Ctx(t)
	tenantID := testpkg.Tenant(t)
	_, account := testpkg.CreateTestPersonWithAccount(t, db, "Cleanup", "Cancellation")
	objects := &memoryObjects{objects: map[string][]byte{}}
	fixture := cleanupOwnerOperation{names: []string{"z-first.pdf", "a-second.pdf", "m-third.pdf"}, objects: objects}
	var oldOwnerID int64
	if mode == "folder" {
		files, _, err := api.NewFileStoreTestModule(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db), objects)
		require.NoError(t, err)
		actor := filestorage.Actor{AccountID: account.ID, Permissions: []string{"files:manage"}}
		folder, err := files.CreateFolder(ctx, filestorage.FolderInput{Name: "Cancellation subject", Visibility: "all_staff"}, actor)
		require.NoError(t, err)
		other, err := files.CreateFolder(ctx, filestorage.FolderInput{Name: "Cancellation sentinel", Visibility: "all_staff"}, actor)
		require.NoError(t, err)
		fixture.ownerID, oldOwnerID = folder.ID, other.ID
		fixture.run = func(ctx context.Context) error { return files.DeleteFolder(ctx, folder.ID, actor) }
	} else {
		fixture.ownerID = testpkg.CreateTestParentAnnouncement(t, db, account.ID, "Cancellation subject").ID
		oldOwnerID = testpkg.CreateTestParentAnnouncement(t, db, account.ID, "Cancellation sentinel").ID
		purger, err := compose.NewAttachmentPurger(compose.AttachmentPurgerDependencies{DB: db, Objects: objects, Observe: func(compose.Observation) {}})
		require.NoError(t, err)
		fixture.run = func(ctx context.Context) error {
			return purger.QueueAttachmentCleanupForAnnouncement(ctx, fixture.ownerID)
		}
	}
	old := time.Date(2020, 1, 2, 3, 4, 5, 0, time.UTC)
	require.NoError(t, testpkg.WithTenantTx(t, ctx, db, tenantID, func(txCtx context.Context, tx bun.Tx) error {
		for _, name := range fixture.names {
			if err := cleanupProbeInsertDocument(txCtx, tx, mode, tenantID, fixture.ownerID, account.ID, name, false, false, old); err != nil {
				return err
			}
			if err := cleanupProbeInsertIntent(txCtx, tx, mode, tenantID, oldOwnerID, name, old); err != nil {
				return err
			}
		}
		return nil
	}))
	for _, name := range fixture.names {
		_, err := objects.SavePrivate(ctx, cleanupProbeKind(mode), tenantID, name, bytes.NewBufferString("%PDF-cancellation"))
		require.NoError(t, err)
	}
	return fixture
}

// pgdriver v1.3.0 does not interrupt an in-flight query when a cancellable
// context without a new deadline is canceled. Releasing the observed blocker
// finishes that statement; the scalar loop must check cancellation before the
// third queue call. This pins persistent sequence effects as well as errors.
func TestCleanupQueueOwnerCancellationStopsBeforeThirdIntent(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"folder", "attachments"} {
		t.Run(mode, func(t *testing.T) {
			testpkg.OwnTenant(t)
			db := testpkg.SetupIsolatedTestDB(t)
			fixture := cleanupOwnerFixture(t, db, mode)
			ctx, cancel := context.WithTimeout(testpkg.Ctx(t), 10*time.Second)
			defer cancel()
			tenantID := testpkg.Tenant(t)
			before, err := cleanupProbeIntentRows(ctx, db, mode, tenantID, fixture.names)
			require.NoError(t, err)
			metadataBefore, err := cleanupProbeMetadataRows(ctx, db, mode, tenantID, fixture.ownerID)
			require.NoError(t, err)
			objectsBefore := cleanupProbeObjects(fixture.objects)
			start := cleanupProtocolSequence(t, ctx, db, mode)
			blocker := cleanupBeginTransaction(t, ctx, db, tenantID)
			cleanupLockName(t, blocker, mode, fixture.names[1])
			contender := cleanupBeginTransaction(t, ctx, db, tenantID)
			var transactionTime time.Time
			require.NoError(t, contender.tx.NewRaw("SELECT transaction_timestamp()").Scan(contender.ctx, &transactionTime))
			queryCtx, cancelQuery := context.WithCancel(contender.ctx)
			defer cancelQuery()
			counter := testpkg.CaptureQueriesForContext(t, db)
			defer counter.Stop()
			done := make(chan error, 1)
			go func() { done <- fixture.run(counter.Context(queryCtx)) }()
			cleanupAwaitBlocker(t, ctx, db, contender.pid, blocker.pid)
			duringSequence := cleanupProtocolSequence(t, ctx, db, mode)
			cancelQuery()
			require.NoError(t, blocker.tx.Commit())
			callErr := cleanupReceive(t, ctx, done)
			counter.Stop()
			end := cleanupProtocolSequence(t, ctx, db, mode)
			partial, readErr := cleanupProbeIntentRows(contender.ctx, contender.tx, mode, tenantID, fixture.names)
			metadataPartial, metadataErr := cleanupProbeMetadataRows(contender.ctx, contender.tx, mode, tenantID, fixture.ownerID)
			var chain []string
			for next := callErr; next != nil; next = errors.Unwrap(next) {
				chain = append(chain, fmt.Sprintf("%T: %s", next, next))
			}
			t.Logf("mode=%s start=%d blocked_sequence=%d end=%d error_chain=%q partial=%q before=%q SQL=%q read_error=%v metadata_error=%v", mode, start, duringSequence, end, chain, partial, before, counter.Queries(), readErr, metadataErr)
			// Always restore the ambient transaction before any semantic assertion,
			// including when a candidate incorrectly returns success.
			require.NoError(t, contender.tx.Rollback())
			after, err := cleanupProbeIntentRows(ctx, db, mode, tenantID, fixture.names)
			require.NoError(t, err)
			require.NoError(t, readErr)
			require.NoError(t, metadataErr)
			require.Equal(t, before, after, "rollback restores full intent rows")
			require.Equal(t, metadataBefore, metadataPartial, "no metadata deletion can precede completion of the queue")
			require.Equal(t, objectsBefore, cleanupProbeObjects(fixture.objects))
			require.Equal(t, start+2, duringSequence)
			require.Equal(t, start+2, end, "cancellation must not consume the third sequence ID")
			require.ErrorIs(t, callErr, context.Canceled)
			kind := "file"
			if mode == "attachments" {
				kind = "attachment"
			}
			require.EqualError(t, callErr, "file storage postgres: queue "+kind+" cleanup: context canceled")
			require.Len(t, partial, 3)
			require.Equal(t, before[2], partial[2], "the third intent stays byte-identical even before rollback")
			for _, row := range partial[:2] {
				var intent struct {
					OwnerID   int64      `json:"owner_id"`
					CleanedAt *time.Time `json:"cleaned_at"`
					CreatedAt time.Time  `json:"created_at"`
					UpdatedAt time.Time  `json:"updated_at"`
				}
				require.NoError(t, json.Unmarshal([]byte(row), &intent))
				require.Equal(t, fixture.ownerID, intent.OwnerID)
				require.Nil(t, intent.CleanedAt)
				require.True(t, intent.CreatedAt.Equal(time.Date(2020, 1, 2, 3, 4, 5, 0, time.UTC)))
				require.True(t, intent.UpdatedAt.Equal(transactionTime))
			}
			for _, query := range counter.Queries() {
				require.False(t, strings.Contains(strings.ToLower(query), "insert into audit."), "no audit append on canceled cleanup")
			}
		})
	}
}

type cleanupHeldTransaction struct {
	ctx context.Context
	tx  bun.Tx
	pid int
}

func cleanupBeginTransaction(t *testing.T, ctx context.Context, db *bun.DB, tenantID int64) cleanupHeldTransaction {
	t.Helper()
	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	t.Cleanup(func() {
		err := tx.Rollback()
		require.True(t, err == nil || errors.Is(err, sql.ErrTxDone), "rollback own transaction: %v", err)
	})
	_, err = tx.ExecContext(ctx, "SELECT set_config('role', 'phoenix_tenant', true), set_config('app.current_tenant_id', ?, true)", fmt.Sprint(tenantID))
	require.NoError(t, err)
	var pid int
	require.NoError(t, tx.NewRaw("SELECT pg_backend_pid()").Scan(ctx, &pid))
	return cleanupHeldTransaction{ctx: testpkg.ContextWithTransaction(testpkg.ContextForTenant(ctx, tenantID), tx), tx: tx, pid: pid}
}

func cleanupLockName(t *testing.T, holder cleanupHeldTransaction, mode, name string) {
	t.Helper()
	query := "SELECT id FROM " + cleanupProtocolTable(mode) + " WHERE tenant_id = current_setting('app.current_tenant_id')::bigint AND filename_stored = ?"
	if mode == "folder" {
		query += " AND object_kind = 'files'"
	}
	var id int64
	require.NoError(t, holder.tx.NewRaw(query+" FOR UPDATE", name).Scan(holder.ctx, &id))
	require.Positive(t, id)
}

// The query observes PostgreSQL's actual wait graph, never a guessed relation
// between locks or a timing delay. Both execution and channels have deadlines.
func cleanupAwaitBlocker(t *testing.T, ctx context.Context, db *bun.DB, waiter, blocker int) {
	t.Helper()
	for ctx.Err() == nil {
		var blocked bool
		require.NoError(t, db.NewRaw("SELECT ?::int = ANY(pg_blocking_pids(?))", blocker, waiter).Scan(ctx, &blocked))
		if blocked {
			t.Logf("lock schedule: waiter=%d blocked_by=%d", waiter, blocker)
			return
		}
	}
	t.Fatalf("waiter %d never blocked on %d: %v", waiter, blocker, ctx.Err())
}

func cleanupReceive(t *testing.T, ctx context.Context, result <-chan error) error {
	t.Helper()
	select {
	case err := <-result:
		return err
	case <-ctx.Done():
		t.Fatalf("cleanup worker did not finish: %v", ctx.Err())
		return ctx.Err()
	}
}
