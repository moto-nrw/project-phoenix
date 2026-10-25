package api

import (
	"context"
	"fmt"
	"time"
)

// seedShiftSeriesStep shows Dienstplan series around two weeks of
// Herbstferien (#3820): one person without holiday care, whose series leaves
// the Ferien out, and one who also works in the Ferien. Everybody else who
// records time works the regular Dienst, so the staff overview expects the
// people the simulation clocks in (#3892); seedTodaysShiftsStep plans the
// current week up to today.
type seedShiftSeriesStep struct{}

// The regular Dienst nets the 480 minutes of the seeded Soll, the same day
// the time-tracking history records. The Früh- and Spätdienst net the same
// Soll half an hour earlier or later.
const (
	regularDutyStart        = "08:00"
	regularDutyEnd          = "16:30"
	regularDutyBreakMinutes = 30
	earlyDutyStart          = "07:30"
	earlyDutyEnd            = "16:00"
	lateDutyStart           = "08:30"
	lateDutyEnd             = "17:00"
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

// seedHerbstferien are the two Ferien weeks the seed plans around: Monday to
// Friday, starting on the first Monday two weeks from today, like NRW's two
// weeks of Herbstferien. The school's holiday period, its Ferienbetreuung
// and the Träger's holiday period all use them.
func seedHerbstferien(today seedDate) (seedDate, seedDate) {
	start := seedDate{Time: nextWeekday(today.AddDays(14).UTCMidnight(), time.Monday)}
	return start, start.AddDays(11)
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
	types, err := seedDutyShiftTypes(rt)
	if err != nil {
		return err
	}
	ferienStart, ferienEnd := seedHerbstferien(todaySeedDate())
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
	series := dutySeries(rt.FixedSeeder, staffIDs, types)
	for _, payload := range series {
		payload["weekdays"] = []int{1, 2, 3, 4, 5}
		payload["calendar_period_id"] = schoolYear.ID
		payload["valid_from"] = tomorrow.String()
		payload["valid_until"] = validUntil.String()
		if _, err := rt.Client.Post("/api/staff-shifts/series", payload); err != nil {
			return fmt.Errorf("seed shift series: %w", err)
		}
	}
	fmt.Printf("  2 Ferien weeks and %d shift series created\n", len(series))
	return nil
}

// dutySeries are the Dienste the series plan from tomorrow on. The
// Frühbetreuung (07:30) is led by people in the Frühdienst: the third staff
// member, who has no holiday care, the second, and the fifth, who stands in
// when somebody is ill (seedTodaysAbsencesStep). The fourth works the
// Spätdienst up to the Busaufsicht and also in the Ferien. Everybody else
// who records time works the regular Dienst. The first staff member has
// single shifts next week (seedStaffShift), so a series would collide with
// them; that person is planned day by day (currentWeekShifts).
func dutySeries(fs *FixedSeeder, staffIDs []int64, types seedDutyTypes) []map[string]any {
	early := func(staffID int64) map[string]any {
		return map[string]any{
			"staff_id": staffID, "start_time": earlyDutyStart, "end_time": earlyDutyEnd,
			"break_minutes": regularDutyBreakMinutes, "shift_type_id": types.early, "include_school_breaks": true,
		}
	}
	noFerien := early(staffIDs[2])
	noFerien["notes"], noFerien["include_school_breaks"] = "Keine Ferienbetreuung", false
	series := []map[string]any{
		noFerien,
		{
			"staff_id": staffIDs[3], "start_time": lateDutyStart, "end_time": lateDutyEnd,
			"break_minutes": regularDutyBreakMinutes, "shift_type_id": types.late,
			"notes": "Arbeitet auch in den Ferien", "include_school_breaks": true,
		},
		early(staffIDs[1]),
	}
	planned := map[int64]bool{staffIDs[0]: true, staffIDs[1]: true, staffIDs[2]: true, staffIDs[3]: true}
	if len(staffIDs) > substituteTodayStaffIndex {
		series = append(series, early(staffIDs[substituteTodayStaffIndex]))
		planned[staffIDs[substituteTodayStaffIndex]] = true
	}
	for _, staffID := range timeTrackingSeedStaffIDs(fs) {
		if planned[staffID] {
			continue
		}
		series = append(series, map[string]any{
			"staff_id": staffID, "start_time": regularDutyStart, "end_time": regularDutyEnd,
			"break_minutes": regularDutyBreakMinutes, "shift_type_id": types.care, "include_school_breaks": true,
		})
	}
	return series
}

// seedTodaysShiftsStep plans the current week up to today for everybody who
// has a Dienst. A series starts tomorrow at the earliest, but the Dienstplan
// and the calendar open on the whole week, and a demo school's simulation
// clocks people in from the first minute (#3892). It runs after the seed's
// own live stamps: their instant check-out would deviate from a planned day
// and need a reason.
type seedTodaysShiftsStep struct{}

func (seedTodaysShiftsStep) Name() string { return "Seeding this week's shifts" }

func (seedTodaysShiftsStep) Run(_ context.Context, rt *Runtime) error {
	if rt == nil || rt.Client == nil || rt.FixedSeeder == nil {
		return fmt.Errorf("today's shifts prerequisites not available")
	}
	staffIDs := orderedSeedStaffIDs(rt.FixedSeeder)
	if len(staffIDs) < 4 {
		return fmt.Errorf("today's shifts need 4 staff members, got %d", len(staffIDs))
	}
	rt.Client.BindAuth(rt.TenantAuth)
	types, err := seedDutyShiftTypes(rt)
	if err != nil {
		return err
	}
	shifts := currentWeekShifts(todaySeedDate(), staffIDs, dutySeries(rt.FixedSeeder, staffIDs, types), types)
	for _, body := range shifts {
		if _, err := rt.Client.Post("/api/staff-shifts", body); err != nil {
			return fmt.Errorf("seed shift on %s: %w", body["date"], err)
		}
	}
	fmt.Printf("  %d shifts planned for this week up to today\n", len(shifts))
	return nil
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

// currentWeekShifts are the single shifts from Monday of today's week up to
// today, Friday at the latest: the series' hours for everybody a series
// covers, and the Spätdienst for the first staff member, whose Freies Spiel
// runs until 17:00. The person on vacation today is not planned today
// (seedTodaysAbsencesStep). On a weekend they fill the week that just ended.
func currentWeekShifts(today seedDate, staffIDs []int64, series []map[string]any, types seedDutyTypes) []map[string]any {
	plan := append([]map[string]any{{
		"staff_id": staffIDs[0], "start_time": lateDutyStart, "end_time": lateDutyEnd,
		"break_minutes": regularDutyBreakMinutes, "shift_type_id": types.late,
	}}, series...)
	monday := seedDate{Time: mostRecentWeekday(today.UTCMidnight(), time.Monday)}
	onVacation := int64(0)
	if len(staffIDs) > vacationTodayStaffIndex {
		onVacation = staffIDs[vacationTodayStaffIndex]
	}
	var shifts []map[string]any
	for day := monday; !day.After(today.UTCMidnight()) && day.Weekday() != time.Saturday; day = day.AddDays(1) {
		for _, shift := range plan {
			if day.String() != today.String() || shift["staff_id"] != onVacation {
				shifts = append(shifts, singleShiftBody(day, shift))
			}
		}
	}
	return shifts
}

// singleShiftBody is one day of a planned Dienst, without the series-only
// fields.
func singleShiftBody(day seedDate, shift map[string]any) map[string]any {
	body := map[string]any{"date": day.String()}
	for _, key := range []string{"staff_id", "start_time", "end_time", "break_minutes", "shift_type_id", "notes"} {
		if value, ok := shift[key]; ok {
			body[key] = value
		}
	}
	return body
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
