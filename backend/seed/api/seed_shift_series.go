package api

import (
	"context"
	"fmt"
	"time"
)

// seedShiftSeriesStep shows Dienstplan series around a week of Ferien
// (#3820): one person without holiday care, whose series leaves the Ferien
// out, and one who also works in the Ferien.
type seedShiftSeriesStep struct{}

func (seedShiftSeriesStep) Name() string { return "Seeding shift series around the Ferien" }

type seedCalendarPeriod struct {
	ID         int64  `json:"id"`
	PeriodType string `json:"period_type"`
	StartDate  string `json:"start_date"`
	EndDate    string `json:"end_date"`
	IsActive   bool   `json:"is_active"`
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
	for _, payload := range series {
		payload["weekdays"] = []int{1, 2, 3, 4, 5}
		payload["calendar_period_id"] = schoolYear.ID
		payload["valid_from"] = tomorrow.String()
		payload["valid_until"] = validUntil.String()
		if _, err := rt.Client.Post("/api/staff-shifts/series", payload); err != nil {
			return fmt.Errorf("seed shift series: %w", err)
		}
	}
	fmt.Println("  1 Ferien week and 2 shift series created")
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
