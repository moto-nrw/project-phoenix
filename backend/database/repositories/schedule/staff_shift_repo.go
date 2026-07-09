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

const tableExprStaffShiftsAsShift = `schedule.staff_shifts AS "staff_shift"`

// StaffShiftRepository implements schedule.StaffShiftRepository
type StaffShiftRepository struct {
	*base.Repository[*schedule.StaffShift]
	db *bun.DB
}

// NewStaffShiftRepository creates a new staff shift repository
func NewStaffShiftRepository(db *bun.DB) schedule.StaffShiftRepository {
	repo := base.NewRepository[*schedule.StaffShift](db, "schedule.staff_shifts", "StaffShift")
	repo.TenantScoped = true
	return &StaffShiftRepository{Repository: repo, db: db}
}

// normalizeWallClock strips the driver-arbitrary year anchor from scanned
// TIME columns so callers can compare and format the wall-clock values.
func normalizeShiftWallClock(shifts []*schedule.StaffShift) {
	for _, s := range shifts {
		s.StartTime = timezone.WallClock(s.StartTime)
		s.EndTime = timezone.WallClock(s.EndTime)
	}
}

// FindByDateRange returns all shifts with start <= date <= end for the
// current tenant, ordered by date, staff, start time.
func (r *StaffShiftRepository) FindByDateRange(ctx context.Context, start, end timezone.Date) ([]*schedule.StaffShift, error) {
	var shifts []*schedule.StaffShift
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&shifts).
		ModelTableExpr(tableExprStaffShiftsAsShift).
		Where(`"staff_shift".date >= ?`, start).
		Where(`"staff_shift".date <= ?`, end).
		OrderExpr(`"staff_shift".date ASC`).
		OrderExpr(`"staff_shift".staff_id ASC`).
		OrderExpr(`"staff_shift".start_time ASC`)

	query = base.WithTenantFilter(ctx, query, "staff_shift")

	if err := query.Scan(ctx); err != nil {
		return nil, &modelBase.DatabaseError{Op: "find staff shifts by date range", Err: err}
	}
	normalizeShiftWallClock(shifts)
	return shifts, nil
}

// FindByStaffAndDateRange returns one staff member's shifts in the range.
func (r *StaffShiftRepository) FindByStaffAndDateRange(ctx context.Context, staffID int64, start, end timezone.Date) ([]*schedule.StaffShift, error) {
	var shifts []*schedule.StaffShift
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&shifts).
		ModelTableExpr(tableExprStaffShiftsAsShift).
		Where(`"staff_shift".staff_id = ?`, staffID).
		Where(`"staff_shift".date >= ?`, start).
		Where(`"staff_shift".date <= ?`, end).
		OrderExpr(`"staff_shift".date ASC`).
		OrderExpr(`"staff_shift".start_time ASC`)

	query = base.WithTenantFilter(ctx, query, "staff_shift")

	if err := query.Scan(ctx); err != nil {
		return nil, &modelBase.DatabaseError{Op: "find staff shifts by staff and date range", Err: err}
	}
	normalizeShiftWallClock(shifts)
	return shifts, nil
}

// FindByStaffIDsAndDate returns the shifts of the given staff members on one
// date (batch lookup for the auto-checkout job).
func (r *StaffShiftRepository) FindByStaffIDsAndDate(ctx context.Context, staffIDs []int64, date timezone.Date) ([]*schedule.StaffShift, error) {
	if len(staffIDs) == 0 {
		return nil, nil
	}
	var shifts []*schedule.StaffShift
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&shifts).
		ModelTableExpr(tableExprStaffShiftsAsShift).
		Where(`"staff_shift".staff_id IN (?)`, bun.List(staffIDs)).
		Where(`"staff_shift".date = ?`, date).
		OrderExpr(`"staff_shift".staff_id ASC`).
		OrderExpr(`"staff_shift".start_time ASC`)

	query = base.WithTenantFilter(ctx, query, "staff_shift")

	if err := query.Scan(ctx); err != nil {
		return nil, &modelBase.DatabaseError{Op: "find staff shifts by staff ids and date", Err: err}
	}
	normalizeShiftWallClock(shifts)
	return shifts, nil
}

// DeleteUpcomingByStaffID removes planned shifts from the given date onwards.
// Past Dienstplan rows stay as history after staff offboarding.
func (r *StaffShiftRepository) DeleteUpcomingByStaffID(ctx context.Context, staffID int64, from timezone.Date) (int64, error) {
	query := base.GetDB(ctx, r.db).NewDelete().
		Table("schedule.staff_shifts").
		Where("staff_id = ?", staffID).
		Where("date >= ?", from)

	if tenantID := tenant.FromContext(ctx); tenantID > 0 {
		query = query.Where("tenant_id = ?", tenantID)
	}

	result, err := query.Exec(ctx)
	if err != nil {
		return 0, &modelBase.DatabaseError{Op: "delete upcoming staff shifts by staff ID", Err: err}
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return 0, &modelBase.DatabaseError{Op: "delete upcoming staff shifts by staff ID", Err: err}
	}
	return rows, nil
}
