package config

import (
	"context"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/uptrace/bun"
)

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

// StaffWorkSchedule defines target working hours for a staff member per day of week.
// When WeekIndex is non-zero or RotationLength > 1 the entry belongs to a multi-week
// rotation (e.g. A/B-Wochen). Existing rows from the single-week era default to
// week_index=0 / rotation_length=1 and behave exactly as before.
type StaffWorkSchedule struct {
	bun.BaseModel `bun:"table:config.staff_work_schedules,alias:staff_work_schedule"`

	ID             int64      `bun:"id,pk,autoincrement" json:"id"`
	TenantID       int64      `bun:"tenant_id,notnull" json:"tenant_id"`
	StaffID        int64      `bun:"staff_id,notnull" json:"staff_id"`
	WeekIndex      int        `bun:"week_index,notnull,default:0" json:"week_index"`
	RotationLength int        `bun:"rotation_length,notnull,default:1" json:"rotation_length"`
	DayOfWeek      int        `bun:"day_of_week,notnull" json:"day_of_week"`
	TargetMinutes  int        `bun:"target_minutes,notnull" json:"target_minutes"`
	StartTime      *time.Time `bun:"start_time" json:"start_time,omitempty"`
	// RotationAnchorDate is the rotation anchor this schedule version was
	// written with (#1842). It is immutable: a schedule change closes these
	// rows and inserts a new version carrying its own anchor, so past weeks
	// keep the A/B parity they were computed with. NULL only on rows written
	// before 1.15.203 with no staff-level anchor to backfill from.
	RotationAnchorDate *timezone.Date `bun:"rotation_anchor_date,type:date" json:"rotation_anchor_date,omitempty"`
	ValidFrom          timezone.Date  `bun:"valid_from,notnull,type:date" json:"valid_from"`
	ValidUntil         *timezone.Date `bun:"valid_until,type:date" json:"valid_until,omitempty"`
	CreatedAt          time.Time      `bun:"created_at,notnull,default:now()" json:"created_at"`
	UpdatedAt          time.Time      `bun:"updated_at,notnull,default:now()" json:"updated_at"`
}

func (s *StaffWorkSchedule) Validate() error {
	if s.StaffID <= 0 {
		return errors.New("staff_id is required")
	}
	if s.DayOfWeek < 0 || s.DayOfWeek > 6 {
		return errors.New("day_of_week must be between 0 (Monday) and 6 (Sunday)")
	}
	if s.TargetMinutes < 0 || s.TargetMinutes > WorkTimeModelMaxDailyMinutes {
		return errors.New("target_minutes must be between 0 and 720 (12h)")
	}
	if s.RotationLength < 1 || s.RotationLength > WorkTimeModelMaxRotation {
		return errors.New("rotation_length must be between 1 and 4")
	}
	if s.WeekIndex < 0 || s.WeekIndex >= s.RotationLength {
		return errors.New("week_index must be between 0 and rotation_length - 1")
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
	GetByStaffIDAndDate(ctx context.Context, staffID int64, date timezone.Date) ([]*StaffWorkSchedule, error)

	// FindByStaffIDsValidInRange returns every schedule entry for the given
	// staff members whose validity window intersects [from, to] (valid_until
	// is exclusive). One batched read for cross-staff aggregations; the
	// date-validity predicate is why this is not expressible via the generic
	// filter-based List.
	FindByStaffIDsValidInRange(ctx context.Context, staffIDs []int64, from, to timezone.Date) ([]*StaffWorkSchedule, error)

	// HasScheduleHistory reports whether the staff member has ever had a
	// schedule snapshot, closed-out versions included. It separates "no
	// schedule was ever written" — where the assigned work-time model may
	// stand in — from "no version is valid in the queried range", where the
	// Soll is genuinely zero.
	HasScheduleHistory(ctx context.Context, staffID int64) (bool, error)

	// FindStaffIDsWithScheduleHistory is HasScheduleHistory batched over many
	// staff members; only staff with at least one schedule version appear.
	FindStaffIDsWithScheduleHistory(ctx context.Context, staffIDs []int64) (map[int64]bool, error)

	// ReplaceSchedule atomically replaces all current schedule entries for a
	// staff member. anchor is the rotation anchor the new version is written
	// with and is stamped onto every inserted row (#1842) — the closed-out
	// rows keep theirs, so a schedule change can no longer re-parity the past.
	// A zero anchor leaves the column NULL (rotation_length 1 has no parity).
	ReplaceSchedule(ctx context.Context, staffID int64, entries []*StaffWorkSchedule, anchor timezone.Date) error
}
