package api

import (
	"context"
	"fmt"
	"time"
)

// seedShiftSeriesStep shows Dienstplan series around a week of Ferien
// (#3820): one person without holiday care, whose series leaves the Ferien
// out, and one who also works in the Ferien. Everybody else who records time
// works the regular Dienst, so the staff overview expects the people the
// simulation clocks in (#3892); seedTodaysShiftsStep plans today.
type seedShiftSeriesStep struct{}

// The regular Dienst nets the 480 minutes of the seeded Soll, the same day
// the time-tracking history records.
const (
	regularDutyStart        = "08:00"
	regularDutyEnd          = "16:30"
	regularDutyBreakMinutes = 30
)

func (seedShiftSeriesStep) Name() string { return "Seeding shift series around the Ferien" }

type seedCalendarPeriod struct {
	ID              int64   `json:"id"`
	Name            string  `json:"name"`
	PeriodType      string  `json:"period_type"`
	StartDate       string  `json:"start_date"`
	EndDate         string  `json:"end_date"`
	WeekCycleLength int     `json:"week_cycle_length"`
	WeekCycleAnchor *string `json:"week_cycle_anchor"`
	IsActive        bool    `json:"is_active"`
}

func (seedShiftSeriesStep) Run(_ context.Context, rt *Runtime) error {
	if rt == nil || rt.Client == nil || rt.FixedSeeder == nil {
		return fmt.Errorf("shift series demo prerequisites not available")
	}
	staffIDs := orderedSeedStaffIDs(rt.FixedSeeder)
	if len(staffIDs) < 4 {
		return fmt.Errorf("shift series demo needs 4 staff members, got %d", len(staffIDs))
	}
	rt.Client.BindAuth(rt.TenantAuth)

	tomorrow := todaySeedDate().AddDays(1)
	schoolYear, err := seedSchoolYearPeriod(rt, tomorrow)
	if err != nil {
		return err
	}
	if schoolYear == nil {
		fmt.Println("  no school year covers tomorrow, shift series skipped")
		return nil
	}
	ferienStart := seedDate{Time: nextWeekday(tomorrow.AddDays(13).UTCMidnight(), time.Monday)}
	ferienEnd := ferienStart.AddDays(4)
	if _, err := rt.Client.Post("/api/timetable/periods", map[string]any{
		"name": "Herbstferien", "period_type": "holiday",
		"start_date": ferienStart.String(), "end_date": ferienEnd.String(),
		"week_cycle_length": 1, "is_active": true,
	}); err != nil {
		return fmt.Errorf("seed holiday period: %w", err)
	}

	// Two weeks after the Ferien, so the week view shows the gap and the
	// return without filling the whole school year.
	validUntil := ferienEnd.AddDays(17)
	if periodEnd, err := time.Parse(seedDateLayout, schoolYear.EndDate); err == nil && validUntil.After(periodEnd.AddDate(0, 0, 1)) {
		validUntil = seedDate{Time: periodEnd.AddDate(0, 0, 1)}
	}
	series := dutySeries(rt.FixedSeeder, staffIDs)
	for _, payload := range series {
		payload["weekdays"] = []int{1, 2, 3, 4, 5}
		payload["calendar_period_id"] = schoolYear.ID
		payload["valid_from"] = tomorrow.String()
		payload["valid_until"] = validUntil.String()
		if _, err := rt.Client.Post("/api/staff-shifts/series", payload); err != nil {
			return fmt.Errorf("seed shift series: %w", err)
		}
	}
	fmt.Printf("  1 Ferien week and %d shift series created\n", len(series))
	return nil
}

