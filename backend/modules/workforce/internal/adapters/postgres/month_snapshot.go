package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/workforce/internal/domain"
	"github.com/uptrace/bun"
)

const tableMonthSnapshots = "active.staff_month_balance_snapshots"
const monthSnapshotAlias = "staff_month_balance_snapshot"
const monthSnapshotOrdinal = `("staff_month_balance_snapshot".year * 12 + "staff_month_balance_snapshot".month)`

type monthSnapshotRow struct {
	bun.BaseModel         `bun:"table:active.staff_month_balance_snapshots,alias:staff_month_balance_snapshot"`
	ID                    int64      `bun:"id,pk,autoincrement"`
	CreatedAt             time.Time  `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt             time.Time  `bun:"updated_at,nullzero,notnull,default:current_timestamp"`
	TenantID              int64      `bun:"tenant_id,notnull"`
	StaffID               int64      `bun:"staff_id,notnull"`
	Year                  int        `bun:"year,notnull"`
	Month                 int        `bun:"month,notnull"`
	ClosingBalanceMinutes int        `bun:"closing_balance_minutes,notnull"`
	CarryInMinutes        int        `bun:"carry_in_minutes,notnull"`
	TargetMinutes         int        `bun:"target_minutes,notnull"`
	ActualMinutes         int        `bun:"actual_minutes,notnull"`
	CreditedMinutes       int        `bun:"credited_minutes,notnull"`
	AdjustmentMinutes     int        `bun:"adjustment_minutes,notnull"`
	ClosedAt              time.Time  `bun:"closed_at,notnull,default:current_timestamp"`
	ClosedBy              int64      `bun:"closed_by,notnull"`
	CloseReason           string     `bun:"close_reason,notnull,default:''"`
	Source                string     `bun:"source,notnull,default:'admin'"`
	ReopenedAt            *time.Time `bun:"reopened_at"`
	ReopenedBy            *int64     `bun:"reopened_by"`
	ReopenReason          string     `bun:"reopen_reason,notnull,default:''"`
}

func monthSnapshotToDomain(v monthSnapshotRow) domain.StaffMonthBalanceSnapshot {
	return domain.StaffMonthBalanceSnapshot{
		ID: v.ID, CreatedAt: v.CreatedAt, UpdatedAt: v.UpdatedAt, TenantID: v.TenantID, StaffID: v.StaffID,
		Year: v.Year, Month: v.Month, ClosingBalanceMinutes: v.ClosingBalanceMinutes, CarryInMinutes: v.CarryInMinutes,
		TargetMinutes: v.TargetMinutes, ActualMinutes: v.ActualMinutes, CreditedMinutes: v.CreditedMinutes,
		AdjustmentMinutes: v.AdjustmentMinutes, ClosedAt: v.ClosedAt, ClosedBy: v.ClosedBy, CloseReason: v.CloseReason,
		Source: v.Source, ReopenedAt: v.ReopenedAt, ReopenedBy: v.ReopenedBy, ReopenReason: v.ReopenReason,
	}
}

func monthSnapshotFromDomain(v domain.StaffMonthBalanceSnapshot) monthSnapshotRow {
	return monthSnapshotRow{
		ID: v.ID, CreatedAt: v.CreatedAt, UpdatedAt: v.UpdatedAt, TenantID: v.TenantID, StaffID: v.StaffID,
		Year: v.Year, Month: v.Month, ClosingBalanceMinutes: v.ClosingBalanceMinutes, CarryInMinutes: v.CarryInMinutes,
		TargetMinutes: v.TargetMinutes, ActualMinutes: v.ActualMinutes, CreditedMinutes: v.CreditedMinutes,
		AdjustmentMinutes: v.AdjustmentMinutes, ClosedAt: v.ClosedAt, ClosedBy: v.ClosedBy, CloseReason: v.CloseReason,
		Source: v.Source, ReopenedAt: v.ReopenedAt, ReopenedBy: v.ReopenedBy, ReopenReason: v.ReopenReason,
	}
}

func (s *Store) monthSnapshotDatabase(ctx context.Context) (bun.IDB, int64, error) {
	db, tenantID, err := s.database(ctx)
	if err == nil && tenantID <= 0 {
		err = errors.New("month snapshots: tenant is required")
	}
	return db, tenantID, err
}

func (s *Store) LatestClosedMonth(ctx context.Context, staffID int64, year, month int) (*domain.StaffMonthBalanceSnapshot, domain.OperationStats, error) {
	db, tenantID, err := s.monthSnapshotDatabase(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	row := new(monthSnapshotRow)
	query := withTenant(db.NewSelect().Model(row).ModelTableExpr(tableMonthSnapshots+` AS "staff_month_balance_snapshot"`).
		Where(`"staff_month_balance_snapshot".staff_id = ?`, staffID).
		Where(`"staff_month_balance_snapshot".reopened_at IS NULL`).
		Where(monthSnapshotOrdinal+" <= ?", year*12+month).OrderExpr(monthSnapshotOrdinal+" DESC").Limit(1), monthSnapshotAlias, tenantID)
	found, stats, err := scanOne(ctx, query, "get latest month balance snapshot")
	if err != nil || !found {
		return nil, stats, err
	}
	result := monthSnapshotToDomain(*row)
	return &result, stats, nil
}

func (s *Store) ClosedMonthSnapshots(ctx context.Context, year, month int) ([]*domain.StaffMonthBalanceSnapshot, domain.OperationStats, error) {
	db, tenantID, err := s.monthSnapshotDatabase(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	var rows []monthSnapshotRow
	query := withTenant(db.NewSelect().Model(&rows).ModelTableExpr(tableMonthSnapshots+` AS "staff_month_balance_snapshot"`).
		Where(`"staff_month_balance_snapshot".year = ?`, year).Where(`"staff_month_balance_snapshot".month = ?`, month).
		Where(`"staff_month_balance_snapshot".reopened_at IS NULL`).OrderExpr(`"staff_month_balance_snapshot".staff_id ASC`), monthSnapshotAlias, tenantID)
	stats, err := scanAll(ctx, query, "get month balance snapshots by month")
	if err != nil {
		return nil, stats, err
	}
	var result []*domain.StaffMonthBalanceSnapshot
	for _, row := range rows {
		value := monthSnapshotToDomain(row)
		result = append(result, &value)
	}
	return result, stats, nil
}

func (s *Store) RecordClosedMonth(ctx context.Context, value domain.StaffMonthBalanceSnapshot) (domain.StaffMonthBalanceSnapshot, domain.OperationStats, error) {
	db, tenantID, err := s.monthSnapshotDatabase(ctx)
	if err != nil {
		return domain.StaffMonthBalanceSnapshot{}, domain.OperationStats{}, err
	}
	row := monthSnapshotFromDomain(value)
	row.TenantID = tenantID
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	_, err = db.NewInsert().Model(&row).ModelTableExpr(tableMonthSnapshots).Returning("*").Exec(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return domain.StaffMonthBalanceSnapshot{}, stats, fmt.Errorf("record closed month: %w", err)
	}
	stats.Rows = 1
	return monthSnapshotToDomain(row), stats, nil
}

func (s *Store) ClosedMonthSnapshotsForStaff(ctx context.Context, staffIDs []int64) ([]*domain.StaffMonthBalanceSnapshot, domain.OperationStats, error) {
	db, tenantID, err := s.monthSnapshotDatabase(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	if len(staffIDs) == 0 {
		return nil, domain.OperationStats{}, nil
	}
	var rows []monthSnapshotRow
	query := withTenant(db.NewSelect().Model(&rows).ModelTableExpr(tableMonthSnapshots+` AS "staff_month_balance_snapshot"`).
		Where(`"staff_month_balance_snapshot".staff_id IN (?)`, bun.List(staffIDs)).
		Where(`"staff_month_balance_snapshot".reopened_at IS NULL`), monthSnapshotAlias, tenantID)
	stats, err := scanAll(ctx, query, "prefetch month balance snapshots")
	if err != nil {
		return nil, stats, err
	}
	var result []*domain.StaffMonthBalanceSnapshot
	for _, row := range rows {
		value := monthSnapshotToDomain(row)
		result = append(result, &value)
	}
	return result, stats, nil
}

func (s *Store) ReopenMonthSnapshot(ctx context.Context, id, actorID int64, at time.Time, reason string) (int64, domain.OperationStats, error) {
	db, tenantID, err := s.monthSnapshotDatabase(ctx)
	if err != nil {
		return 0, domain.OperationStats{}, err
	}
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	result, err := db.NewUpdate().Table(tableMonthSnapshots).Set("reopened_at = ?", at).Set("reopened_by = ?", actorID).Set("reopen_reason = ?", reason).
		Where("id = ?", id).Where("tenant_id = ?", tenantID).Exec(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return 0, stats, fmt.Errorf("reopen month snapshot: %w", err)
	}
	affected, err := result.RowsAffected()
	stats.Rows = affected
	return affected, stats, err
}
