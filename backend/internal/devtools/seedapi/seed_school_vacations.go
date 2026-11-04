package api

import (
	"context"
	"fmt"
	"time"
)

// seedSchoolVacationsStep stores the school vacations of the school year for
// every federal state through the local-only seed route (#5345). The real
// vacations come from the daily worker sync with the OpenHolidays API; the
// seed never asks it, so a seeded stack shows vacations offline. The first
// vacation is next week, inside the two weeks the planning demo
// materializes, so the Betreuungsplan shows the vacation hint for its
// Regeltermine.
type seedSchoolVacationsStep struct{}

func (seedSchoolVacationsStep) Name() string { return "Seeding school vacations" }

func (seedSchoolVacationsStep) Run(_ context.Context, rt *Runtime) error {
	if rt == nil || rt.Client == nil {
		return fmt.Errorf("school vacation demo prerequisites not available")
	}
	rt.Client.BindAuth(rt.TenantAuth)
	vacations := seedSchoolVacationWindows(todaySeedDate())
	if _, err := rt.Client.PostWithHeaders("/api/timetable/school-vacations/seed", map[string]any{"vacations": vacations},
		map[string]string{seedTokenHeader: "true"}); err != nil {
		return fmt.Errorf("seed school vacations: %w", err)
	}
	fmt.Printf("  %d school vacations stored for every federal state\n", len(vacations))
	return nil
}

// seedSchoolVacationWindows names a one-week vacation from next Monday and
// the Christmas, Easter and summer vacations after it, with the dates the
// NRW vacations usually have.
func seedSchoolVacationWindows(today seedDate) []map[string]string {
	autumnStart := seedDate{Time: nextWeekday(today.AddDays(1).UTCMidnight(), time.Monday)}
	autumnEnd := autumnStart.AddDays(4)
	year := autumnEnd.Year()
	if !autumnEnd.Before(time.Date(year, time.December, 23, 0, 0, 0, 0, time.UTC)) {
		year++
	}
	window := func(name string, start, end time.Time) map[string]string {
		return map[string]string{"name": name, "start_date": start.Format(seedDateLayout), "end_date": end.Format(seedDateLayout)}
	}
	return []map[string]string{
		window("Herbstferien", autumnStart.Time, autumnEnd.Time),
		window("Weihnachtsferien", time.Date(year, time.December, 23, 0, 0, 0, 0, time.UTC), time.Date(year+1, time.January, 6, 0, 0, 0, 0, time.UTC)),
		window("Osterferien", time.Date(year+1, time.March, 29, 0, 0, 0, 0, time.UTC), time.Date(year+1, time.April, 10, 0, 0, 0, 0, time.UTC)),
		window("Sommerferien", time.Date(year+1, time.July, 12, 0, 0, 0, 0, time.UTC), time.Date(year+1, time.August, 24, 0, 0, 0, 0, time.UTC)),
	}
}
