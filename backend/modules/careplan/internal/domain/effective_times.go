package domain

import (
	"time"

	timezone "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

type EffectiveScheduleFields struct {
	StudentID int64
	Weekday   int
	Time      time.Time
	Notes     *string
}

type EffectiveExceptionFields struct {
	ID                int64
	StudentID         int64
	Date              timezone.Date
	Time              *time.Time
	Reason            *string
	Source            string
	CreatedBy         int64
	CreatedByGuardian *int64
	CreatedAt         time.Time
	TenantID          int64
	ExcusedFrom       *time.Time
	ExcusedReason     *string
	ExcusedCreatedBy  *int64
	ExcusedOwnsTime   bool
	ExcusedAuto       bool
	TimeChanged       bool
}

type EffectiveNoteFields struct {
	ID        int64
	StudentID int64
	Content   string
}

type ExceptionCollisionPolicy uint8

const (
	ExceptionCollisionReject ExceptionCollisionPolicy = iota
	ExceptionCollisionUpdate
)

type EffectiveTimeData[S, E, N any] struct {
	Schedules  []S
	Exceptions []E
	Notes      []N
}

type EffectiveDayNote struct {
	ID      int64
	Content string
}

type EffectiveTimeResult struct {
	Date        timezone.Date
	Time        *time.Time
	WeekdayName string
	IsException bool
	Notes       string
	DayNotes    []EffectiveDayNote
	// RegularTime is the recurring plan's time for that weekday, kept next
	// to the effective one so a caller can say WHAT changed and not only
	// THAT something changed (#2294). It stays filled when an exception
	// overrides the day; nil means the recurring plan carries no time for
	// the day (no row, or an arrival row inheriting the class timetable).
	RegularTime *time.Time
	// ChangedAt is when the overriding exception was recorded. Only set
	// together with IsException. It is the row's creation timestamp: a
	// later edit of the same day keeps the first entry's stamp, which is
	// the conservative reading ("known since at least then").
	ChangedAt *time.Time
}

func ISOWeekday(date timezone.Date) int {
	weekday := int(date.Weekday())
	if weekday == 0 {
		return 7
	}
	return weekday
}

func WeekdayName(weekday int) string {
	names := [...]string{"", "Montag", "Dienstag", "Mittwoch", "Donnerstag", "Freitag", "Samstag", "Sonntag"}
	if weekday < 1 || weekday >= len(names) {
		return ""
	}
	return names[weekday]
}

// ApplyTimePatch preserves omitted fields and compares normalized wall clocks.
// An unchanged clock must retain partial-absence ownership of the pickup time.
func (fields *EffectiveExceptionFields) ApplyTimePatch(date timezone.Date, reason *string, value *time.Time, clearValue bool) {
	fields.Date = date
	if fields.Time != nil {
		normalized := timezone.NormalizeWallClock(*fields.Time)
		fields.Time = &normalized
	}
	if fields.ExcusedFrom != nil {
		normalized := timezone.NormalizeWallClock(*fields.ExcusedFrom)
		fields.ExcusedFrom = &normalized
	}
	if reason != nil {
		// Nil omits the patch; an empty string clears the stored reason.
		fields.Reason = reason
		if *reason == "" {
			fields.Reason = nil
		}
	}
	if value != nil {
		normalized := timezone.NormalizeWallClock(*value)
		if fields.Time == nil || !timezone.SameClockTime(*fields.Time, normalized) {
			fields.TimeChanged = true
		}
		fields.Time = &normalized
	} else if clearValue {
		fields.Time = nil
		fields.TimeChanged = true
	}
}
