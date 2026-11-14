package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-ogs/project-phoenix/modules/workforce/internal/domain"
	"github.com/moto-ogs/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// LockShiftType takes the row lock a staff_shifts or staff_shift_series FK
// check conflicts with, so the reference count read after it stays true
// until the removal commits (#6845).
func (s *Store) LockShiftType(ctx context.Context, id int64) (bool, domain.OperationStats, error) {
	db, scope, err := s.database(ctx)
	if err != nil {
		return false, domain.OperationStats{}, err
	}
	// The shift type is a leaf of the lock order: nothing is locked after it.
	if err := tenant.OrderLock(ctx, tenant.LockLeaf, tableShiftTypes); err != nil {
		return false, domain.OperationStats{}, err
	}
	row := &shiftTypeRow{}
	query := withTenant(db.NewSelect().
		Model(row).
		ModelTableExpr(tableShiftTypes+` AS "shift_type"`).
		Column("id").
		Where(`"shift_type".id = ?`, id).
		For("UPDATE"), aliasShiftType, scope)
	return scanOne(ctx, query, "lock shift type")
}

// ArchiveShiftType keeps the first archive time when the type is archived
// again.
func (s *Store) ArchiveShiftType(ctx context.Context, id int64) (domain.ShiftType, bool, domain.OperationStats, error) {
	db, scope, err := s.database(ctx)
	if err != nil {
		return domain.ShiftType{}, false, domain.OperationStats{}, err
	}
	row := &shiftTypeRow{}
	query := withTenant(db.NewUpdate().
		Model(row).
		ModelTableExpr(tableShiftTypes+` AS "shift_type"`).
		Set("archived_at = COALESCE(archived_at, NOW())").
		Set("updated_at = NOW()").
		Where(`"shift_type".id = ?`, id).
		Returning("*"), aliasShiftType, scope)
	stats, err := execAffected(ctx, query, "archive shift type")
	if err != nil || stats.Rows != 1 {
		return domain.ShiftType{}, false, stats, err
	}
	return shiftTypeToDomain(*row), true, stats, nil
}

// shiftPlanReferenceRow is one (source, key) count: key is a shift type for
// "shift" and "series", a calendar period for "period".
type shiftPlanReferenceRow struct {
	Source string `bun:"source"`
	Key    int64  `bun:"key"`
	Count  int    `bun:"count"`
}

// shiftTypeReferencesOfOneType counts the shifts and series carrying one type.
const shiftTypeReferencesOfOneType = `(
	SELECT 'shift' AS source, shift_type_id AS key, COUNT(*)::int AS count
	FROM schedule.staff_shifts
	WHERE tenant_id = ? AND shift_type_id = ?
	GROUP BY shift_type_id
	UNION ALL
	SELECT 'series' AS source, shift_type_id AS key, COUNT(*)::int AS count
	FROM schedule.staff_shift_series
	WHERE tenant_id = ? AND shift_type_id = ?
	GROUP BY shift_type_id
) AS refs`

// shiftPlanReferencesOfSchool counts the shifts and series per type and every
// series segment per period, ended ones included: each still holds its
// period through the FK.
const shiftPlanReferencesOfSchool = `(
	SELECT 'shift' AS source, shift_type_id AS key, COUNT(*)::int AS count
	FROM schedule.staff_shifts
	WHERE tenant_id = ? AND shift_type_id IS NOT NULL
	GROUP BY shift_type_id
	UNION ALL
	SELECT 'series' AS source, shift_type_id AS key, COUNT(*)::int AS count
	FROM schedule.staff_shift_series
	WHERE tenant_id = ? AND shift_type_id IS NOT NULL
	GROUP BY shift_type_id
	UNION ALL
	SELECT 'period' AS source, calendar_period_id AS key, COUNT(*)::int AS count
	FROM schedule.staff_shift_series
	WHERE tenant_id = ?
	GROUP BY calendar_period_id
) AS refs`

// CountShiftTypeReferences counts the planned rows carrying one type in one
// statement.
func (s *Store) CountShiftTypeReferences(ctx context.Context, shiftTypeID int64) (domain.ShiftTypeReferences, domain.OperationStats, error) {
	rows, stats, err := s.countShiftPlan(ctx, func(db bun.IDB, tenantID int64) *bun.SelectQuery {
		return db.NewSelect().ColumnExpr("source, key, count").
			TableExpr(shiftTypeReferencesOfOneType, tenantID, shiftTypeID, tenantID, shiftTypeID)
	})
	if err != nil {
		return domain.ShiftTypeReferences{}, stats, err
	}
	return foldShiftPlanReferences(rows).ByShiftType[shiftTypeID], stats, nil
}

// CountShiftPlanReferences reads both planning tables in one UNION ALL, so
// the count costs one round trip however many types and periods the school
// has.
func (s *Store) CountShiftPlanReferences(ctx context.Context) (domain.ShiftPlanReferences, domain.OperationStats, error) {
	rows, stats, err := s.countShiftPlan(ctx, func(db bun.IDB, tenantID int64) *bun.SelectQuery {
		return db.NewSelect().ColumnExpr("source, key, count").
			TableExpr(shiftPlanReferencesOfSchool, tenantID, tenantID, tenantID)
	})
	if err != nil {
		return domain.ShiftPlanReferences{}, stats, err
	}
	return foldShiftPlanReferences(rows), stats, nil
}

// countShiftPlan runs one tenant-scoped count statement.
func (s *Store) countShiftPlan(ctx context.Context, build func(bun.IDB, int64) *bun.SelectQuery) ([]shiftPlanReferenceRow, domain.OperationStats, error) {
	db, scope, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	if err := scope.Validate(); err != nil {
		return nil, domain.OperationStats{}, err
	}
	var rows []shiftPlanReferenceRow
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err = build(db, scope.TenantID).Scan(ctx, &rows)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return nil, stats, fmt.Errorf("workforce postgres: count shift plan references: %w", err)
	}
	stats.Rows = int64(len(rows))
	return rows, stats, nil
}

func foldShiftPlanReferences(rows []shiftPlanReferenceRow) domain.ShiftPlanReferences {
	result := domain.ShiftPlanReferences{
		ByShiftType:            map[int64]domain.ShiftTypeReferences{},
		SeriesByCalendarPeriod: map[int64]int{},
	}
	for _, row := range rows {
		switch row.Source {
		case "period":
			result.SeriesByCalendarPeriod[row.Key] = row.Count
		case "shift":
			entry := result.ByShiftType[row.Key]
			entry.Shifts = row.Count
			result.ByShiftType[row.Key] = entry
		default:
			entry := result.ByShiftType[row.Key]
			entry.Series = row.Count
			result.ByShiftType[row.Key] = entry
		}
	}
	return result
}
