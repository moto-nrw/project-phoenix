package testdb

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestCloneGCDefersLockedOrphan(t *testing.T) {
	t.Parallel()
	cfg := integrationConfig(t)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	name := ClonePrefix + SanitizeRunID("") + "_busy"
	createBareClone(t, ctx, cfg, name)
	t.Cleanup(func() { dropBareClone(t, cfg, name) })

	maint := openSQL(cfg.MaintenanceDSN())
	defer func() { _ = maint.Close() }()
	blocker, err := maint.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer func() { _ = blocker.Rollback() }()
	_, err = blocker.ExecContext(ctx, `SELECT oid FROM pg_database WHERE datname = $1 FOR UPDATE`, name)
	require.NoError(t, err)

	conn, unlock, err := acquireLifecycleLock(ctx, maint)
	require.NoError(t, err)
	defer unlock()

	gcCtx, gcCancel := context.WithTimeout(ctx, 3*time.Second)
	defer gcCancel()
	dropped, err := gcLocked(gcCtx, conn, cfg.TemplateName(), ClonePrefix+RunID()+"_")
	require.NoError(t, err)
	require.NotContains(t, dropped, name)
	require.True(t, databaseExists(t, ctx, cfg, name), "busy orphan must be left for a later cleanup pass")

	require.NoError(t, blocker.Rollback())
	dropped, err = gcLocked(ctx, conn, cfg.TemplateName(), ClonePrefix+RunID()+"_")
	require.NoError(t, err)
	require.Contains(t, dropped, name)
	require.False(t, databaseExists(t, ctx, cfg, name), "cleanup must collect the orphan once its lock is released")
}
