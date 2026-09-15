package active

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
)

// This file holds the work-schedule vocabulary the retained work-session
// service owns: the schedule snapshot and work-time-template values it reads
// and writes, and the pure Soll-minute derivations over them. The rows
// themselves are persisted by the workforce repositories behind the ports
// below; nothing here knows how they are stored.

// Day-of-week constants (ISO: 0=Monday, 6=Sunday).
const (
	DayMonday    = 0
	DayTuesday   = 1
	DayWednesday = 2
	DayThursday  = 3
	DayFriday    = 4
	DaySaturday  = 5
	DaySunday    = 6
)

const (
	// MaxScheduleRotation caps a schedule at four rotation weeks.
	MaxScheduleRotation = 4
	// MaxDailyTargetMinutes caps a single day's target working time at 12
	// hours.
	MaxDailyTargetMinutes = 720
)

// WorkScheduleRow is one date-valid target-hours row of a staff member's
// schedule snapshot. When WeekIndex is non-zero or RotationLength > 1 the row
// belongs to a multi-week rotation (A/B-Wochen).
type WorkScheduleRow struct {
	StaffID        int64
	WeekIndex      int
	RotationLength int
	DayOfWeek      int
	TargetMinutes  int
	StartTime      *time.Time
	// RotationAnchorDate is the rotation anchor this schedule version was
	// written with (#1842). It is immutable: a schedule change closes these
	// rows and inserts a new version carrying its own anchor, so past weeks
	// keep the A/B parity they were computed with. Unset only on rows written
	// before 1.15.203 with no staff-level anchor to backfill from.
	RotationAnchorDate *timezone.Date
	ValidFrom          timezone.Date
	ValidUntil         *timezone.Date
}

// WorkTimeTemplate is a named working-time pattern ("Vollzeit 40h Mo-Fr")
// spanning up to four rotation weeks, shared by any number of staff members.
type WorkTimeTemplate struct {
	ID                 int64
	Name               string
	RotationLength     int
	RotationAnchorDate timezone.Date
	Entries            []*WorkTimeTemplateEntry
}

// WorkTimeTemplateEntry carries the target minutes of one
// (week index, day of week) slot inside a template.
type WorkTimeTemplateEntry struct {
	WeekIndex     int
	DayOfWeek     int
	TargetMinutes int
	StartTime     *time.Time
}

// WorkSessionSchedules is the schedule capability used by work-session
// enforcement, summaries, and effective-dated schedule replacement.
type WorkSessionSchedules interface {
	GetByStaffIDAndDate(context.Context, int64, timezone.Date) ([]*WorkScheduleRow, error)
	FindByStaffIDsValidInRange(context.Context, []int64, timezone.Date, timezone.Date) ([]*WorkScheduleRow, error)
	GetCurrentByStaffID(context.Context, int64) ([]*WorkScheduleRow, error)
	ReplaceSchedule(context.Context, int64, []*WorkScheduleRow, timezone.Date) error
}

// WorkSessionTimeModels is the work-time-template capability the work-session
// service still calls: reading one template and saving a custom schedule as a
// new one. Template administration itself runs through the Workforce facade.
type WorkSessionTimeModels interface {
	FindByID(context.Context, int64) (*WorkTimeTemplate, error)
	Create(context.Context, *WorkTimeTemplate, []*WorkTimeTemplateEntry) error
}

// ISODayIndex maps a date's weekday onto the schedule day constants
// (DayMonday=0 … DaySunday=6).
func ISODayIndex(d timezone.Date) int {
	weekday := int(d.Weekday())
	if weekday == 0 {
		return DaySunday
	}
	return weekday - 1
}

// ScheduleRotationLength returns the rotation length spanned by the given
// schedule rows (minimum 1).
func ScheduleRotationLength(rows []*WorkScheduleRow) int {
	rotation := 1
	for _, row := range rows {
		if row != nil && row.RotationLength > rotation {
			rotation = row.RotationLength
		}
	}
	return rotation
}

// ResolveRotationWeek returns the rotation week index for a date relative to
// the anchor. Negative deltas (date before anchor) wrap forwards so the result
// is always in [0, rotationLength). Callers pass week-start dates, so the day
// difference is an exact multiple of 7.
func ResolveRotationWeek(rotationLength int, anchor, date timezone.Date) int {
	if rotationLength <= 1 {
		return 0
	}
	delta := anchor.DaysUntil(date) / 7
	mod := delta % rotationLength
	if mod < 0 {
		mod += rotationLength
	}
	return mod
}

