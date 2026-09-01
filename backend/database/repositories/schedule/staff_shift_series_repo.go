package schedule

import (
	"context"
	"database/sql"
	"errors"

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
// offboarding). Already tighter bounds are kept. A cap at or before
// valid_from clamps to valid_from — an empty segment that materializes
// nothing — instead of violating chk_staff_shift_series_validity.
func (r *StaffShiftSeriesRepository) CapValidUntil(ctx context.Context, id int64, until timezone.Date) error {
	query := base.GetDB(ctx, r.db).NewUpdate().
		Model((*schedule.StaffShiftSeries)(nil)).
		ModelTableExpr(tableExprStaffShiftSeriesAsSeries).
		Set("valid_until = GREATEST(valid_from, ?)", until).
		Where(`"staff_shift_series".id = ?`, id).
		Where(`("staff_shift_series".valid_until IS NULL OR "staff_shift_series".valid_until > ?)`, until)

	query = base.WithTenantFilter(ctx, query, "staff_shift_series")

	if _, err := query.Exec(ctx); err != nil {
		return &modelBase.DatabaseError{Op: "cap staff shift series valid_until", Err: base.TranslateNotFound(err)}
	}
	return nil
}

// CapAllByStaffID bounds every series segment of one staff member at the
// exclusive date (staff offboarding). Segments that already end earlier keep
// their tighter bound; segments starting on or after the cap collapse to an
// empty range (valid_until = valid_from) and generate nothing on future
// splits.
func (r *StaffShiftSeriesRepository) CapAllByStaffID(ctx context.Context, staffID int64, until timezone.Date) (int64, error) {
	query := base.GetDB(ctx, r.db).NewUpdate().
		Model((*schedule.StaffShiftSeries)(nil)).
		ModelTableExpr(tableExprStaffShiftSeriesAsSeries).
		Set("valid_until = GREATEST(valid_from, ?)", until).
		Where(`"staff_shift_series".staff_id = ?`, staffID).
		Where(`("staff_shift_series".valid_until IS NULL OR "staff_shift_series".valid_until > ?)`, until)

	query = base.WithTenantFilter(ctx, query, "staff_shift_series")

	result, err := query.Exec(ctx)
	if err != nil {
		return 0, &modelBase.DatabaseError{Op: "cap staff shift series by staff", Err: base.TranslateNotFound(err)}
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return 0, &modelBase.DatabaseError{Op: "cap staff shift series by staff", Err: base.TranslateNotFound(err)}
	}
	return rows, nil
}

// FindOverlappingInLineage returns another chronological segment in one split
// lineage that is active on or after from. Root rows store a NULL series_root_id,
// so resolve both roots and successors through COALESCE.
func (r *StaffShiftSeriesRepository) FindOverlappingInLineage(ctx context.Context, rootID, excludeID int64, from timezone.Date) (*schedule.StaffShiftSeries, error) {
	series := new(schedule.StaffShiftSeries)
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(series).
		ModelTableExpr(tableExprStaffShiftSeriesAsSeries).
		Where(`COALESCE("staff_shift_series".series_root_id, "staff_shift_series".id) = ?`, rootID).
		Where(`"staff_shift_series".id != ?`, excludeID).
		Where(`("staff_shift_series".valid_until IS NULL OR "staff_shift_series".valid_until > ?)`, from).
		OrderExpr(`"staff_shift_series".valid_from ASC`).
		Limit(1)

	query = base.WithTenantFilter(ctx, query, "staff_shift_series")
	if err := query.Scan(ctx); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, &modelBase.DatabaseError{Op: "find overlapping staff shift series in lineage", Err: base.TranslateNotFound(err)}
	}
	return series, nil
}
