package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/delivery/internal/domain"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
)

// realtimeNotificationRow maps platform.realtime_notifications (#5504).
type realtimeNotificationRow struct {
	bun.BaseModel `bun:"table:platform.realtime_notifications"`

	ID            int64           `bun:"id,pk,autoincrement"`
	TenantID      int64           `bun:"tenant_id,notnull"`
	Scope         string          `bun:"scope,notnull"`
	ActiveGroupID string          `bun:"active_group_id,notnull"`
	AccountIDs    []int64         `bun:"account_ids,array,notnull"`
	StaffPortal   bool            `bun:"staff_portal,notnull"`
	SchoolPortal  bool            `bun:"school_portal,notnull"`
	Content       json.RawMessage `bun:"content,type:jsonb,notnull"`
}

const deleteExpiredRealtimeNotificationsSQL = `DELETE FROM platform.realtime_notifications
	WHERE tenant_id = ? AND created_at < ?`

func (s *Store) SaveRealtimeNotification(ctx context.Context, notification domain.RealtimeNotification) (int64, error) {
	db, err := s.tenantDB(ctx, notification.TenantID)
	if err != nil {
		return 0, err
	}
	accountIDs := notification.AccountIDs
	if accountIDs == nil {
		accountIDs = []int64{}
	}
	row := realtimeNotificationRow{
		TenantID: notification.TenantID, Scope: notification.Scope, ActiveGroupID: notification.ActiveGroupID,
		AccountIDs: accountIDs, StaffPortal: notification.StaffPortal, SchoolPortal: notification.SchoolPortal,
		Content: notification.Content,
	}
	if _, err := db.NewInsert().Model(&row).ExcludeColumn("id").Returning("id").Exec(ctx); err != nil {
		return 0, fmt.Errorf("delivery postgres: store realtime notification: %w", err)
	}
	return row.ID, nil
}

func (s *Store) LoadRealtimeNotification(ctx context.Context, tenantID, id int64, notBefore time.Time) (domain.RealtimeNotification, bool, error) {
	db, err := s.tenantDB(ctx, tenantID)
	if err != nil {
		return domain.RealtimeNotification{}, false, err
	}
	var row realtimeNotificationRow
	err = db.NewSelect().Model(&row).
		Where("id = ?", id).
		Where("tenant_id = ?", tenantID).
		Where("created_at >= ?", notBefore).
		Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.RealtimeNotification{}, false, nil
	}
	if err != nil {
		return domain.RealtimeNotification{}, false, fmt.Errorf("delivery postgres: load realtime notification: %w", err)
	}
	return domain.RealtimeNotification{
		ID: row.ID, TenantID: row.TenantID, Scope: row.Scope, ActiveGroupID: row.ActiveGroupID,
		AccountIDs: row.AccountIDs, StaffPortal: row.StaffPortal, SchoolPortal: row.SchoolPortal,
		Content: row.Content,
	}, true, nil
}

// CurrentRealtimeRecipients applies the delivery-time role predicates of
// platform.delivery_realtime_recipients, the same as Web Push applies.
func (s *Store) CurrentRealtimeRecipients(ctx context.Context, tenantID int64, recipient string, accountIDs, studentIDs []int64) ([]int64, error) {
	if len(accountIDs) == 0 {
		return nil, nil
	}
	db, err := s.tenantDB(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	query := db.NewSelect().
		ColumnExpr(`account_id`).
		TableExpr(`platform.delivery_realtime_recipients`).
		Where(`tenant_id = ?`, tenantID).
		Where(`account_id IN (?)`, bun.List(accountIDs)).
		Where(`active`)
	switch recipient {
	case domain.RealtimeRecipientStaff:
		query = query.Where(`has_staff_role`)
	case domain.RealtimeRecipientSchool:
		query = query.Where(`has_staff_role AND has_school_role`)
	case domain.RealtimeRecipientGuardian:
		query = query.Where(`has_guardian_role`)
		if len(studentIDs) > 0 {
			query = query.Where(`guardian_student_ids && ?::BIGINT[]`, pgdialect.Array(studentIDs))
		}
	default:
		return nil, fmt.Errorf("delivery postgres: unknown realtime recipient %q", recipient)
	}
	var current []int64
	if err := query.Scan(ctx, &current); err != nil {
		return nil, fmt.Errorf("delivery postgres: current realtime recipients: %w", err)
	}
	return current, nil
}

func (s *Store) DeleteExpiredRealtimeNotifications(ctx context.Context, tenantID int64, before time.Time) (int, error) {
	db, err := s.tenantDB(ctx, tenantID)
	if err != nil {
		return 0, err
	}
	return affected("delete expired realtime notifications")(db.ExecContext(ctx, deleteExpiredRealtimeNotificationsSQL, tenantID, before))
}