// dutySeries are the Dienste the series plan from tomorrow on: two around the
// Ferien, and the regular Dienst for everybody else who records time. The
// first staff member has single shifts next week (seedStaffShift), so a series
// would collide with them; that person is planned for today only.
func dutySeries(fs *FixedSeeder, staffIDs []int64) []map[string]any {
	series := []map[string]any{
		{
			"staff_id": staffIDs[2], "start_time": "08:00", "end_time": "13:00",
			"notes": "Keine Ferienbetreuung", "include_school_breaks": false,
		},
		{
			"staff_id": staffIDs[3], "start_time": "11:30", "end_time": "16:30", "break_minutes": 30,
			"notes": "Arbeitet auch in den Ferien", "include_school_breaks": true,
		},
	}
	planned := map[int64]bool{staffIDs[0]: true, staffIDs[2]: true, staffIDs[3]: true}
	for _, staffID := range timeTrackingSeedStaffIDs(fs) {
		if planned[staffID] {
			continue
		}
		series = append(series, map[string]any{
			"staff_id": staffID, "start_time": regularDutyStart, "end_time": regularDutyEnd,
			"break_minutes": regularDutyBreakMinutes, "include_school_breaks": true,
		})
	}
	return series
}

// seedTodaysShiftsStep plans today for everybody who has a Dienst. A series
// starts tomorrow at the earliest, but a demo school's simulation clocks
// people in from the first minute (#3892). It runs after the seed's own live
// stamps: their instant check-out would deviate from a planned day and need a
// reason.
type seedTodaysShiftsStep struct{}

func (seedTodaysShiftsStep) Name() string { return "Seeding today's shifts" }

func (seedTodaysShiftsStep) Run(_ context.Context, rt *Runtime) error {
	if rt == nil || rt.Client == nil || rt.FixedSeeder == nil {
		return fmt.Errorf("today's shifts prerequisites not available")
	}
	staffIDs := orderedSeedStaffIDs(rt.FixedSeeder)
	if len(staffIDs) < 4 {
		return fmt.Errorf("today's shifts need 4 staff members, got %d", len(staffIDs))
	}
	rt.Client.BindAuth(rt.TenantAuth)
	return seedTodaysShifts(rt, todaySeedDate(), staffIDs, dutySeries(rt.FixedSeeder, staffIDs))
}

// timeTrackingSeedStaffIDs are the staff members who record their time: all
// but the external ones.
func timeTrackingSeedStaffIDs(fs *FixedSeeder) []int64 {
	ids := make([]int64, 0, len(DemoStaff))
	for _, staff := range DemoStaff {
		id := fs.staffIDs[fmt.Sprintf("%s %s", staff.FirstName, staff.LastName)]
		if id != 0 && staff.Position != "Extern" {
			ids = append(ids, id)
		}
	}
	return ids
}

// seedTodaysShifts plans today for everybody a series covers, and the regular
// Dienst for the first staff member.
func seedTodaysShifts(rt *Runtime, today seedDate, staffIDs []int64, series []map[string]any) error {
	if today.Weekday() == time.Saturday || today.Weekday() == time.Sunday {
		return nil
	}
	shifts := append([]map[string]any{{
		"staff_id": staffIDs[0], "start_time": regularDutyStart, "end_time": regularDutyEnd,
		"break_minutes": regularDutyBreakMinutes,
	}}, series...)
	for _, shift := range shifts {
		body := map[string]any{"date": today.String()}
		for _, key := range []string{"staff_id", "start_time", "end_time", "break_minutes", "notes"} {
			if value, ok := shift[key]; ok {
				body[key] = value
			}
		}
		if _, err := rt.Client.Post("/api/staff-shifts", body); err != nil {
			return fmt.Errorf("seed today's shift: %w", err)
		}
	}
	return nil
}

// seedSchoolYearPeriod returns the active school year containing day, which
// the planning bootstrap guarantees for today; nil when none covers it.
func seedSchoolYearPeriod(rt *Runtime, day seedDate) (*seedCalendarPeriod, error) {
	raw, err := rt.Client.Post("/api/timetable/periods/bootstrap", nil)
	if err != nil {
		return nil, fmt.Errorf("bootstrap planning periods: %w", err)
	}
	var response struct {
		Data struct {
			Periods []seedCalendarPeriod `json:"periods"`
		} `json:"data"`
	}
	if err := parseJSON(raw, &response); err != nil {
		return nil, fmt.Errorf("parse planning periods: %w", err)
	}
	for _, period := range response.Data.Periods {
		if period.PeriodType == "school_year" && period.IsActive &&
			period.StartDate <= day.String() && day.String() <= period.EndDate {
			return &period, nil
		}
	}
	return nil, nil
}
