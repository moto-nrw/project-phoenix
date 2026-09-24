package timetable

// Staffing rules of a Betreuungsplan block (#1838, #1839, #1840). They are
// pure decisions over counts and staff rows, so every reader (the list
// endpoints, the /gaps read, the deliberately-unstaffed acknowledgement and
// the atomic deviations endpoint) applies the same rule.

// RequiredStaffForChildren returns the number of staff required to supervise
// childrenCount children at the given ratio (max children per staff member),
// rounded up. ratio < 1 is defensively clamped to 1 so a corrupted override
// can never divide by zero or invert the relationship; the registry
// validation (1-30) already prevents this in practice.
func RequiredStaffForChildren(childrenCount, ratio int) int {
	if childrenCount <= 0 {
		return 0
	}
	if ratio < 1 {
		ratio = 1
	}
	return (childrenCount + ratio - 1) / ratio
}

// EffectiveRequiredStaff returns the required staffing level for a block: the
// manual Personalbedarf override when one is set (#1839 — a set value wins
// over the Betreuungsschlüssel-derived figure), otherwise the derived
// RequiredStaffForChildren value. A nil override means "derive"; a negative
// override is defensively treated as 0 (validation already rejects it on
// write, so this only guards a corrupted stored value).
func EffectiveRequiredStaff(override *int, childrenCount, ratio int) int {
	if override != nil {
		if *override < 0 {
			return 0
		}
		return *override
	}
	return RequiredStaffForChildren(childrenCount, ratio)
}

// Understaffing detection (#1840). A block is understaffed when fewer people
// are actually present than the number of planned positions, or when nobody
// is present at all. /gaps, the deliberately-unstaffed acknowledgement and
// the atomic deviations endpoint share this one rule, so a partially covered
// block (two planned, one absent without replacement) is reported and
// acknowledgeable exactly like a fully empty one, and the acknowledgement is
// rejected only when the block is genuinely fully staffed.
//
// Counting:
//   - planned = staff rows that are NOT substitutes (the base-plan positions,
//     whether or not currently absent).
//   - present = staff rows that are NOT absent (planned people still there
//     plus any substitute covering an absence).
//
// A substitute counts toward present (it restores coverage for the absent
// planned person) but not toward planned, so a fully substituted block reads
// as staffed and an unreplaced absence reads as understaffed.

// IsUnderstaffedCounts reports whether a block with planned non-substitute
// positions and present non-absent people is below its planned staffing.
func IsUnderstaffedCounts(present, planned int) bool {
	return present == 0 || present < planned
}

// IsUnderstaffed reports whether the block's staff rows leave it
// understaffed (see IsUnderstaffedCounts).
func IsUnderstaffed(rows []InstanceStaff) bool {
	present, planned := 0, 0
	for _, row := range rows {
		if !row.IsAbsent {
			present++
		}
		if !row.IsSubstitute {
			planned++
		}
	}
	return IsUnderstaffedCounts(present, planned)
}

// Slot sources name which rule supplied a student's arrival or pickup slot on
// a given day. They are plain strings because the wire format is a plain
// JSON string.
const (
	SlotSourceSchedule  = "schedule"
	SlotSourceException = "exception"
	SlotSourceNone      = "none"
)

// ISO weekdays that carry a weekly arrival or pickup schedule.
const (
	isoMonday = 1
	isoFriday = 5
)

// ResolveSlotSource encodes the single "exception overrides the weekly
// schedule" precedence shared by the student day/week view and the
// exception-conflict warnings. A date-specific exception always wins (even
// when it encodes an absence with no time); otherwise a weekly schedule
// applies on weekdays Mon–Fri; otherwise there is no slot. weekday is ISO
// 1..7.
//
// Callers pass whether an exception and a schedule exist for the
// (student, date) and map the returned source back onto their own shape.
func ResolveSlotSource(hasException, hasSchedule bool, weekday int) string {
	if hasException {
		return SlotSourceException
	}
	if weekday >= isoMonday && weekday <= isoFriday && hasSchedule {
		return SlotSourceSchedule
	}
	return SlotSourceNone
}
