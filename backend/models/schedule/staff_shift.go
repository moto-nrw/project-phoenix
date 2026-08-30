package schedule

import (
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/users"
)

const (
	tableScheduleStaffShifts        = "schedule.staff_shifts"
	tableScheduleStaffShiftsAsShift = `schedule.staff_shifts AS "staff_shift"`
	MaxStaffShiftBreakMinutes       = 300
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
	// values via timezone.NormalizeWallClock.
	StartTime    time.Time `bun:"start_time,notnull" json:"start_time"`
	EndTime      time.Time `bun:"end_time,notnull" json:"end_time"`
	BreakMinutes int       `bun:"break_minutes,notnull,default:0" json:"break_minutes"`
	// ShiftTypeID optionally links the shift to a tenant-defined Schichtart
	// (#1836). Nullable: a shift may have no type. ON DELETE SET NULL keeps the
	// shift when its type is deleted.
	ShiftTypeID *int64 `bun:"shift_type_id" json:"shift_type_id,omitempty"`
	Notes       string `bun:"notes" json:"notes,omitempty"`
	// SeriesID links a materialized row to its StaffShiftSeries (#1889).
	// Detached marks a series row the user edited via "Nur diese Woche":
	// re-plans delete and regenerate only non-detached rows.
	SeriesID *int64 `bun:"series_id" json:"series_id,omitempty"`
	Detached bool   `bun:"detached,notnull,default:false" json:"detached"`
	// SeriesOccurrenceDate is the concrete recurrence slot this row originally
	// materialized for. It deliberately does not follow Date when an occurrence
	// is moved: exceptions, re-plans, and splits must keep addressing the source
	// slot rather than accidentally consuming the moved row's current date.
	// Rows created as standalone leave it nil. A series hard-delete may leave the
	// now-standalone row's old value behind, but all consumers ignore it when
	// SeriesID is nil. This is persistence metadata, not an API field.
	SeriesOccurrenceDate *timezone.Date `bun:"series_occurrence_date,type:date" json:"-"`
	// Cancelled marks a shift that does not take place: the staff member is
	// absent or the position is deliberately left open (#1841). The row is kept
	// for the plan/history but excluded from planned minutes and auto-checkout.
	Cancelled bool `bun:"cancelled,notnull,default:false" json:"cancelled"`
	// ChangeReason carries the optional "why" for a flexible daily change
	// (extended/shortened times, a cancellation, or a replacement, #1841).
	ChangeReason *string `bun:"change_reason" json:"change_reason,omitempty"`
	// OriginShiftID, when set, marks this shift as a replacement covering
	// another (cancelled) shift. Several replacements sharing one origin split a
	// single gap across multiple people (#1841).
	OriginShiftID *int64 `bun:"origin_shift_id" json:"origin_shift_id,omitempty"`
	// SickAbsenceID identifies the active.staff_absences row whose #1843 sick
	// cascade cancelled this shift. NULL for manual cancellations; deleting the
	// sick report reactivates only shifts carrying its id.
	SickAbsenceID *int64 `bun:"sick_absence_id" json:"sick_absence_id,omitempty"`
	CreatedBy     int64  `bun:"created_by,notnull" json:"created_by"`
	UpdatedBy     *int64 `bun:"updated_by" json:"updated_by,omitempty"`

	Staff *users.Staff `bun:"rel:belongs-to,join:tenant_id=tenant_id,join:staff_id=id" json:"staff,omitempty"`
	// ShiftType is the resolved Schichtart (name + color) for ShiftTypeID, when
	// set. Not loaded by ordinary scans — the service populates it so readers who
	// cannot call the admin-only /api/shift-types endpoint (a staff member on
	// /api/time-tracking/shifts) still see the label and color (#1844). Nil when
	// the shift has no type or the resolution was skipped.
	ShiftType *ShiftType `bun:"rel:belongs-to,join:tenant_id=tenant_id,join:shift_type_id=id" json:"shift_type,omitempty"`
}

// Validate ensures shift data is consistent.
func (s *StaffShift) Validate() error {
	if s.StaffID <= 0 {
		return errors.New("staff ID is required")
	}
	if s.Date.IsZero() {
		return errors.New("date is required")
	}
	if err := validateShiftWindow(s.StartTime, s.EndTime, s.BreakMinutes); err != nil {
		return err
	}
	if s.CreatedBy <= 0 {
		return errors.New("created by is required")
	}
	return nil
}

// validateShiftWindow holds the wall-clock rules shared by single shifts and
// series: end after start, break within 0..MaxStaffShiftBreakMinutes and not
// longer than the shift itself.
func validateShiftWindow(startTime, endTime time.Time, breakMinutes int) error {
	start := timezone.NormalizeWallClock(startTime)
	end := timezone.NormalizeWallClock(endTime)
	if !end.After(start) {
		return errors.New("end time must be after start time")
	}
	if breakMinutes < 0 {
		return errors.New("break minutes must not be negative")
	}
	if breakMinutes > MaxStaffShiftBreakMinutes {
		return errors.New("break minutes must not exceed 300")
	}
	durationMinutes := int(end.Sub(start) / time.Minute)
	if breakMinutes > durationMinutes {
		return errors.New("break minutes must not exceed shift duration")
	}
	return nil
}

// Overlaps reports whether the wall-clock windows of two shifts intersect.
// Callers must ensure both shifts belong to the same date; touching
// boundaries (one ends exactly when the other starts) do not overlap.
func (s *StaffShift) Overlaps(other *StaffShift) bool {
	aStart, aEnd := timezone.NormalizeWallClock(s.StartTime), timezone.NormalizeWallClock(s.EndTime)
	bStart, bEnd := timezone.NormalizeWallClock(other.StartTime), timezone.NormalizeWallClock(other.EndTime)
	return aStart.Before(bEnd) && bStart.Before(aEnd)
}

// Contains reports whether other's wall-clock window lies entirely within s's
// window, shared boundaries included. Callers must ensure both shifts belong to
// the same date. Used to keep a replacement inside the gap of the cancelled
// origin it covers (#1841).
func (s *StaffShift) Contains(other *StaffShift) bool {
	sStart, sEnd := timezone.NormalizeWallClock(s.StartTime), timezone.NormalizeWallClock(s.EndTime)
	oStart, oEnd := timezone.NormalizeWallClock(other.StartTime), timezone.NormalizeWallClock(other.EndTime)
	return !oStart.Before(sStart) && !oEnd.After(sEnd)
}

// StartInstant returns the shift's planned start as an absolute instant in
// Berlin time (Date + wall-clock StartTime). time.Date normalizes values
// that fall into a DST gap.
func (s *StaffShift) StartInstant() time.Time {
	wc := timezone.NormalizeWallClock(s.StartTime)
	return time.Date(s.Date.Year, s.Date.Month, s.Date.Day, wc.Hour(), wc.Minute(), wc.Second(), 0, timezone.Berlin)
}

// EndInstant returns the shift's planned end as an absolute instant in
// Berlin time (Date + wall-clock EndTime). time.Date normalizes values that
// fall into a DST gap.
func (s *StaffShift) EndInstant() time.Time {
	wc := timezone.NormalizeWallClock(s.EndTime)
	return time.Date(s.Date.Year, s.Date.Month, s.Date.Day, wc.Hour(), wc.Minute(), wc.Second(), 0, timezone.Berlin)
}
