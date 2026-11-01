package parentpostgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/communication"
)

// FindSend serializes one keyed group send before it can create a thread.
// The row index remains the final guard. The lock belongs to this transaction.
func (s *GroupThreadStore) FindSend(ctx context.Context, accountID int64, key string) (*communication.ParentGroupThread, string, error) {
	if key == "" {
		return nil, "", nil
	}
	db, tenantID, err := s.tenantDB(ctx, "find group send")
	if err != nil {
		return nil, "", err
	}
	if _, err := db.ExecContext(ctx, "SELECT pg_advisory_xact_lock(hashtextextended(?, 0))", fmt.Sprintf("group-send:%d:%d:%s", tenantID, accountID, key)); err != nil {
		return nil, "", err
	}
	row := new(groupMessageRow)
	err = db.NewSelect().Model(row).Where("gm.tenant_id = ? AND gm.sender_account_id = ? AND gm.client_message_id = ?", tenantID, accountID, key).Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, "", nil
	}
	if err != nil {
		return nil, "", err
	}
	thread, err := s.FindThread(ctx, row.ThreadID, false)
	return thread, row.Body, err
}
