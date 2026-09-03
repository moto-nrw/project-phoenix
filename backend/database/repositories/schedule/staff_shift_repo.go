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

// FindByID returns one shift with its TIME columns re-anchored via
// timezone.NormalizeWallClock. The embedded generic scan leaves the driver's year-0
// anchor on TIME columns; a whole-model update that preserves the stored
// window would write that anchor back as an out-of-range timestamp
// (SQLSTATE 22008 — exposed by the #1843 cascade's plain-cancel path, latent
// since #1841 because the cancellation modal always sends explicit times).
func (r *StaffShiftRepository) FindByID(ctx context.Context, id any) (*schedule.StaffShift, error) {
	shift, err := r.Repository.FindByID(ctx, id)
	if err != nil || shift == nil {
		return shift, err
	}
	normalizeShiftWallClock([]*schedule.StaffShift{shift})
	return shift, nil
}

// ListWithOptions exposes the generic filtered list while preserving the
// repository's wall-clock normalization invariant.
func (r *StaffShiftRepository) ListWithOptions(ctx context.Context, options *modelBase.QueryOptions) ([]*schedule.StaffShift, error) {
	shifts, err := r.Repository.ListWithOptions(ctx, options)
	if err != nil {
		return nil, err
	}
	normalizeShiftWallClock(shifts)
	return shifts, nil
}

// normalizeWallClock strips the driver-arbitrary year anchor from scanned
// TIME columns so callers can compare and format the wall-clock values.
func normalizeShiftWallClock(shifts []*schedule.StaffShift) {
	for _, s := range shifts {
		s.StartTime = timezone.NormalizeWallClock(s.StartTime)
		s.EndTime = timezone.NormalizeWallClock(s.EndTime)
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
		return nil, &modelBase.DatabaseError{Op: "find staff shifts by date range", Err: base.TranslateNotFound(err)}
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
		return nil, &modelBase.DatabaseError{Op: "find staff shifts by staff and date range", Err: base.TranslateNotFound(err)}
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
		return nil, &modelBase.DatabaseError{Op: "find staff shifts by staff ids and date", Err: base.TranslateNotFound(err)}
	}
	normalizeShiftWallClock(shifts)
	return shifts, nil
}

// FindByOriginShiftID returns every replacement shift covering the given
// origin (the cover set of one gap, #1841), ordered by start time.
func (r *StaffShiftRepository) FindByOriginShiftID(ctx context.Context, originShiftID int64) ([]*schedule.StaffShift, error) {
	var shifts []*schedule.StaffShift
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&shifts).
		ModelTableExpr(tableExprStaffShiftsAsShift).
		Where(`"staff_shift".origin_shift_id = ?`, originShiftID).
		OrderExpr(`"staff_shift".start_time ASC`)

	query = base.WithTenantFilter(ctx, query, "staff_shift")

	if err := query.Scan(ctx); err != nil {
		return nil, &modelBase.DatabaseError{Op: "find staff shifts by origin shift id", Err: base.TranslateNotFound(err)}
	}
	normalizeShiftWallClock(shifts)
	return shifts, nil
}

// FindByStaffIDsAndDates returns only shifts that can affect the supplied
// staff/date coverage comparisons. This avoids scanning every tenant shift in
// the continuous span between sparse candidate dates.
func (r *StaffShiftRepository) FindByStaffIDsAndDates(ctx context.Context, staffIDs []int64, dates []timezone.Date) ([]*schedule.StaffShift, error) {
	if len(staffIDs) == 0 || len(dates) == 0 {
		return nil, nil
	}
	var shifts []*schedule.StaffShift
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&shifts).
		ModelTableExpr(tableExprStaffShiftsAsShift).
		Where(`"staff_shift".staff_id IN (?)`, bun.List(staffIDs)).
		Where(`"staff_shift".date IN (?)`, bun.List(dates)).
		OrderExpr(`"staff_shift".date ASC`).
		OrderExpr(`"staff_shift".staff_id ASC`).
		OrderExpr(`"staff_shift".start_time ASC`)

	query = base.WithTenantFilter(ctx, query, "staff_shift")
	if err := query.Scan(ctx); err != nil {
		return nil, &modelBase.DatabaseError{Op: "find staff shifts by staff IDs and dates", Err: base.TranslateNotFound(err)}
	}
	normalizeShiftWallClock(shifts)
	return shifts, nil
}

// FindUsedCalendarWeeks returns distinct ISO-week Mondays for tenant shifts in
// the inclusive range. Cancelled shifts do not mark a week as planned — every
// consumer (staff pool, coverage warnings, schedule overview) already ignores
// them in its per-staff data, so counting them here would report a Dienstplan
// as "in use" that offers nobody a usable shift. The result is small even for
// multi-week probes.
func (r *StaffShiftRepository) FindUsedCalendarWeeks(ctx context.Context, start, end timezone.Date) ([]timezone.Date, error) {
	type usedWeekRow struct {
		WeekStart timezone.Date `bun:"week_start,type:date"`
	}
	var rows []usedWeekRow
	query := base.GetDB(ctx, r.db).NewSelect().
		TableExpr(tableExprStaffShiftsAsShift).
		ColumnExpr(`DISTINCT date_trunc('week', "staff_shift".date)::date AS week_start`).
		Where(`"staff_shift".date >= ?`, start).
		Where(`"staff_shift".date <= ?`, end).
		Where(`"staff_shift".cancelled = FALSE`).
		OrderExpr(`week_start ASC`)

	query = base.WithTenantFilter(ctx, query, "staff_shift")
	if err := query.Scan(ctx, &rows); err != nil {
		return nil, &modelBase.DatabaseError{Op: "find used staff-shift calendar weeks", Err: base.TranslateNotFound(err)}
	}
	weeks := make([]timezone.Date, 0, len(rows))
	for _, row := range rows {
		weeks = append(weeks, row.WeekStart)
	}
	return weeks, nil
}

