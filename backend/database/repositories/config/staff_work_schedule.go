package config

import (
	"context"
	"fmt"
	"time"

	repoBase "github.com/moto-nrw/project-phoenix/database/repositories/base"
	"github.com/moto-nrw/project-phoenix/models/config"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

const tableStaffWorkSchedules = "config.staff_work_schedules"

// StaffWorkScheduleRepository implements config.StaffWorkScheduleRepository
type StaffWorkScheduleRepository struct {
	db *bun.DB
}

// NewStaffWorkScheduleRepository creates a new StaffWorkScheduleRepository
func NewStaffWorkScheduleRepository(db *bun.DB) config.StaffWorkScheduleRepository {
	return &StaffWorkScheduleRepository{db: db}
}

// GetCurrentByStaffID returns schedule entries where valid_until IS NULL (currently active)
func (r *StaffWorkScheduleRepository) GetCurrentByStaffID(ctx context.Context, staffID int64) ([]*config.StaffWorkSchedule, error) {
	var entries []*config.StaffWorkSchedule
	query := repoBase.GetDB(ctx, r.db).NewSelect().
		Model(&entries).
		ModelTableExpr(tableStaffWorkSchedules).
		Where("staff_id = ?", staffID).
		Where("valid_until IS NULL").
		OrderExpr("day_of_week ASC")

	tenantID := tenant.FromContext(ctx)
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
func (r *StaffWorkScheduleRepository) GetByStaffIDAndDate(ctx context.Context, staffID int64, date time.Time) ([]*config.StaffWorkSchedule, error) {
	var entries []*config.StaffWorkSchedule
	query := repoBase.GetDB(ctx, r.db).NewSelect().
		Model(&entries).
		ModelTableExpr(tableStaffWorkSchedules).
		Where("staff_id = ?", staffID).
		Where("valid_from <= ?", date).
		Where("valid_until IS NULL OR valid_until >= ?", date).
		OrderExpr("day_of_week ASC")

	tenantID := tenant.FromContext(ctx)
	if tenantID > 0 {
		query = query.Where("tenant_id = ?", tenantID)
	}

	err := query.Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get schedule for date: %w", err)
	}
	return entries, nil
}

// ReplaceSchedule atomically replaces all current schedule entries for a staff member.
// It sets valid_until on existing entries and inserts the new ones.
func (r *StaffWorkScheduleRepository) ReplaceSchedule(ctx context.Context, staffID int64, entries []*config.StaffWorkSchedule) error {
	db := repoBase.GetDB(ctx, r.db)
	tenantID := tenant.FromContext(ctx)
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)

	// Close existing current entries by setting valid_until to today
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

	for _, e := range entries {
		e.StaffID = staffID
		e.ValidFrom = today
		e.ValidUntil = nil
		e.CreatedAt = now
		e.UpdatedAt = now
		if tenantID > 0 {
			e.SetTenantID(tenantID)
		}
		if err := e.Validate(); err != nil {
			return fmt.Errorf("invalid schedule entry for day %d: %w", e.DayOfWeek, err)
		}
	}

	if _, err := db.NewInsert().
		Model(&entries).
		ModelTableExpr(tableStaffWorkSchedules).
		Exec(ctx); err != nil {
		return fmt.Errorf("failed to insert schedule entries: %w", err)
	}

	return nil
}
