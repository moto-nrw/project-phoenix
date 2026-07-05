package schedule

import (
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/uptrace/bun"
)

const (
	tableScheduleStaffShifts  = "schedule.staff_shifts"
	MaxStaffShiftBreakMinutes = 300
)

// StaffShift is one planned work shift for a staff member on a concrete
// calendar date (Dienstplan). Multiple non-overlapping shifts per staff per
// day are allowed (split shifts). Distinct from the contractual weekly Soll
// in config.staff_work_schedules, which only stores target minutes per
// weekday — this table carries the actual planned wall-clock times that the
// auto-checkout job (#1798) closes forgotten sessions against.
type StaffShift struct {
	base.Model `bun:"schema:schedule,table:staff_shifts"`
	base.TenantModel
	StaffID int64         `bun:"staff_id,notnull" json:"staff_id"`
	Date    timezone.Date `bun:"date,notnull,type:date" json:"date"`
	// StartTime/EndTime map TIME WITHOUT TIME ZONE columns; only the
	// wall-clock portion is meaningful. Repositories normalize scanned
	// values via timezone.WallClock.
	StartTime    time.Time `bun:"start_time,notnull" json:"start_time"`
	EndTime      time.Time `bun:"end_time,notnull" json:"end_time"`
	BreakMinutes int       `bun:"break_minutes,notnull,default:0" json:"break_minutes"`
	Notes        string    `bun:"notes" json:"notes,omitempty"`
	CreatedBy    int64     `bun:"created_by,notnull" json:"created_by"`
	UpdatedBy    *int64    `bun:"updated_by" json:"updated_by,omitempty"`

	Staff *users.Staff `bun:"rel:belongs-to,join:staff_id=id" json:"staff,omitempty"`
}

func (s *StaffShift) BeforeAppendModel(query any) error {
	if q, ok := query.(*bun.UpdateQuery); ok {
		q.ModelTableExpr(tableScheduleStaffShifts)
	}
	if q, ok := query.(*bun.DeleteQuery); ok {
		q.ModelTableExpr(tableScheduleStaffShifts)
	}
	if q, ok := query.(*bun.InsertQuery); ok {
		q.ModelTableExpr(tableScheduleStaffShifts)
	}
	return nil
}

func (s *StaffShift) TableName() string { return tableScheduleStaffShifts }

// Validate ensures shift data is consistent.
func (s *StaffShift) Validate() error {
	if s.StaffID <= 0 {
		return errors.New("staff ID is required")
	}
	if s.Date.IsZero() {
		return errors.New("date is required")
	}
	start := timezone.WallClock(s.StartTime)
	end := timezone.WallClock(s.EndTime)
	if !end.After(start) {
		return errors.New("end time must be after start time")
	}
	if s.BreakMinutes < 0 {
		return errors.New("break minutes must not be negative")
	}
	if s.BreakMinutes > MaxStaffShiftBreakMinutes {
		return errors.New("break minutes must not exceed 300")
	}
	durationMinutes := int(end.Sub(start) / time.Minute)
	if s.BreakMinutes > durationMinutes {
		return errors.New("break minutes must not exceed shift duration")
	}
	if s.CreatedBy <= 0 {
		return errors.New("created by is required")
	}
	return nil
}

// Overlaps reports whether the wall-clock windows of two shifts intersect.
// Callers must ensure both shifts belong to the same date; touching
// boundaries (one ends exactly when the other starts) do not overlap.
func (s *StaffShift) Overlaps(other *StaffShift) bool {
	aStart, aEnd := timezone.WallClock(s.StartTime), timezone.WallClock(s.EndTime)
	bStart, bEnd := timezone.WallClock(other.StartTime), timezone.WallClock(other.EndTime)
	return aStart.Before(bEnd) && bStart.Before(aEnd)
}

// EndInstant returns the shift's planned end as an absolute instant in
// Berlin time (Date + wall-clock EndTime). time.Date normalizes values that
// fall into a DST gap.
func (s *StaffShift) EndInstant() time.Time {
	wc := timezone.WallClock(s.EndTime)
	return time.Date(s.Date.Year, s.Date.Month, s.Date.Day, wc.Hour(), wc.Minute(), wc.Second(), 0, timezone.Berlin)
}

// GetID implements the Entity interface
func (s *StaffShift) GetID() interface{} {
	return s.ID
}

// GetCreatedAt implements the Entity interface
func (s *StaffShift) GetCreatedAt() time.Time {
	return s.CreatedAt
}

// GetUpdatedAt implements the Entity interface
func (s *StaffShift) GetUpdatedAt() time.Time {
	return s.UpdatedAt
}
