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
// schedule entries: an explicit staff-level anchor wins, otherwise the
// earliest valid_from among the entries.
func ResolveScheduleAnchor(staffAnchor *timezone.Date, entries []*StaffWorkSchedule) timezone.Date {
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

// WeeklyTargetFromSchedule sums the target minutes a staff member's schedule
// entries yield for the week starting at weekStart (a Monday). Entries apply
// per day when valid_from <= day and valid_until is unset or after the day
// (valid_until is exclusive, matching the repository predicate). The boolean
// reports whether any entry applied at all, so callers can distinguish a
// zero-minute week from "no schedule".
func WeeklyTargetFromSchedule(entries []*StaffWorkSchedule, staffAnchor *timezone.Date, weekStart timezone.Date) (int, bool) {
	total := 0
	found := false
	for offset := 0; offset < 7; offset++ {
		date := weekStart.AddDays(offset)
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
			continue
		}
		anchor := ResolveScheduleAnchor(staffAnchor, dayEntries)
		rotationWeek := ResolveWeekIndex(ScheduleRotationLength(dayEntries), MondayOf(anchor), MondayOf(date))
		dayIndex := ISODayIndex(date)
		for _, e := range dayEntries {
			if e.WeekIndex == rotationWeek && e.DayOfWeek == dayIndex {
				total += e.TargetMinutes
				found = true
			}
		}
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
