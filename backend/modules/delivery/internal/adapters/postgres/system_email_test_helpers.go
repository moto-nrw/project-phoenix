//go:build testhooks

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

func (s *Store) DeleteSystemEmailForTest(ctx context.Context, id int64) error {
	return s.adminTx(ctx, func(txCtx context.Context, db bun.IDB) error {
		_, err := db.NewRaw(`DELETE FROM platform.email_outbox WHERE id = ? AND tenant_id IS NULL`, id).Exec(txCtx)
		return err
	})
}

// ClaimEmail is the same exact-entry acquisition for mail transport adapters.
func (s *Store) ClaimEmail(ctx context.Context, tenantID, id int64, now, expires time.Time) (domain.Intent, bool, error) {
	var row intentRow
	var acquired bool
	var scope any = tenantID
	if tenantID == 0 {
		scope = nil
	}
	err := s.adminTx(ctx, func(txCtx context.Context, db bun.IDB) error {
		err := db.NewRaw(`UPDATE platform.email_outbox
   SET status = 'claimed', lease_token = uuid_generate_v4()::text, lease_expires_at = ?, updated_at = ?
   WHERE COALESCE(tenant_id, 0) = COALESCE(?, 0) AND id = ? AND
    ((status = 'pending' AND next_retry_at <= ?) OR
     (status = 'claimed' AND lease_expires_at <= ? AND lease_token NOT LIKE 'dispatch:%'))
   RETURNING *`, expires, now, scope, id, now, now).Scan(txCtx, &row)
		if !errors.Is(err, sql.ErrNoRows) {
			acquired = err == nil
			return err
		}
		return db.NewRaw(`SELECT * FROM platform.email_outbox WHERE COALESCE(tenant_id, 0) = COALESCE(?, 0) AND id = ?`, scope, id).Scan(txCtx, &row)
	})
	if err != nil {
		return domain.Intent{}, false, fmt.Errorf("delivery: claim exact email: %w", err)
	}
	return toDomainRows(domain.TransportEmail, []intentRow{row})[0], acquired, nil
}
