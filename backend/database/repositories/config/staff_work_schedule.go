package config

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/config"
	"github.com/uptrace/bun"
)

const tableStaffWorkSchedules = "config.staff_work_schedules"

// StaffWorkScheduleRepository implements config.StaffWorkScheduleRepository
type StaffWorkScheduleRepository struct {
	runtime Runtime
}

// NewStaffWorkScheduleRepository creates a new StaffWorkScheduleRepository
func NewStaffWorkScheduleRepository(runtime Runtime) config.StaffWorkScheduleRepository {
	return &StaffWorkScheduleRepository{runtime: runtime}
}

// GetCurrentByStaffID returns schedule entries where valid_until IS NULL (currently active)
func (r *StaffWorkScheduleRepository) GetCurrentByStaffID(ctx context.Context, staffID int64) ([]*config.StaffWorkSchedule, error) {
	var entries []*config.StaffWorkSchedule
	query := r.runtime.DB(ctx).NewSelect().
		Model(&entries).
		ModelTableExpr(tableStaffWorkSchedules+` AS "staff_work_schedule"`).
		Where("staff_id = ?", staffID).
		Where("valid_until IS NULL").
		OrderExpr("day_of_week ASC")

	tenantID := r.runtime.TenantID(ctx)
	if tenantID > 0 {
		query = query.Where("tenant_id = ?", tenantID)
	}

	err := query.Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get current schedule: %w", err)
	}
	return entries, nil
}

// GetByStaffIDAndDate returns schedule entries valid for a specific date
func (r *StaffWorkScheduleRepository) GetByStaffIDAndDate(ctx context.Context, staffID int64, date timezone.Date) ([]*config.StaffWorkSchedule, error) {
	var entries []*config.StaffWorkSchedule
	query := r.runtime.DB(ctx).NewSelect().
		Model(&entries).
		ModelTableExpr(tableStaffWorkSchedules+` AS "staff_work_schedule"`).
		Where("staff_id = ?", staffID).
		Where("valid_from <= ?", date).
		Where("(valid_until IS NULL OR valid_until > ?)", date).
		OrderExpr("day_of_week ASC")

	tenantID := r.runtime.TenantID(ctx)
	if tenantID > 0 {
		query = query.Where("tenant_id = ?", tenantID)
	}

	err := query.Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get schedule for date: %w", err)
	}
	return entries, nil
}

// FindByStaffIDsValidInRange returns every schedule entry for the given staff
// members whose validity window intersects [from, to] (valid_until exclusive,
// matching GetByStaffIDAndDate).
func (r *StaffWorkScheduleRepository) FindByStaffIDsValidInRange(ctx context.Context, staffIDs []int64, from, to timezone.Date) ([]*config.StaffWorkSchedule, error) {
	if len(staffIDs) == 0 {
		return nil, nil
	}
	var entries []*config.StaffWorkSchedule
	query := r.runtime.DB(ctx).NewSelect().
		Model(&entries).
		ModelTableExpr(tableStaffWorkSchedules+` AS "staff_work_schedule"`).
		Where("staff_id IN (?)", bun.List(staffIDs)).
		Where("valid_from <= ?", to).
		Where("(valid_until IS NULL OR valid_until > ?)", from).
		OrderExpr("staff_id ASC, day_of_week ASC")

	tenantID := r.runtime.TenantID(ctx)
	if tenantID > 0 {
		query = query.Where("tenant_id = ?", tenantID)
	}

	if err := query.Scan(ctx); err != nil {
		return nil, fmt.Errorf("failed to get schedules for staff range: %w", err)
	}
	return entries, nil
}