// ResolveScheduleAnchor returns the rotation anchor for a staff member's
// schedule rows, in descending order of historical truth (#1842):
//
//  1. the anchor persisted on the rows themselves — the one the version was
//     actually written with, immune to later schedule edits;
//  2. the staff-level anchor — the live value, which every schedule change
//     overwrites, so it is only correct for the CURRENT version;
//  3. the earliest valid_from among the rows.
//
// Rows reaching step 2 predate migration 1.15.203 and had no staff anchor to
// backfill; step 2 then reproduces the pre-1.15.203 behaviour exactly.
func ResolveScheduleAnchor(staffAnchor *timezone.Date, rows []*WorkScheduleRow) timezone.Date {
	for _, row := range rows {
		if row != nil && row.RotationAnchorDate != nil && !row.RotationAnchorDate.IsZero() {
			return *row.RotationAnchorDate
		}
	}
	if staffAnchor != nil {
		return *staffAnchor
	}
	var earliest timezone.Date
	for _, row := range rows {
		if row == nil {
			continue
		}
		if earliest.IsZero() || row.ValidFrom.Before(earliest) {
			earliest = row.ValidFrom
		}
	}
	return earliest
}

// DailyTargetFromSchedule resolves the target minutes a staff member's
// schedule rows yield for one calendar day. Rows apply when
// valid_from <= day and valid_until is unset or after the day (valid_until is
// exclusive, matching the repository predicate). The boolean reports whether a
// row MATCHED the day's rotation week + weekday — a date-valid schedule that
// simply has no row for this day/rotation returns (0, false), so callers can
// distinguish "day off per plan" from a plain zero.
func DailyTargetFromSchedule(rows []*WorkScheduleRow, staffAnchor *timezone.Date, date timezone.Date) (int, bool) {
	dayRows := make([]*WorkScheduleRow, 0, len(rows))
	for _, row := range rows {
		if row == nil || row.ValidFrom.After(date) {
			continue
		}
		if row.ValidUntil != nil && !row.ValidUntil.After(date) {
			continue
		}
		dayRows = append(dayRows, row)
	}
	if len(dayRows) == 0 {
		return 0, false
	}
	anchor := ResolveScheduleAnchor(staffAnchor, dayRows)
	rotationWeek := ResolveRotationWeek(ScheduleRotationLength(dayRows), anchor.StartOfISOWeek(), date.StartOfISOWeek())
	dayIndex := ISODayIndex(date)
	total := 0
	matched := false
	for _, row := range dayRows {
		if row.WeekIndex == rotationWeek && row.DayOfWeek == dayIndex {
			total += row.TargetMinutes
			matched = true
		}
	}
	return total, matched
}

// DailyTargetFromTemplate resolves the target minutes a work-time template
// yields for one calendar day, honouring its rotation. The boolean is false
// when the template is nil or has no entries.
func DailyTargetFromTemplate(template *WorkTimeTemplate, anchor timezone.Date, date timezone.Date) (int, bool) {
	if template == nil || len(template.Entries) == 0 {
		return 0, false
	}
	rotation := template.RotationLength
	if rotation < 1 {
		rotation = 1
	}
	rotationWeek := ResolveRotationWeek(rotation, anchor.StartOfISOWeek(), date.StartOfISOWeek())
	dayIndex := ISODayIndex(date)
	total := 0
	for _, entry := range template.Entries {
		if entry != nil && entry.WeekIndex == rotationWeek && entry.DayOfWeek == dayIndex {
			total += entry.TargetMinutes
		}
	}
	return total, true
}

// WeeklyTargetFromSchedule sums the target minutes a staff member's schedule
// rows yield for the week starting at weekStart (a Monday). The boolean
// reports whether any row applied at all, so callers can distinguish a
// zero-minute week from "no schedule".
func WeeklyTargetFromSchedule(rows []*WorkScheduleRow, staffAnchor *timezone.Date, weekStart timezone.Date) (int, bool) {
	total := 0
	found := false
	for offset := 0; offset < 7; offset++ {
		date := weekStart.AddDays(offset)
		dayTarget, matched := DailyTargetFromSchedule(rows, staffAnchor, date)
		if !matched {
			continue
		}
		total += dayTarget
		found = true
	}
	return total, found
}

// WeeklyTargetsFromTemplate resolves the weekly target minutes a work-time
// template yields for each given week start (Mondays), keyed by that week
// start. Weeks whose rotation index has no entries are absent from the result.
func WeeklyTargetsFromTemplate(template *WorkTimeTemplate, anchor timezone.Date, weekStarts []timezone.Date) map[timezone.Date]int {
	if template == nil || len(template.Entries) == 0 || len(weekStarts) == 0 {
		return nil
	}
	rotation := template.RotationLength
	if rotation < 1 {
		rotation = 1
	}
	targetsByRotationWeek := make(map[int]int, rotation)
	for _, entry := range template.Entries {
		if entry == nil {
			continue
		}
		targetsByRotationWeek[entry.WeekIndex] += entry.TargetMinutes
	}
	targets := make(map[timezone.Date]int, len(weekStarts))
	for _, weekStart := range weekStarts {
		rotationWeek := ResolveRotationWeek(rotation, anchor.StartOfISOWeek(), weekStart.StartOfISOWeek())
		if target, ok := targetsByRotationWeek[rotationWeek]; ok {
			targets[weekStart] = target
		}
	}
	if len(targets) == 0 {
		return nil
	}
	return targets
}