// BulkCreate inserts all shifts in one multi-row statement (series
// materialization, #1889). Tenant IDs are populated from context like the
// generic Create.
func (r *StaffShiftRepository) BulkCreate(ctx context.Context, shifts []*schedule.StaffShift) error {
	if len(shifts) == 0 {
		return nil
	}
	for _, s := range shifts {
		base.EnsureTenantID(ctx, s)
	}
	_, err := base.GetDB(ctx, r.db).NewInsert().
		Model(&shifts).
		ModelTableExpr("schedule.staff_shifts").
		Exec(ctx)
	if err != nil {
		return &modelBase.DatabaseError{Op: "bulk create staff shifts", Err: base.TranslateNotFound(err)}
	}
	return nil
}

// DeleteNonDetachedBySeriesFrom removes a series' regenerable rows on or
// after from. Detached rows ("Nur diese Woche" edits) survive re-plans.
func (r *StaffShiftRepository) DeleteNonDetachedBySeriesFrom(ctx context.Context, seriesID int64, from timezone.Date) (int64, error) {
	query := base.GetDB(ctx, r.db).NewDelete().
		Table("schedule.staff_shifts").
		Where("series_id = ?", seriesID).
		Where("detached = FALSE").
		Where("date >= ?", from)

	if tenantID := tenant.FromContext(ctx); tenantID > 0 {
		query = query.Where("tenant_id = ?", tenantID)
	}

	result, err := query.Exec(ctx)
	if err != nil {
		return 0, &modelBase.DatabaseError{Op: "delete non-detached staff shifts by series", Err: base.TranslateNotFound(err)}
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return 0, &modelBase.DatabaseError{Op: "delete non-detached staff shifts by series", Err: base.TranslateNotFound(err)}
	}
	return rows, nil
}

// RepointDetachedSeriesFrom moves a series' detached occurrences on or after
// from to the successor series created by a split, preserving the deviations.
// A moved row's current date is not its recurrence slot; use the immutable
// source date so a cross-boundary move remains attached to the right segment.
func (r *StaffShiftRepository) RepointDetachedSeriesFrom(ctx context.Context, fromSeriesID, toSeriesID int64, from timezone.Date) (int64, error) {
	query := base.GetDB(ctx, r.db).NewUpdate().
		Table("schedule.staff_shifts").
		Set("series_id = ?", toSeriesID).
		Where("series_id = ?", fromSeriesID).
		Where("detached = TRUE").
		Where("series_occurrence_date >= ?", from)

	if tenantID := tenant.FromContext(ctx); tenantID > 0 {
		query = query.Where("tenant_id = ?", tenantID)
	}

	result, err := query.Exec(ctx)
	if err != nil {
		return 0, &modelBase.DatabaseError{Op: "repoint detached staff shifts to successor series", Err: base.TranslateNotFound(err)}
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return 0, &modelBase.DatabaseError{Op: "repoint detached staff shifts to successor series", Err: base.TranslateNotFound(err)}
	}
	return rows, nil
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
		return 0, &modelBase.DatabaseError{Op: "delete upcoming staff shifts by staff ID", Err: base.TranslateNotFound(err)}
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return 0, &modelBase.DatabaseError{Op: "delete upcoming staff shifts by staff ID", Err: base.TranslateNotFound(err)}
	}
	return rows, nil
}

// FindByStaffIDsAndDateRange is FindByStaffAndDateRange for many staff members
// in one round trip, keyed by staff ID. A batched IN-lookup the generic filter
// API cannot express as a single query. Wall-clock normalization is applied
// exactly as in the single-staff twin — shiftNetMinutes depends on it.
func (r *StaffShiftRepository) FindByStaffIDsAndDateRange(ctx context.Context, staffIDs []int64, start, end timezone.Date) (map[int64][]*schedule.StaffShift, error) {
	result := make(map[int64][]*schedule.StaffShift, len(staffIDs))
	if len(staffIDs) == 0 {
		// bun renders an empty IN list as invalid SQL.
		return result, nil
	}

	var shifts []*schedule.StaffShift
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&shifts).
		ModelTableExpr(tableExprStaffShiftsAsShift).
		Where(`"staff_shift".staff_id IN (?)`, bun.List(staffIDs)).
		Where(`"staff_shift".date >= ?`, start).
		Where(`"staff_shift".date <= ?`, end).
		OrderExpr(`"staff_shift".staff_id ASC`).
		OrderExpr(`"staff_shift".date ASC`).
		OrderExpr(`"staff_shift".start_time ASC`)

	query = base.WithTenantFilter(ctx, query, "staff_shift")

	if err := query.Scan(ctx); err != nil {
		return nil, &modelBase.DatabaseError{Op: "find staff shifts by staff IDs and date range", Err: base.TranslateNotFound(err)}
	}
	normalizeShiftWallClock(shifts)
	for _, shift := range shifts {
		result[shift.StaffID] = append(result[shift.StaffID], shift)
	}
	return result, nil
}
