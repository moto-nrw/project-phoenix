package config

import (
	"github.com/moto-nrw/project-phoenix/internal/timezone"
)

// This file holds pure weekly-target derivations shared by consumers that
// resolve contractual Soll minutes from work-time data (staff schedule
// snapshots and work-time models). They mirror the resolution semantics used
// by the time-tracking weekly summaries (services/active): a date-valid
// staff_work_schedules row set wins, the assigned work-time model is the
// fallback, and rotation weeks anchor on Monday-aligned week starts.

// MondayOf returns the Monday of the ISO week containing d.
func MondayOf(d timezone.Date) timezone.Date {
	weekday := int(d.Weekday())
	if weekday == 0 {
		weekday = 7
	}
	return d.AddDays(1 - weekday)
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
// schedule entries (minimum 1).
func ScheduleRotationLength(entries []*StaffWorkSchedule) int {
	rotation := 1
	for _, e := range entries {
		if e != nil && e.RotationLength > rotation {
			rotation = e.RotationLength
		}
	}
	return rotation
}

// ResolveScheduleAnchor returns the rotation anchor for a staff member's
// schedule entries, in descending order of historical truth (#1842):
//
//  1. the anchor persisted on the entries themselves — the one the version was
//     actually written with, immune to later schedule edits;
//  2. the staff-level anchor — the live value, which every schedule change
//     overwrites, so it is only correct for the CURRENT version;
//  3. the earliest valid_from among the entries.
//
// Entries reaching step 2 predate migration 1.15.203 and had no staff anchor
// to backfill; step 2 then reproduces the pre-1.15.203 behaviour exactly.
func ResolveScheduleAnchor(staffAnchor *timezone.Date, entries []*StaffWorkSchedule) timezone.Date {
	for _, e := range entries {
		if e != nil && e.RotationAnchorDate != nil && !e.RotationAnchorDate.IsZero() {
			return *e.RotationAnchorDate
		}
	}
	if staffAnchor != nil {
		return *staffAnchor
	}
	var earliest timezone.Date
	for _, e := range entries {
		if e == nil {
			continue
		}
		if earliest.IsZero() || e.ValidFrom.Before(earliest) {
			earliest = e.ValidFrom
		}
	}
	return earliest
}

// DailyTargetFromSchedule resolves the target minutes a staff member's
// schedule entries yield for one calendar day. Entries apply when
// valid_from <= day and valid_until is unset or after the day (valid_until
// is exclusive, matching the repository predicate). The boolean reports
// whether an entry MATCHED the day's rotation week + weekday (the historical
// WeeklyTargetFromSchedule semantics) — a date-valid schedule that simply
// has no row for this day/rotation returns (0, false), so callers can
// distinguish "day off per plan" from a plain zero.
func DailyTargetFromSchedule(entries []*StaffWorkSchedule, staffAnchor *timezone.Date, date timezone.Date) (int, bool) {
	dayEntries := make([]*StaffWorkSchedule, 0, len(entries))
	for _, e := range entries {
		if e == nil || e.ValidFrom.After(date) {
			continue
		}
		if e.ValidUntil != nil && !e.ValidUntil.After(date) {
			continue
		}
		dayEntries = append(dayEntries, e)
	}
	if len(dayEntries) == 0 {
		return 0, false
	}
	anchor := ResolveScheduleAnchor(staffAnchor, dayEntries)
	rotationWeek := ResolveWeekIndex(ScheduleRotationLength(dayEntries), MondayOf(anchor), MondayOf(date))
	dayIndex := ISODayIndex(date)
	total := 0
	matched := false
	for _, e := range dayEntries {
		if e.WeekIndex == rotationWeek && e.DayOfWeek == dayIndex {
			total += e.TargetMinutes
			matched = true
		}
	}
	return total, matched
}

// DailyTargetFromModel resolves the target minutes a work-time model yields
// for one calendar day, honouring the model's rotation. The boolean is false
// when the model is nil or has no entries.
func DailyTargetFromModel(model *WorkTimeModel, anchor timezone.Date, date timezone.Date) (int, bool) {
	if model == nil || len(model.Entries) == 0 {
		return 0, false
	}
	rotation := model.RotationLength
	if rotation < 1 {
		rotation = 1
	}
	rotationWeek := ResolveWeekIndex(rotation, MondayOf(anchor), MondayOf(date))
	dayIndex := ISODayIndex(date)
	total := 0
	for _, e := range model.Entries {
		if e != nil && e.WeekIndex == rotationWeek && e.DayOfWeek == dayIndex {
			total += e.TargetMinutes
		}
	}
	return total, true
}

// WeeklyTargetFromSchedule sums the target minutes a staff member's schedule
// entries yield for the week starting at weekStart (a Monday). The boolean
// reports whether any entry applied at all, so callers can distinguish a
// zero-minute week from "no schedule".
func WeeklyTargetFromSchedule(entries []*StaffWorkSchedule, staffAnchor *timezone.Date, weekStart timezone.Date) (int, bool) {
	total := 0
	found := false
	for offset := 0; offset < 7; offset++ {
		date := weekStart.AddDays(offset)
		dayTarget, matched := DailyTargetFromSchedule(entries, staffAnchor, date)
		if !matched {
			continue
		}
		total += dayTarget
		found = true
	}
	return total, found
}

// WeeklyTargetsFromModel resolves the weekly target minutes a work-time model
// yields for each given week start (Mondays), keyed by that week start. Weeks
// whose rotation index has no entries are absent from the result.
func WeeklyTargetsFromModel(model *WorkTimeModel, anchor timezone.Date, weekStarts []timezone.Date) map[timezone.Date]int {
	if model == nil || len(model.Entries) == 0 || len(weekStarts) == 0 {
		return nil
	}
	rotation := model.RotationLength
	if rotation < 1 {
		rotation = 1
	}
	targetsByRotationWeek := make(map[int]int, rotation)
	for _, e := range model.Entries {
		if e == nil {
			continue
		}
		targetsByRotationWeek[e.WeekIndex] += e.TargetMinutes
	}
	targets := make(map[timezone.Date]int, len(weekStarts))
	for _, weekStart := range weekStarts {
		rotationWeek := ResolveWeekIndex(rotation, MondayOf(anchor), MondayOf(weekStart))
		if target, ok := targetsByRotationWeek[rotationWeek]; ok {
			targets[weekStart] = target
		}
	}
	if len(targets) == 0 {
		return nil
	}
	return targets
}
