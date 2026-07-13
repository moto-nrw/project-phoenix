package schedule

import (
	"context"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/uptrace/bun"
)

const tableExprStaffShiftSeriesAsSeries = `schedule.staff_shift_series AS "staff_shift_series"`

// StaffShiftSeriesRepository implements schedule.StaffShiftSeriesRepository
type StaffShiftSeriesRepository struct {
	*base.Repository[*schedule.StaffShiftSeries]
	db *bun.DB
}

// NewStaffShiftSeriesRepository creates a new staff shift series repository
func NewStaffShiftSeriesRepository(db *bun.DB) schedule.StaffShiftSeriesRepository {
	repo := base.NewRepository[*schedule.StaffShiftSeries](db, "schedule.staff_shift_series", "StaffShiftSeries")
	repo.TenantScoped = true
	return &StaffShiftSeriesRepository{Repository: repo, db: db}
}

// CapValidUntil bounds a series segment at the exclusive date (split / end /
// offboarding). Already tighter bounds are kept.
func (r *StaffShiftSeriesRepository) CapValidUntil(ctx context.Context, id int64, until timezone.Date) error {
	query := base.GetDB(ctx, r.db).NewUpdate().
		Model((*schedule.StaffShiftSeries)(nil)).
		ModelTableExpr(tableExprStaffShiftSeriesAsSeries).
		Set("valid_until = ?", until).
		Where(`"staff_shift_series".id = ?`, id).
		Where(`("staff_shift_series".valid_until IS NULL OR "staff_shift_series".valid_until > ?)`, until)

	query = base.WithTenantFilter(ctx, query, "staff_shift_series")

	if _, err := query.Exec(ctx); err != nil {
		return &modelBase.DatabaseError{Op: "cap staff shift series valid_until", Err: err}
	}
	return nil
}

// CapAllByStaffID bounds every series segment of one staff member at the
// exclusive date (staff offboarding). Segments that already end earlier keep
// their tighter bound; segments entirely after the cap collapse to an empty
// range and simply generate nothing on future splits.
func (r *StaffShiftSeriesRepository) CapAllByStaffID(ctx context.Context, staffID int64, until timezone.Date) (int64, error) {
	query := base.GetDB(ctx, r.db).NewUpdate().
		Model((*schedule.StaffShiftSeries)(nil)).
		ModelTableExpr(tableExprStaffShiftSeriesAsSeries).
		Set("valid_until = ?", until).
		Where(`"staff_shift_series".staff_id = ?`, staffID).
		Where(`("staff_shift_series".valid_until IS NULL OR "staff_shift_series".valid_until > ?)`, until)

	query = base.WithTenantFilter(ctx, query, "staff_shift_series")

	result, err := query.Exec(ctx)
	if err != nil {
		return 0, &modelBase.DatabaseError{Op: "cap staff shift series by staff", Err: err}
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return 0, &modelBase.DatabaseError{Op: "cap staff shift series by staff", Err: err}
	}
	return rows, nil
}
