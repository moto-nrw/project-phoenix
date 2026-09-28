package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/workforce/internal/domain"
	"github.com/uptrace/bun"
)

const (
	tableStaffTargetOverrides = "config.staff_target_overrides"
	aliasStaffTargetOverride  = "staff_target_override"
)

type staffTargetOverrideRow struct {
	bun.BaseModel `bun:"table:config.staff_target_overrides,alias:staff_target_override"`
	ID            int64        `bun:"id,pk,autoincrement"`
	TenantID      int64        `bun:"tenant_id,notnull"`
	StaffID       int64        `bun:"staff_id,notnull"`
	StartDate     calendarDate `bun:"start_date,notnull,type:date"`
	EndDate       calendarDate `bun:"end_date,notnull,type:date"`
	DailyMinutes  int          `bun:"daily_minutes,notnull"`
	CreatedBy     *int64       `bun:"created_by"`
	CreatedAt     time.Time    `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt     time.Time    `bun:"updated_at,nullzero,notnull,default:current_timestamp"`
}

func (s *Store) ListStaffTargetOverrides(ctx context.Context, query domain.TargetOverrideQuery) ([]domain.StaffTargetOverride, domain.OperationStats, error) {
	if len(query.StaffIDs) == 0 {
		return nil, domain.OperationStats{}, nil
	}
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	rows := []staffTargetOverrideRow{}
	selectQuery := withTenant(db.NewSelect().Model(&rows).
		ModelTableExpr(tableStaffTargetOverrides+` AS "staff_target_override"`).
		Where(`"staff_target_override".staff_id IN (?)`, bun.List(query.StaffIDs)), aliasStaffTargetOverride, tenantID)
	if query.From != "" && query.To != "" {
		selectQuery = selectQuery.
			Where(`"staff_target_override".start_date <= ?`, calendarDate(query.To)).
			Where(`"staff_target_override".end_date >= ?`, calendarDate(query.From))
	}
	selectQuery = selectQuery.OrderExpr(`"staff_target_override".staff_id ASC, "staff_target_override".start_date ASC, "staff_target_override".id ASC`)
	stats, err := scanAll(ctx, selectQuery, "list staff target overrides")
	if err != nil {
		return nil, stats, err
	}
	stats.Rows = int64(len(rows))
	result := make([]domain.StaffTargetOverride, 0, len(rows))
	for _, row := range rows {
		result = append(result, staffTargetOverrideToDomain(row))
	}
	return result, stats, nil
}

func (s *Store) FindStaffTargetOverride(ctx context.Context, staffID, id int64) (domain.StaffTargetOverride, bool, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.StaffTargetOverride{}, false, domain.OperationStats{}, err
	}
	row := &staffTargetOverrideRow{}
	query := withTenant(db.NewSelect().Model(row).
		ModelTableExpr(tableStaffTargetOverrides+` AS "staff_target_override"`).
		Where(`"staff_target_override".id = ?`, id).
		Where(`"staff_target_override".staff_id = ?`, staffID), aliasStaffTargetOverride, tenantID)
	found, stats, err := scanOne(ctx, query, "find staff target override")
	if err != nil || !found {
		return domain.StaffTargetOverride{}, found, stats, err
	}
	return staffTargetOverrideToDomain(*row), true, stats, nil
}

func (s *Store) CreateStaffTargetOverride(ctx context.Context, value domain.StaffTargetOverride) (domain.StaffTargetOverride, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.StaffTargetOverride{}, domain.OperationStats{}, err
	}
	row := staffTargetOverrideFromDomain(value)
	if row.TenantID == 0 {
		row.TenantID = tenantID
	}
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	_, err = db.NewInsert().Model(row).ModelTableExpr(tableStaffTargetOverrides).Returning("*").Exec(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return domain.StaffTargetOverride{}, stats, fmt.Errorf("workforce postgres: insert staff target override: %w", err)
	}
	stats.Rows = 1
	return staffTargetOverrideToDomain(*row), stats, nil
}

func (s *Store) DeleteStaffTargetOverride(ctx context.Context, staffID, id int64) (bool, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return false, domain.OperationStats{}, err
	}
	query := withTenant(db.NewDelete().Model((*staffTargetOverrideRow)(nil)).
		ModelTableExpr(tableStaffTargetOverrides+` AS "staff_target_override"`).
		Where(`"staff_target_override".id = ?`, id).
		Where(`"staff_target_override".staff_id = ?`, staffID), aliasStaffTargetOverride, tenantID)
	stats, err := execAffected(ctx, query, "delete staff target override")
	return err == nil && stats.Rows == 1, stats, err
}

func staffTargetOverrideFromDomain(value domain.StaffTargetOverride) *staffTargetOverrideRow {
	return &staffTargetOverrideRow{
		ID: value.ID, TenantID: value.TenantID, StaffID: value.StaffID,
		StartDate: calendarDate(value.StartDate), EndDate: calendarDate(value.EndDate),
		DailyMinutes: value.DailyMinutes, CreatedBy: value.CreatedBy,
		CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
	}
}

func staffTargetOverrideToDomain(row staffTargetOverrideRow) domain.StaffTargetOverride {
	return domain.StaffTargetOverride{
		ID: row.ID, TenantID: row.TenantID, StaffID: row.StaffID,
		StartDate: string(row.StartDate), EndDate: string(row.EndDate),
		DailyMinutes: row.DailyMinutes, CreatedBy: row.CreatedBy,
		CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}
}
