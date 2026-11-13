package schoolcalendar

import (
	"context"
	"errors"

	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
)

// ErrFederalStateUnavailable is returned when the tenant's federal state
// cannot be resolved or names no supported holiday region.
var ErrFederalStateUnavailable = errors.New("federal state is not available")

// HolidayRegion is the federal state whose statutory holidays apply to a
// tenant: its ISO code (operations.federal_state) and its German name.
type HolidayRegion struct {
	Code string `json:"code"`
	Name string `json:"name"`
}

// NonWorkingDayQuery answers "which days carry no Soll" for the caller's
// tenant. Statutory holidays come from the tenant's federal-state setting
// (#1418 3a), closing days from the school's own closure ranges (#1418 3b).
// Every date uses DateLayout; the sets are keyed by date for O(1) lookups.
type NonWorkingDayQuery interface {
	// TenantHolidays lists the statutory holidays in [from, to]. It never
	// includes closing days: the holidays endpoint and the Feiertagsarbeit
	// warning must not start reporting closures.
	TenantHolidays(ctx context.Context, from, to string) ([]Holiday, error)
	// TenantHolidayRegion names the federal state TenantHolidays computes
	// for, so a school can check which holidays apply (#4775).
	TenantHolidayRegion(ctx context.Context) (HolidayRegion, error)
	// TenantHolidayDates is TenantHolidays as a date set.
	TenantHolidayDates(ctx context.Context, from, to string) (map[string]bool, error)
	// ClosingDayDates expands the closure ranges touching [from, to] into a
	// date set clamped to the window.
	ClosingDayDates(ctx context.Context, from, to string) (map[string]bool, error)
	// NonWorkingDayDates is the union of holidays and closing days. A day
	// that is both collapses into one key, so it can never be subtracted
	// twice from a Soll.
	NonWorkingDayDates(ctx context.Context, from, to string) (map[string]bool, error)
}

// CareCalendarQuery reads the days of a window on which the school's own
// calendar gives no care, its closing days and its active holiday periods,
// in one statement; each is a date set clamped to the window (#5994).
type CareCalendarQuery interface {
	CareCalendarDays(ctx context.Context, from, to calendar.Date) (closing, holidays map[calendar.Date]bool, err error)
}
