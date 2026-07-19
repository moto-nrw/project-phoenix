package schedule

import scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"

// SlotSource identifies which rule supplied a student's arrival/pickup slot on
// a given day. Kept as plain strings because the wire format is a plain JSON
// string — no need for a typed enum.
const (
	SlotSourceSchedule  = "schedule"
	SlotSourceException = "exception"
	SlotSourceNone      = "none"
)

// ResolveSlotSource encodes the single "exception overrides the weekly
// schedule" precedence rule shared by the student day/week view and the
// exception-conflict warnings. A date-specific exception always wins (even when
// it encodes an absence with no time); otherwise a weekly schedule applies on
// weekdays Mon–Fri; otherwise there is no slot. weekday is ISO 1..7.
//
// This is the ONE authoritative implementation of the precedence — callers pass
// in whether an exception / schedule exists for the (student, date) and map the
// returned source back onto their own response shape.
func ResolveSlotSource(hasException, hasSchedule bool, weekday int) string {
	if hasException {
		return SlotSourceException
	}
	if weekday >= scheduleModel.WeekdayMonday && weekday <= scheduleModel.WeekdayFriday && hasSchedule {
		return SlotSourceSchedule
	}
	return SlotSourceNone
}
