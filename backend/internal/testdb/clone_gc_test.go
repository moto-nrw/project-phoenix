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

	maint := openSQL(cfg.MaintenanceDSN())
	defer func() { _ = maint.Close() }()
	conn, unlock, err := acquireLifecycleLock(ctx, maint)
	require.NoError(t, err)
	defer unlock()

	name := ClonePrefix + SanitizeRunID("") + "_busy"
	_, err = conn.ExecContext(ctx, `CREATE DATABASE `+quoteIdentifier(name))
	require.NoError(t, err)
	t.Cleanup(func() { dropBareClone(t, cfg, name) })

	blocker, err := maint.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer func() { _ = blocker.Rollback() }()
	var oid int64
	err = blocker.QueryRowContext(ctx, `SELECT oid FROM pg_database WHERE datname = $1 FOR UPDATE`, name).Scan(&oid)
	require.NoError(t, err)

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