// HasScheduleHistory reports whether the staff member has ever had a schedule
// snapshot, closed-out versions included.
func (r *StaffWorkScheduleRepository) HasScheduleHistory(ctx context.Context, staffID int64) (bool, error) {
	query := r.runtime.DB(ctx).NewSelect().
		Model((*config.StaffWorkSchedule)(nil)).
		ModelTableExpr(tableStaffWorkSchedules+` AS "staff_work_schedule"`).
		Where("staff_id = ?", staffID)

	tenantID := r.runtime.TenantID(ctx)
	if tenantID > 0 {
		query = query.Where("tenant_id = ?", tenantID)
	}

	exists, err := query.Exists(ctx)
	if err != nil {
		return false, fmt.Errorf("failed to check schedule history: %w", err)
	}
	return exists, nil
}

// ReplaceSchedule atomically replaces all current schedule entries for a staff member.
// It sets valid_until on existing entries and inserts the new ones.
func (r *StaffWorkScheduleRepository) ReplaceSchedule(ctx context.Context, staffID int64, entries []*config.StaffWorkSchedule, anchor timezone.Date) error {
	if err := r.runtime.LockStaffBalance(ctx, staffID); err != nil {
		return err
	}
	db := r.runtime.DB(ctx)
	tenantID := r.runtime.TenantID(ctx)
	now := time.Now()
	today := timezone.TodayDate()

	for _, e := range entries {
		e.StaffID = staffID
		e.ValidFrom = today
		e.ValidUntil = nil
		e.CreatedAt = now
		e.UpdatedAt = now
		e.RotationAnchorDate = nil
		if !anchor.IsZero() {
			versionAnchor := anchor
			e.RotationAnchorDate = &versionAnchor
		}
		if tenantID > 0 {
			e.SetTenantID(tenantID)
		}
		if err := e.Validate(); err != nil {
			return fmt.Errorf("invalid schedule entry for day %d: %w", e.DayOfWeek, err)
		}
	}

	// Close existing current entries at today, using an exclusive valid_until.
	updateQuery := db.NewUpdate().
		TableExpr(tableStaffWorkSchedules).
		Set("valid_until = ?", today).
		Set("updated_at = ?", now).
		Where("staff_id = ?", staffID).
		Where("valid_until IS NULL")

	if tenantID > 0 {
		updateQuery = updateQuery.Where("tenant_id = ?", tenantID)
	}

	if _, err := updateQuery.Exec(ctx); err != nil {
		return fmt.Errorf("failed to close existing schedule: %w", err)
	}

	// Insert new entries (only if there are any)
	if len(entries) == 0 {
		return nil
	}

	if _, err := db.NewInsert().
		Model(&entries).
		ModelTableExpr(tableStaffWorkSchedules).
		Exec(ctx); err != nil {
		return fmt.Errorf("failed to insert schedule entries: %w", err)
	}

	return nil
}

// FindStaffIDsWithScheduleHistory is HasScheduleHistory for many staff members
// in one round trip. A batched DISTINCT the generic filter API cannot express
// as a single query. Deliberately date-unbounded, exactly like its single-staff
// twin: the answer is "did this staff member EVER have a schedule version",
// which is what decides whether the assigned work-time model may stand in.
func (r *StaffWorkScheduleRepository) FindStaffIDsWithScheduleHistory(ctx context.Context, staffIDs []int64) (map[int64]bool, error) {
	result := make(map[int64]bool, len(staffIDs))
	if len(staffIDs) == 0 {
		// bun renders an empty IN list as invalid SQL.
		return result, nil
	}

	var rows []struct {
		StaffID int64 `bun:"staff_id"`
	}
	query := r.runtime.DB(ctx).NewSelect().
		ColumnExpr("DISTINCT staff_id").
		TableExpr(tableStaffWorkSchedules).
		Where("staff_id IN (?)", bun.List(staffIDs))

	if tenantID := r.runtime.TenantID(ctx); tenantID > 0 {
		query = query.Where("tenant_id = ?", tenantID)
	}

	if err := query.Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("failed to check schedule history for staff batch: %w", err)
	}
	for _, row := range rows {
		result[row.StaffID] = true
	}
	return result, nil
}
