package schoolcalendar

import (
	"context"
	"errors"
)

// ErrFederalStateUnavailable is returned when the tenant's federal state
// cannot be resolved or names no supported holiday region.
var ErrFederalStateUnavailable = errors.New("federal state is not available")

// NonWorkingDayQuery answers "which days carry no Soll" for the caller's
// tenant. Statutory holidays come from the tenant's federal-state setting
// (#1418 3a), closing days from the school's own closure ranges (#1418 3b).
// Every date uses DateLayout; the sets are keyed by date for O(1) lookups.
type NonWorkingDayQuery interface {
	// TenantHolidays lists the statutory holidays in [from, to]. It never
	// includes closing days: the holidays endpoint and the Feiertagsarbeit
	// warning must not start reporting closures.
	TenantHolidays(ctx context.Context, from, to string) ([]Holiday, error)
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
