package workforce

import (
	"time"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The pure Soll arithmetic every target reader shares: the schedule and
// template rows, the rotation rules over them, and the order in which one
// day's target is resolved. Moved out of the retained time-tracking services
// (#3259); they keep type and function aliases while their consumers migrate.

// ScheduleRow is one date-valid target-hours row of a staff member's schedule
// snapshot. When WeekIndex is non-zero or RotationLength > 1 the row belongs
// to a multi-week rotation (A/B-Wochen).
type ScheduleRow struct {
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
	RotationAnchorDate *calendar.Date
	ValidFrom          calendar.Date
	ValidUntil         *calendar.Date
}

// ScheduleTemplate is a named working-time pattern ("Vollzeit 40h Mo-Fr")
// spanning up to four rotation weeks, shared by any number of staff members.
type ScheduleTemplate struct {
	ID                 int64
	Name               string
	RotationLength     int
	RotationAnchorDate calendar.Date
	Entries            []*ScheduleTemplateEntry
}

// ScheduleTemplateEntry carries the target minutes of one (week index, day of
// week) slot inside a template.
type ScheduleTemplateEntry struct {
	WeekIndex     int
	DayOfWeek     int
	TargetMinutes int
	StartTime     *time.Time
}

// ISODayIndex maps a date's weekday onto the schedule day index
// (Monday=0 … Sunday=6).
func ISODayIndex(d calendar.Date) int {
	weekday := int(d.Weekday())
	if weekday == 0 {
		return 6
	}
	return weekday - 1
}

// ScheduleRotationLength returns the rotation length spanned by the given
// schedule rows (minimum 1).
func ScheduleRotationLength(rows []*ScheduleRow) int {
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
func ResolveRotationWeek(rotationLength int, anchor, date calendar.Date) int {
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
func ResolveScheduleAnchor(staffAnchor *calendar.Date, rows []*ScheduleRow) calendar.Date {
	for _, row := range rows {
		if row != nil && row.RotationAnchorDate != nil && !row.RotationAnchorDate.IsZero() {
			return *row.RotationAnchorDate
		}
	}
	if staffAnchor != nil {
		return *staffAnchor
	}
	var earliest calendar.Date
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
func DailyTargetFromSchedule(rows []*ScheduleRow, staffAnchor *calendar.Date, date calendar.Date) (int, bool) {
	dayRows := make([]*ScheduleRow, 0, len(rows))
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
func DailyTargetFromTemplate(template *ScheduleTemplate, anchor calendar.Date, date calendar.Date) (int, bool) {
	if template == nil || len(template.Entries) == 0 {
		return 0, false
	}
	rotation := max(template.RotationLength, 1)
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
func WeeklyTargetFromSchedule(rows []*ScheduleRow, staffAnchor *calendar.Date, weekStart calendar.Date) (int, bool) {
	total := 0
	found := false
	for offset := range 7 {
		dayTarget, matched := DailyTargetFromSchedule(rows, staffAnchor, weekStart.AddDays(offset))
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
func WeeklyTargetsFromTemplate(template *ScheduleTemplate, anchor calendar.Date, weekStarts []calendar.Date) map[calendar.Date]int {
	if template == nil || len(template.Entries) == 0 || len(weekStarts) == 0 {
		return nil
	}
	rotation := max(template.RotationLength, 1)
	targetsByRotationWeek := make(map[int]int, rotation)
	for _, entry := range template.Entries {
		if entry == nil {
			continue
		}
		targetsByRotationWeek[entry.WeekIndex] += entry.TargetMinutes
	}
	targets := make(map[calendar.Date]int, len(weekStarts))
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

// DayTargetResolver prices the contractual Soll of single days in one fixed
// order (#1418, #1842, #3259):
//
//  1. a Sonderarbeitszeit day (Overrides, already without weekends and
//     statutory holidays) sets its own minutes, even on a closing day;
//  2. a statutory holiday or closing day (NonWorkingDays) is zero (§2 EntgFG:
//     the contractual hours of that day fall away, so absence credits are
//     zero there too);
//  3. date-valid schedule rows win (Schedule, nil when the range has none);
//  4. the assigned work-time template is the fallback (Template, nil when it
//     may not stand in).
type DayTargetResolver struct {
	Overrides      map[string]int
	NonWorkingDays map[calendar.Date]bool
	Schedule       func(calendar.Date) (int, bool)
	Template       func(calendar.Date) (int, bool)
}

// TargetFor returns the Soll of one day in minutes.
func (r *DayTargetResolver) TargetFor(d calendar.Date) int {
	if minutes, ok := r.Overrides[d.String()]; ok {
		return minutes
	}
	if r.NonWorkingDays[d] {
		return 0
	}
	if r.Schedule != nil {
		target, _ := r.Schedule(d)
		return target
	}
	if r.Template != nil {
		target, _ := r.Template(d)
		return target
	}
	return 0
}

// SourceFor names what set the Soll of a day; empty is the regular schedule.
func (r *DayTargetResolver) SourceFor(d calendar.Date) string {
	if _, ok := r.Overrides[d.String()]; ok {
		return TargetSourceOverride
	}
	return ""
}
