//go:build testhooks

package postgres

import (
	"context"

	"github.com/uptrace/bun"
)

func (s *Store) DeleteSystemEmailForTest(ctx context.Context, id int64) error {
	return s.inAdminTransaction(ctx, func(txCtx context.Context, db bun.IDB) error {
		_, err := db.NewRaw(`DELETE FROM platform.email_outbox WHERE id = ? AND tenant_id IS NULL`, id).Exec(txCtx)
		return err
	})
}
