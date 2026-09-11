package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const presenceBackfillCheckpointVersion = "1.15.381"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     presenceBackfillCheckpointVersion,
		Description: "Persist resumable Presence backfill checkpoints and batch evidence (#2761)",
		DependsOn:   []string{presenceExpandVersion},
	})
	Migrations.MustRegister(presenceBackfillCheckpointUp, presenceBackfillCheckpointDown)
}

func presenceBackfillCheckpointUp(ctx context.Context, db *bun.DB) error {
	return db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		_, err := tx.ExecContext(ctx, `
   SET LOCAL lock_timeout = '5s';
   CREATE TABLE active.presence_backfill_checkpoints (
    tenant_id BIGINT PRIMARY KEY REFERENCES platform.schools(id),
    state JSONB NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
   );
   CREATE TABLE active.presence_backfill_batches (
    tenant_id BIGINT NOT NULL REFERENCES active.presence_backfill_checkpoints(tenant_id) ON DELETE CASCADE,
    batch_number BIGINT NOT NULL,
    duration_ms DOUBLE PRECISION NOT NULL CHECK (duration_ms >= 0),
    pool_wait_ms DOUBLE PRECISION NOT NULL CHECK (pool_wait_ms >= 0),
    lock_wait_ms DOUBLE PRECISION NOT NULL CHECK (lock_wait_ms >= 0),
    retries INTEGER NOT NULL CHECK (retries >= 0),
    deadlocks INTEGER NOT NULL CHECK (deadlocks >= 0),
    completed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (tenant_id, batch_number)
   );
   REVOKE ALL ON active.presence_backfill_checkpoints, active.presence_backfill_batches FROM phoenix_tenant;
   GRANT SELECT ON active.presence_backfill_checkpoints, active.presence_backfill_batches TO phoenix_tenant;
   GRANT ALL ON active.presence_backfill_checkpoints, active.presence_backfill_batches TO phoenix_admin;
  `)
		if err != nil {
			return fmt.Errorf("create Presence backfill checkpoints: %w", err)
		}
		return provisionTenantRLS(ctx, tx, "active.presence_backfill_checkpoints", "active.presence_backfill_batches")
	})
}

func presenceBackfillCheckpointDown(ctx context.Context, db *bun.DB) error {
	// Never discard an interrupted backfill's checkpoint through schema rollback.
	return db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		_, err := tx.ExecContext(ctx, `
   SET LOCAL lock_timeout = '5s';
   LOCK TABLE active.presence_backfill_checkpoints IN ACCESS EXCLUSIVE MODE;
   DO $$ BEGIN
    IF EXISTS (SELECT FROM active.presence_backfill_checkpoints) THEN
     RAISE EXCEPTION 'Presence backfill checkpoints must be retained; stop the runner instead';
    END IF;
   END $$;
   DROP TABLE active.presence_backfill_batches;
   DROP TABLE active.presence_backfill_checkpoints;
  `)
		return err
	})
}
