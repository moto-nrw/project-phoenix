package users

import (
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

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
	// KeepAlumni are the graduates the page keeps anyway — a child with a
	// still-open presence stays visible.
	KeepAlumni []int64
	// CareStatus selects the side of the enrolment interval; empty means
	// running.
	CareStatus string
	// CareStatusOn is the caller's frozen calendar day for that boundary.
	CareStatusOn calendar.Date
	// Page is 1-based; a PageSize of 0 returns the whole selection, which is
	// what the exports need.
	Page     int
	PageSize int
}

// CalendarDate is the calendar-day value the retained models carry, named here
// so a composition seam can pass one across without reaching for the shared
// date package itself.
type CalendarDate = calendar.Date
