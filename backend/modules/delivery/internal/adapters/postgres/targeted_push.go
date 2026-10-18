package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/delivery/internal/domain"
	"github.com/uptrace/bun"
)

// ClaimPush selects exactly the requested school's entry, never another job.
// A committed pre-dispatch lease may be reclaimed by this path or the worker.
func (s *Store) ClaimPush(ctx context.Context, tenantID, id int64, now, expires time.Time) (domain.Intent, bool, error) {
	var row intentRow
	var acquired bool
	err := s.adminTx(ctx, func(txCtx context.Context, db bun.IDB) error {
		err := db.NewRaw(`UPDATE platform.push_outbox
   SET status = 'claimed', lease_token = uuid_generate_v4()::text, lease_expires_at = ?, updated_at = ?
   WHERE tenant_id = ? AND id = ? AND
    ((status = 'pending') OR
     (status = 'claimed' AND lease_expires_at <= ? AND lease_token NOT LIKE 'dispatch:%'))
   RETURNING *`, expires, now, tenantID, id, now).Scan(txCtx, &row)
		if !errors.Is(err, sql.ErrNoRows) {
			acquired = err == nil
			return err
		}
		return db.NewRaw(`SELECT * FROM platform.push_outbox WHERE tenant_id = ? AND id = ?`, tenantID, id).Scan(txCtx, &row)
	})
	if err != nil {
		return domain.Intent{}, false, fmt.Errorf("delivery: claim exact push: %w", err)
	}
	return toDomainRows(domain.TransportPush, []intentRow{row})[0], acquired, nil
}
