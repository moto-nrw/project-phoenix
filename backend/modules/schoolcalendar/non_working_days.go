package schoolcalendar

import (
	"context"
	"errors"
	"fmt"
	"time"
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

func (m *Module) TenantHolidays(ctx context.Context, from, to string) ([]Holiday, error) {
	if err := validateHolidayWindow(from, to); err != nil {
		return nil, err
	}
	region, err := m.tenantRegion(ctx)
	if err != nil {
		return nil, err
	}
	return m.engine.ListHolidays(ctx, region, from, to)
}

func (m *Module) TenantHolidayDates(ctx context.Context, from, to string) (map[string]bool, error) {
	if err := validateHolidayWindow(from, to); err != nil {
		return nil, err
	}
	region, err := m.tenantRegion(ctx)
	if err != nil {
		return nil, err
	}
	return m.engine.HolidayDates(ctx, region, from, to)
}

func (m *Module) ClosingDayDates(ctx context.Context, from, to string) (map[string]bool, error) {
	if from == "" || to == "" {
		return nil, invalidClosingDay("closing day window needs both from and to dates")
	}
	if err := validateDate(from, "closing day window from", invalidClosingDay); err != nil {
		return nil, err
	}
	if err := validateDate(to, "closing day window to", invalidClosingDay); err != nil {
		return nil, err
	}
	set := make(map[string]bool)
	if to < from {
		return set, nil
	}
	ranges, err := m.engine.ListClosingDays(ctx, ClosingDayFilter{OverlappingFrom: from, OverlappingTo: to})
	if err != nil {
		return nil, err
	}
	for _, closing := range ranges {
		start := max(closing.StartDate, from)
		end := min(closing.EndDate, to)
		for day := start; day <= end; day = nextDay(day) {
			set[day] = true
		}
	}
	return set, nil
}

func (m *Module) NonWorkingDayDates(ctx context.Context, from, to string) (map[string]bool, error) {
	set, err := m.TenantHolidayDates(ctx, from, to)
	if err != nil {
		return nil, err
	}
	closing, err := m.ClosingDayDates(ctx, from, to)
	if err != nil {
		return nil, err
	}
	for day := range closing {
		set[day] = true
	}
	return set, nil
}

// tenantRegion resolves the tenant's federal state and checks that it names
// a supported holiday region; the setting is the single source of truth.
func (m *Module) tenantRegion(ctx context.Context) (string, error) {
	region, err := m.engine.FederalState(ctx)
	if err != nil {
		return "", fmt.Errorf("%w: %w", ErrFederalStateUnavailable, err)
	}
	if !m.engine.ValidHolidayRegion(region) {
		return "", fmt.Errorf("%w: unsupported region %q", ErrFederalStateUnavailable, region)
	}
	return region, nil
}

// nextDay advances a DateLayout day by one; the input was validated, so a
// parse failure cannot happen and would only end the loop early.
func nextDay(day string) string {
	value, err := time.Parse(DateLayout, day)
	if err != nil {
		return "9999-12-31"
	}
	return value.AddDate(0, 0, 1).Format(DateLayout)
}
