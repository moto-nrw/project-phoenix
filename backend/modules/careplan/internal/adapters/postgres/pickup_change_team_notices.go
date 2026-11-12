package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/uptrace/bun"

	"github.com/moto-ogs/project-phoenix/modules/careplan/internal/domain"
	"github.com/moto-ogs/project-phoenix/modules/careplan/internal/ports"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
)

// The unread markers of changed pickup times (#6855): one row per approved
// one-day request of a family and staff member told about it.
const tableExprPickupChangeTeamNotices = `schedule.pickup_change_team_notices AS "pickup_notice"`

type pickupChangeTeamNoticeRow struct {
	bun.BaseModel      `bun:"table:schedule.pickup_change_team_notices,alias:pickup_notice"`
	TenantID           int64        `bun:"tenant_id,notnull"`
	RequestID          int64        `bun:"request_id,pk"`
	RecipientAccountID int64        `bun:"recipient_account_id,pk"`
	PickupDate         calendarDate `bun:"pickup_date,notnull,type:date"`
	InformedAt         time.Time    `bun:"informed_at,nullzero,notnull,default:current_timestamp"`
	ReadAt             *time.Time   `bun:"read_at"`
}

// PickupChangeTeamStore keeps the markers over the caller's tenant
// transaction and reads the child's first name from the student directory.
type PickupChangeTeamStore struct {
	*Store
	directory StudentDirectory
}

var _ ports.PickupChangeTeamStore = (*PickupChangeTeamStore)(nil)

// NewPickupChangeTeamStore binds the markers to the store and the directory.
func NewPickupChangeTeamStore(store *Store, directory StudentDirectory) *PickupChangeTeamStore {
	if store == nil || directory == nil {
		panic("care plan pickup change notices: store and student directory are required")
	}
	return &PickupChangeTeamStore{Store: store, directory: directory}
}

// UpsertPickupChangeNotices writes all markers in one statement. Telling the
// team again about the same request (a corrected decision) sets them unread.
func (s *PickupChangeTeamStore) UpsertPickupChangeNotices(ctx context.Context, requestID int64, date calendar.Date, recipientAccountIDs []int64) (domain.OperationStats, error) {
	if len(recipientAccountIDs) == 0 {
		return domain.OperationStats{}, nil
	}
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.OperationStats{}, err
	}
	if tenantID <= 0 {
		return domain.OperationStats{}, fmt.Errorf("care plan postgres: upsert pickup change notices: tenant is required")
	}
	rows := make([]pickupChangeTeamNoticeRow, 0, len(recipientAccountIDs))
	for _, accountID := range recipientAccountIDs {
		rows = append(rows, pickupChangeTeamNoticeRow{
			TenantID: tenantID, RequestID: requestID, RecipientAccountID: accountID, PickupDate: calendarDate(date),
		})
	}
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	result, err := db.NewInsert().
		Model(&rows).
		ModelTableExpr(tableExprPickupChangeTeamNotices).
		ExcludeColumn("informed_at", "read_at").
		On("CONFLICT (request_id, recipient_account_id) DO UPDATE").
		Set("informed_at = NOW()").
		Set("read_at = NULL").
		Exec(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return stats, fmt.Errorf("care plan postgres: upsert pickup change notices: %w", err)
	}
	if affected, rowsErr := result.RowsAffected(); rowsErr == nil {
		stats.Rows = affected
	}
	return stats, nil
}

type pickupChangeNoticeReadRow struct {
	RequestID  int64           `bun:"request_id"`
	StudentID  int64           `bun:"student_id"`
	FirstName  string          `bun:"first_name"`
	Status     string          `bun:"status"`
	Payload    json.RawMessage `bun:"payload"`
	InformedAt time.Time       `bun:"informed_at"`
}

// ListUnreadPickupChangeNotices joins the marker to its request and the
// child's directory row: a child whose person row is gone has nothing to
// link to and drops out.
func (s *PickupChangeTeamStore) ListUnreadPickupChangeNotices(ctx context.Context, readerAccountID int64, from calendar.Date, limit int) ([]ports.PickupChangeNoticeRow, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	rows := make([]pickupChangeNoticeReadRow, 0)
	query := withTenant(db.NewSelect().TableExpr(tableExprPickupChangeTeamNotices), "pickup_notice", tenantID).
		ColumnExpr(`"pickup_notice".request_id, "request".student_id, "student".first_name, "request".status, "request".payload, "pickup_notice".informed_at`).
		Join(`JOIN schedule.care_schedule_change_requests AS "request" ON "request".tenant_id = "pickup_notice".tenant_id AND "request".id = "pickup_notice".request_id`).
		Join(`JOIN (?) AS "student" ON "student".tenant_id = "request".tenant_id AND "student".id = "request".student_id AND "student".first_name IS NOT NULL`, s.directory.StudentDisplayRows(db, tenantID)).
		Where(`"pickup_notice".recipient_account_id = ?`, readerAccountID).
		Where(`"pickup_notice".read_at IS NULL`).
		Where(`"pickup_notice".pickup_date >= ?`, string(from)).
		Where(`"request".status IN (?, ?)`, requestApproved, requestRejected).
		OrderExpr(`"pickup_notice".informed_at DESC, "pickup_notice".request_id DESC`).
		Limit(int64(limit))
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err = query.Scan(ctx, &rows)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return nil, stats, fmt.Errorf("care plan postgres: list pickup change notices: %w", err)
	}
	stats.Rows = int64(len(rows))
	out := make([]ports.PickupChangeNoticeRow, 0, len(rows))
	for _, row := range rows {
		out = append(out, ports.PickupChangeNoticeRow{
			RequestID: row.RequestID, StudentID: row.StudentID, StudentFirstName: row.FirstName,
			Status: row.Status, Payload: row.Payload, InformedAt: row.InformedAt,
		})
	}
	return out, stats, nil
}

// MarkPickupChangeNoticesRead clears the reader's markers of one child.
func (s *PickupChangeTeamStore) MarkPickupChangeNoticesRead(ctx context.Context, readerAccountID, studentID int64) (domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.OperationStats{}, err
	}
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	result, err := withTenant(db.NewUpdate().TableExpr(tableExprPickupChangeTeamNotices), "pickup_notice", tenantID).
		Set("read_at = NOW()").
		Where(`"pickup_notice".recipient_account_id = ?`, readerAccountID).
		Where(`"pickup_notice".read_at IS NULL`).
		Where(`"pickup_notice".request_id IN (SELECT "request".id FROM schedule.care_schedule_change_requests AS "request" WHERE "request".tenant_id = ? AND "request".student_id = ?)`, tenantID, studentID).
		Exec(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return stats, fmt.Errorf("care plan postgres: mark pickup change notices read: %w", err)
	}
	if affected, rowsErr := result.RowsAffected(); rowsErr == nil {
		stats.Rows = affected
	}
	return stats, nil
}
