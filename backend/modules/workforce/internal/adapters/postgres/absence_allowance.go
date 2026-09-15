package postgres

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/workforce/internal/domain"
	"github.com/uptrace/bun"
)

func (s *Store) StaffAbsenceTypeEntitlement(ctx context.Context, staffID, absenceTypeID int64, year int) (float64, bool, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return 0, false, domain.OperationStats{}, err
	}
	var days float64
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err = db.NewSelect().TableExpr("active.staff_absence_type_allowances").Column("entitled_days").
		Where("tenant_id = ?", tenantID).Where("staff_id = ?", staffID).
		Where("absence_type_id = ?", absenceTypeID).Where("year = ?", year).Limit(1).Scan(ctx, &days)
	stats.StatementDuration = time.Since(started)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, false, stats, nil
	}
	if err != nil {
		return 0, false, stats, err
	}
	stats.Rows = 1
	return days, true, stats, nil
}

type staffAbsenceAllowanceRow struct {
	bun.BaseModel `bun:"table:active.staff_absence_type_allowances"`
	TenantID      int64   `bun:"tenant_id"`
	StaffID       int64   `bun:"staff_id"`
	AbsenceTypeID int64   `bun:"absence_type_id"`
	Year          int     `bun:"year"`
	EntitledDays  float64 `bun:"entitled_days"`
}

func (s *Store) UpsertStaffAbsenceTypeAllowance(ctx context.Context, input domain.SetAbsenceTypeAllowance) (domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.OperationStats{}, err
	}
	row := staffAbsenceAllowanceRow{TenantID: tenantID, StaffID: input.StaffID, AbsenceTypeID: input.AbsenceTypeID, Year: input.Year, EntitledDays: input.EntitledDays}
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	_, err = db.NewInsert().Model(&row).On("CONFLICT (tenant_id, staff_id, absence_type_id, year) DO UPDATE").
		Set("entitled_days = EXCLUDED.entitled_days").Set("updated_at = CURRENT_TIMESTAMP").Exec(ctx)
	stats.StatementDuration = time.Since(started)
	if err == nil {
		stats.Rows = 1
	}
	return stats, err
}

type staffAbsenceAllowanceChangeRow struct {
	bun.BaseModel   `bun:"table:active.staff_absence_type_allowance_changes"`
	TenantID        int64    `bun:"tenant_id"`
	StaffID         int64    `bun:"staff_id"`
	AbsenceTypeID   int64    `bun:"absence_type_id"`
	Year            int      `bun:"year"`
	OldEntitledDays *float64 `bun:"old_entitled_days"`
	NewEntitledDays float64  `bun:"new_entitled_days"`
	Reason          string   `bun:"reason"`
	ChangedBy       int64    `bun:"changed_by"`
}

func (s *Store) RecordStaffAbsenceTypeAllowanceChange(ctx context.Context, input domain.SetAbsenceTypeAllowance, oldDays *float64) (domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.OperationStats{}, err
	}
	row := staffAbsenceAllowanceChangeRow{
		TenantID: tenantID, StaffID: input.StaffID, AbsenceTypeID: input.AbsenceTypeID, Year: input.Year,
		OldEntitledDays: oldDays, NewEntitledDays: input.EntitledDays, Reason: strings.TrimSpace(input.Reason), ChangedBy: input.ChangedBy,
	}
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	_, err = db.NewInsert().Model(&row).Exec(ctx)
	stats.StatementDuration = time.Since(started)
	if err == nil {
		stats.Rows = 1
	}
	return stats, err
}
