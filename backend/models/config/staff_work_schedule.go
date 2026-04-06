package config

import (
	"context"
	"errors"
	"time"

	"github.com/uptrace/bun"
)

const tableStaffWorkSchedules = "config.staff_work_schedules"

// Day of week constants (ISO: 0=Monday, 6=Sunday)
const (
	DayMonday    = 0
	DayTuesday   = 1
	DayWednesday = 2
	DayThursday  = 3
	DayFriday    = 4
	DaySaturday  = 5
	DaySunday    = 6
)

// StaffWorkSchedule defines target working hours for a staff member per day of week
type StaffWorkSchedule struct {
	ID            int64      `bun:"id,pk,autoincrement" json:"id"`
	TenantID      int64      `bun:"tenant_id,notnull" json:"tenant_id"`
	StaffID       int64      `bun:"staff_id,notnull" json:"staff_id"`
	DayOfWeek     int        `bun:"day_of_week,notnull" json:"day_of_week"`
	TargetMinutes int        `bun:"target_minutes,notnull" json:"target_minutes"`
	ValidFrom     time.Time  `bun:"valid_from,notnull,type:date" json:"valid_from"`
	ValidUntil    *time.Time `bun:"valid_until,type:date" json:"valid_until,omitempty"`
	CreatedAt     time.Time  `bun:"created_at,notnull,default:now()" json:"created_at"`
	UpdatedAt     time.Time  `bun:"updated_at,notnull,default:now()" json:"updated_at"`
}

func (s *StaffWorkSchedule) BeforeAppendModel(query any) error {
	if q, ok := query.(*bun.UpdateQuery); ok {
		q.ModelTableExpr(tableStaffWorkSchedules)
	}
	if q, ok := query.(*bun.DeleteQuery); ok {
		q.ModelTableExpr(tableStaffWorkSchedules)
	}
	if q, ok := query.(*bun.InsertQuery); ok {
		q.ModelTableExpr(tableStaffWorkSchedules)
	}
	return nil
}

func (s *StaffWorkSchedule) TableName() string {
	return tableStaffWorkSchedules
}

func (s *StaffWorkSchedule) Validate() error {
	if s.StaffID <= 0 {
		return errors.New("staff_id is required")
	}
	if s.DayOfWeek < 0 || s.DayOfWeek > 6 {
		return errors.New("day_of_week must be between 0 (Monday) and 6 (Sunday)")
	}
	if s.TargetMinutes < 0 || s.TargetMinutes > 720 {
		return errors.New("target_minutes must be between 0 and 720 (12h)")
	}
	if s.ValidFrom.IsZero() {
		return errors.New("valid_from is required")
	}
	if s.ValidUntil != nil && s.ValidUntil.Before(s.ValidFrom) {
		return errors.New("valid_until must be on or after valid_from")
	}
	return nil
}

func (s *StaffWorkSchedule) SetTenantID(tenantID int64) {
	s.TenantID = tenantID
}

// StaffWorkScheduleRepository defines operations for managing staff work schedules
type StaffWorkScheduleRepository interface {
	// GetCurrentByStaffID returns the currently valid schedule entries for a staff member
	GetCurrentByStaffID(ctx context.Context, staffID int64) ([]*StaffWorkSchedule, error)

	// GetByStaffIDAndDate returns schedule entries valid for a specific date
	GetByStaffIDAndDate(ctx context.Context, staffID int64, date time.Time) ([]*StaffWorkSchedule, error)

	// ReplaceSchedule atomically replaces all current schedule entries for a staff member
	ReplaceSchedule(ctx context.Context, staffID int64, entries []*StaffWorkSchedule) error
}

// WeeklyTargetFromSchedule calculates total weekly target minutes from schedule entries
func WeeklyTargetFromSchedule(entries []*StaffWorkSchedule) *int {
	if len(entries) == 0 {
		return nil
	}
	total := 0
	for _, e := range entries {
		total += e.TargetMinutes
	}
	return &total
}

// DailyTargetFromSchedule returns target minutes for a specific day of week
func DailyTargetFromSchedule(entries []*StaffWorkSchedule, dayOfWeek int) *int {
	for _, e := range entries {
		if e.DayOfWeek == dayOfWeek {
			return &e.TargetMinutes
		}
	}
	return nil
}
