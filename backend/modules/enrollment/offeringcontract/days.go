package offeringcontract

// dayCodes are the canonical day codes of the enrollment module in ISO
// order: index 0 is Monday (ISO 1). Offerings, form schedules, the weekly
// pickup and arrival schedules and the reports all use them.
var dayCodes = [...]string{"mon", "tue", "wed", "thu", "fri", "sat", "sun"}

// schoolDays is the number of leading day codes that are school days.
const schoolDays = 5

// DayCode returns the code of the ISO weekday (Monday = 1), or "" outside
// 1..7.
func DayCode(isoWeekday int) string {
	if isoWeekday < 1 || isoWeekday > len(dayCodes) {
		return ""
	}
	return dayCodes[isoWeekday-1]
}

// SchoolDayCode returns the code of the ISO weekday when it is a school day
// (Monday to Friday), or "".
func SchoolDayCode(isoWeekday int) string {
	if isoWeekday > schoolDays {
		return ""
	}
	return DayCode(isoWeekday)
}

// DayNumber returns the ISO weekday of the code (Monday = 1), or 0 for an
// unknown code.
func DayNumber(code string) int {
	for i, c := range dayCodes {
		if c == code {
			return i + 1
		}
	}
	return 0
}

// IsDayCode reports whether code is one of the seven day codes.
func IsDayCode(code string) bool {
	return DayNumber(code) > 0
}

// IsSchoolDayCode reports whether code is a school day, Monday to Friday.
func IsSchoolDayCode(code string) bool {
	n := DayNumber(code)
	return n > 0 && n <= schoolDays
}

// SchoolDayCodes returns the school day codes in order, as a fresh slice.
func SchoolDayCodes() []string {
	out := make([]string, schoolDays)
	copy(out, dayCodes[:schoolDays])
	return out
}
