package users

import "github.com/moto-nrw/project-phoenix/internal/timezone"

// Care-status windows of the staff directory: the two sides of the enrolment
// interval, and the unbounded form for callers that manage both.
const (
	StudentCareStatusRunning = "running"
	StudentCareStatusEnded   = "ended"
	StudentCareStatusAll     = "all"
)

// StudentDirectoryFilter narrows the staff directory page. Every field is
// optional; an empty filter is the tenant's whole non-alumni roster.
//
// It lives beside the retained row because both the HTTP adapter that builds
// it and the composition seam that hands it to the owner carry it, and neither
// may reach the other's package.
type StudentDirectoryFilter struct {
	// IDs restricts the page to these children; empty means no restriction.
	IDs []int64
	// SchoolClasses matches any of them, trimmed and case-insensitively.
	SchoolClasses []string
	// GradeLevels matches the first run of digits in the free-text class
	// name, so "3a" and "Klasse 3a" both count as grade 3 and "13a" does not.
	GradeLevels []int
	// GuardianNameContains is a case-insensitive substring match.
	GuardianNameContains string
	// KeepAlumni are the graduates the page keeps anyway — a child with a
	// still-open presence stays visible.
	KeepAlumni []int64
	// CareStatus selects the side of the enrolment interval; empty means
	// running.
	CareStatus string
	// CareStatusOn is the caller's frozen calendar day for that boundary.
	CareStatusOn timezone.Date
	// Page is 1-based; a PageSize of 0 returns the whole selection, which is
	// what the exports need.
	Page     int
	PageSize int
}

// OptionalCalendarDate reads a calendar day the owner rendered, or nil when it
// is unset. A value the owner cannot have produced becomes unset rather than a
// wrong day.
func OptionalCalendarDate(value string) *timezone.Date {
	if value == "" {
		return nil
	}
	parsed, err := timezone.ParseDate(value)
	if err != nil {
		return nil
	}
	return &parsed
}

// CalendarDate is the calendar-day value the retained models carry, named here
// so a composition seam can pass one across without reaching for the shared
// date package itself.
type CalendarDate = timezone.Date

// RenderCalendarDate is the inverse of OptionalCalendarDate: the owner's wire
// form of a day, empty when there is none.
func RenderCalendarDate(value *CalendarDate) string {
	if value == nil {
		return ""
	}
	return value.String()
}
