package schedule

import (
	"context"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

const tableExprStaffShiftSeriesExceptions = `schedule.staff_shift_series_exceptions AS "staff_shift_series_exception"`

// StaffShiftSeriesExceptionRepository implements
// schedule.StaffShiftSeriesExceptionRepository
type StaffShiftSeriesExceptionRepository struct {
	db *bun.DB
}

// NewStaffShiftSeriesExceptionRepository creates a new exception repository
func NewStaffShiftSeriesExceptionRepository(db *bun.DB) schedule.StaffShiftSeriesExceptionRepository {
	return &StaffShiftSeriesExceptionRepository{db: db}
}

// Create records one deliberately removed occurrence of a series. Recording
// the same source slot again is a successful no-op: a detached row may be
// moved away, back, and away again while still referring to that one slot.
func (r *StaffShiftSeriesExceptionRepository) Create(ctx context.Context, exception *schedule.StaffShiftSeriesException) error {
	base.EnsureTenantID(ctx, exception)
	_, err := base.GetDB(ctx, r.db).NewInsert().
		Model(exception).
		ModelTableExpr("schedule.staff_shift_series_exceptions").
		On("CONFLICT (tenant_id, series_id, date) DO NOTHING").
		Exec(ctx)
	if err != nil {
		return &modelBase.DatabaseError{Op: "create staff shift series exception", Err: err}
	}
	return nil
}

// FindDatesBySeriesID returns the excepted dates of one series.
func (r *StaffShiftSeriesExceptionRepository) FindDatesBySeriesID(ctx context.Context, seriesID int64) ([]timezone.Date, error) {
	type exceptionDateRow struct {
		Date timezone.Date `bun:"date,type:date"`
	}
	var rows []exceptionDateRow
	query := base.GetDB(ctx, r.db).NewSelect().
		TableExpr(tableExprStaffShiftSeriesExceptions).
		ColumnExpr(`"staff_shift_series_exception".date`).
		Where(`"staff_shift_series_exception".series_id = ?`, seriesID).
		OrderExpr(`"staff_shift_series_exception".date ASC`)

	query = base.WithTenantFilter(ctx, query, "staff_shift_series_exception")
	if err := query.Scan(ctx, &rows); err != nil {
		return nil, &modelBase.DatabaseError{Op: "find staff shift series exception dates", Err: err}
	}
	dates := make([]timezone.Date, 0, len(rows))
	for _, row := range rows {
		dates = append(dates, row.Date)
	}
	return dates, nil
}

// RepointToSeriesFrom moves exceptions on or after from to the successor
// series created by a split, so removed occurrences stay removed.
func (r *StaffShiftSeriesExceptionRepository) RepointToSeriesFrom(ctx context.Context, fromSeriesID, toSeriesID int64, from timezone.Date) (int64, error) {
	query := base.GetDB(ctx, r.db).NewUpdate().
		Table("schedule.staff_shift_series_exceptions").
		Set("series_id = ?", toSeriesID).
		Where("series_id = ?", fromSeriesID).
		Where("date >= ?", from)

	if tenantID := tenant.FromContext(ctx); tenantID > 0 {
		query = query.Where("tenant_id = ?", tenantID)
	}

	result, err := query.Exec(ctx)
	if err != nil {
		return 0, &modelBase.DatabaseError{Op: "repoint staff shift series exceptions", Err: err}
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return 0, &modelBase.DatabaseError{Op: "repoint staff shift series exceptions", Err: err}
	}
	return rows, nil
}
