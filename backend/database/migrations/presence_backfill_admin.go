package migrations

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/uptrace/bun"
)

func PresenceBackfillTenants(ctx context.Context, db *bun.DB) ([]int64, error) {
	if db == nil {
		return nil, fmt.Errorf("database is required")
	}
	var ids []int64
	err := db.NewRaw(`SELECT id FROM platform.schools ORDER BY id`).Scan(ctx, &ids)
	return ids, err
}

// RestartPresenceBackfill is the pre-cutover rollback: only this tenant's
// targets and batch evidence are cleared; the checkpoint restarts from zero.
// It serializes with batches through the same checkpoint row lock. It must
// not be used once target tables become authoritative in #2762.
func RestartPresenceBackfill(ctx context.Context, db *bun.DB, tenantID int64) error {
	if db == nil || tenantID <= 0 {
		return fmt.Errorf("database and positive tenant ID are required")
	}
	return db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.ExecContext(ctx, `SET LOCAL lock_timeout = '5s'; SET LOCAL statement_timeout = '60s'`); err != nil {
			return err
		}
		state, err := json.Marshal(PresenceBackfillReport{TenantID: tenantID, Phase: "sessions", Pass: 1})
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO active.presence_backfill_checkpoints (tenant_id, state)
   VALUES (?, ?::jsonb) ON CONFLICT (tenant_id) DO NOTHING`, tenantID, string(state)); err != nil {
			return err
		}
		if _, err := readPresenceCheckpoint(ctx, tx, tenantID, true); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM active.activity_session_attendance WHERE tenant_id = ?`, tenantID); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM active.activity_sessions WHERE tenant_id = ?`, tenantID); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM active.presence_backfill_batches WHERE tenant_id = ?`, tenantID); err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, `UPDATE active.presence_backfill_checkpoints SET state = ?::jsonb, updated_at = clock_timestamp() WHERE tenant_id = ?`, string(state), tenantID)
		return err
	})
}
