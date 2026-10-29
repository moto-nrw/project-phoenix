package statistics

// Belegung nach Uhrzeit (#4764). Definitions (binding, mirrored on the screen):
//
//   - raster   = 30-minute slots in Berlin wall-clock time; a child counts in a
//     slot when its window overlaps the slot, so [07:45, 16:00) counts from
//     07:30 up to and including 15:30
//   - plan     = the child is enrolled, Care Plan expects it (booked, not
//     cancelled with "Kommt heute nicht"), it carries no sick, excused or
//     class-trip status day, and the plan names an arrival before the pickup;
//     the window is [arrival, pickup)
//   - actual   = the attendance rows of the day, [check-in, check-out); an open
//     row runs until now on today and counts only in its check-in slot on an
//     earlier day, because nobody recorded when the child left
//   - days     = the care days of the report (weekends only where they follow
//     Friday's plan, minus public holidays, closing days and holiday periods);
//     averages divide by the care days of the weekday that still have their
//     data: plan values stop at the status-day retention cutoff (without the
//     sick days the plan would count absent children), actual values at the
//     attendance retention cutoff; in a period, today joins the actual values
//     only once it is over

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence/internal/ports"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

const (
	slotsPerDay = 24 * 60 / studentpresence.OccupancySlotMinutes
	// The default axis covers 07:00 to 17:00; data outside widens it.
	defaultFirstSlot = 7 * 60 / studentpresence.OccupancySlotMinutes
	defaultLastSlot  = 17*60/studentpresence.OccupancySlotMinutes - 1
)

// minuteWindow is a half-open window [start, end) in minutes after midnight.
type minuteWindow struct{ start, end int }

// daySlots counts children per slot for one day.
type daySlots [slotsPerDay]int

// addWindows counts one child once per slot its windows touch, however many
// rows overlap there.
func (d *daySlots) addWindows(windows []minuteWindow) {
	var hit [slotsPerDay]bool
	for _, w := range windows {
		for slot := range slotsPerDay {
			slotStart := slot * studentpresence.OccupancySlotMinutes
			if w.start < slotStart+studentpresence.OccupancySlotMinutes && w.end > slotStart {
				hit[slot] = true
			}
		}
	}
	for slot, ok := range hit {
		if ok {
			d[slot]++
		}
	}
}

// occupancyInputs are the facts the section counts.
type occupancyInputs struct {
	students   []*ports.StatisticsStudent
	careDays   map[calendar.Date]bool
	statusDays []ports.StatusDay
	planned    map[int64]map[calendar.Date]ports.PlannedCareDay
	attendance []*ports.Attendance
	// planFrom and actualFrom are the retention cutoffs; zero means none.
	planFrom   calendar.Date
	actualFrom calendar.Date
	today      calendar.Date
	now        time.Time
	// singleDay reports a single date, where today shows its live numbers.
	singleDay bool
}

func (s *service) fillOccupancySection(ctx context.Context, report *studentpresence.StatisticsReport, filters studentpresence.StatisticsFilters, today calendar.Date, now time.Time, students []*ports.StatisticsStudent, inputs attendanceInputs) error {
	if s.cfg.PlannedCareDays == nil || s.cfg.AttendanceIntervals == nil || s.cfg.Retention == nil {
		return fmt.Errorf("statistics occupancy: planned care days, attendance and retention are required")
	}
	planDays, actualDays, err := s.occupancyRetention(ctx)
	if err != nil {
		return err
	}
	in := occupancyInputs{
		students:   students,
		careDays:   inputs.careDays,
		statusDays: inputs.statusDays,
		planFrom:   today.AddDays(-planDays),
		actualFrom: today.AddDays(-actualDays),
		today:      today,
		now:        now,
		singleDay:  filters.From == filters.To,
	}
	ids := make([]int64, 0, len(students))
	for _, st := range students {
		if st != nil && st.EnrolledOn != nil {
			ids = append(ids, st.ID)
		}
	}
	if len(ids) > 0 {
		if in.planned, err = s.cfg.PlannedCareDays.PlannedCareDays(ctx, ids, filters.From, filters.To); err != nil {
			return fmt.Errorf("load planned care days: %w", err)
		}
		if in.attendance, err = s.loadAttendanceRows(ctx, ids, filters.From, filters.To, in.actualFrom); err != nil {
			return err
		}
	}
	occupancy := buildOccupancy(in)
	occupancy.PlanDataDays, occupancy.PlanDataFrom = planDays, in.planFrom
	occupancy.ActualDataDays, occupancy.ActualDataFrom = actualDays, in.actualFrom
	report.Occupancy = &occupancy
	return nil
}

// occupancyRetention resolves how far back the status days and the
// attendance reach.
func (s *service) occupancyRetention(ctx context.Context) (int, int, error) {
	planDays, err := s.cfg.Retention.StatusDayRetentionDays(ctx)
	if err != nil {
		return 0, 0, fmt.Errorf("load status day retention: %w", err)
	}
	actualDays, err := s.cfg.Retention.AttendanceRetentionDays(ctx)
	if err != nil {
		return 0, 0, fmt.Errorf("load attendance retention: %w", err)
	}
	return planDays, actualDays, nil
}

// loadAttendanceRows reads no further back than the retention cutoff the
// screen names, whether or not the cleanup job has caught up with it yet.
func (s *service) loadAttendanceRows(ctx context.Context, ids []int64, from, to, actualFrom calendar.Date) ([]*ports.Attendance, error) {
	if from.Before(actualFrom) {
		from = actualFrom
	}
	if to.Before(from) {
		return nil, nil
	}
	rows, _, err := s.cfg.AttendanceIntervals.ListAttendance(ctx, studentpresence.AttendanceFilter{
		StudentIDs: ids, FromDate: from.String(), UntilDate: to.String(),
	})
	if err != nil {
		return nil, fmt.Errorf("load attendance intervals: %w", err)
	}
	return rows, nil
}

// weekdaySeries are the counted days of one weekday, plan and actual apart.
type weekdaySeries struct{ plan, actual []daySlots }

// buildOccupancy counts every care day and aggregates the days per weekday.
func buildOccupancy(in occupancyInputs) studentpresence.StatisticsOccupancy {
	planned, untimed := plannedSlotsByDay(in)
	byWeekday, first, last := weekdaySeriesOf(in, planned)
	out := studentpresence.StatisticsOccupancy{
		SlotMinutes:     studentpresence.OccupancySlotMinutes,
		Weekdays:        []studentpresence.StatisticsOccupancyWeekday{},
		UntimedPlanDays: untimed,
	}
	for weekday := 1; weekday <= 7; weekday++ {
		series, ok := byWeekday[weekday]
		if !ok {
			continue
		}
		row := studentpresence.StatisticsOccupancyWeekday{
			Weekday: weekday, PlanDays: len(series.plan), ActualDays: len(series.actual),
		}
		for slot := first; slot <= last; slot++ {
			cell := studentpresence.StatisticsOccupancySlot{StartMinute: slot * studentpresence.OccupancySlotMinutes}
			cell.PlannedAverage, cell.PlannedMax = averageAndMax(series.plan, slot)
			cell.ActualAverage, cell.ActualMax = averageAndMax(series.actual, slot)
			row.Slots = append(row.Slots, cell)
		}
		out.Weekdays = append(out.Weekdays, row)
	}
	return out
}

// weekdaySeriesOf sorts every care day into its weekday, keeps only the days
// whose data is still there, and returns the slot axis the data needs.
func weekdaySeriesOf(in occupancyInputs, planned map[calendar.Date]daySlots) (map[int]*weekdaySeries, int, int) {
	actual := actualSlotsByDay(in)
	byWeekday := map[int]*weekdaySeries{}
	first, last := defaultFirstSlot, defaultLastSlot
	for day := range in.careDays {
		series, ok := byWeekday[isoWeekday(day)]
		if !ok {
			series = &weekdaySeries{}
			byWeekday[isoWeekday(day)] = series
		}
		if !day.Before(in.planFrom) {
			series.plan = append(series.plan, planned[day])
			first, last = widenAxis(first, last, planned[day])
		}
		if in.countsActual(day) {
			series.actual = append(series.actual, actual[day])
			first, last = widenAxis(first, last, actual[day])
		}
	}
	return byWeekday, first, last
}

// countsActual reports whether the day's attendance is complete and kept.
func (in occupancyInputs) countsActual(day calendar.Date) bool {
	if day.Before(in.actualFrom) || day.After(in.today) {
		return false
	}
	return day.Before(in.today) || in.singleDay
}

// plannedSlotsByDay counts the planned children per retained care day and the
// child days the plan expects without a usable time pair.
func plannedSlotsByDay(in occupancyInputs) (map[calendar.Date]daySlots, int) {
	absent := make(map[dayKey]bool, len(in.statusDays))
	for key, status := range classifyStatusDays(in.statusDays) {
		absent[key] = status == statusSick || status == statusExcused
	}
	out := make(map[calendar.Date]daySlots, len(in.careDays))
	untimed := 0
	for day := range in.careDays {
		var slots daySlots
		if day.Before(in.planFrom) {
			out[day] = slots
			continue
		}
		for _, st := range in.students {
			if st == nil || st.EnrolledOn == nil || !st.EnrolledOn(day, in.today) || absent[dayKey{st.ID, day}] {
				continue
			}
			plan := in.planned[st.ID][day]
			if !plan.Expected {
				continue
			}
			window, ok := plannedWindow(plan)
			if !ok {
				untimed++
				continue
			}
			slots.addWindows([]minuteWindow{window})
		}
		out[day] = slots
	}
	return out, untimed
}

func plannedWindow(plan ports.PlannedCareDay) (minuteWindow, bool) {
	if plan.Arrival == nil || plan.Pickup == nil {
		return minuteWindow{}, false
	}
	window := minuteWindow{
		start: plan.Arrival.Hour()*60 + plan.Arrival.Minute(),
		end:   plan.Pickup.Hour()*60 + plan.Pickup.Minute(),
	}
	return window, window.end > window.start
}

// actualSlotsByDay counts the children per care day from their attendance
// rows.
func actualSlotsByDay(in occupancyInputs) map[calendar.Date]daySlots {
	out := map[calendar.Date]daySlots{}
	for day, byStudent := range actualWindowsByDay(in) {
		var slots daySlots
		for _, ws := range byStudent {
			slots.addWindows(ws)
		}
		out[day] = slots
	}
	return out
}

// actualWindowsByDay groups the attendance windows per care day and child.
// Only enrolled children of the report population count, so the group and
// class filters hold for the actual values too.
func actualWindowsByDay(in occupancyInputs) map[calendar.Date]map[int64][]minuteWindow {
	enrolled := make(map[int64]*ports.StatisticsStudent, len(in.students))
	for _, st := range in.students {
		if st != nil && st.EnrolledOn != nil {
			enrolled[st.ID] = st
		}
	}
	windows := map[calendar.Date]map[int64][]minuteWindow{}
	for _, row := range in.attendance {
		if row == nil || !in.careDays[row.Date] {
			continue
		}
		st, ok := enrolled[row.StudentID]
		if !ok || !st.EnrolledOn(row.Date, in.today) {
			continue
		}
		if windows[row.Date] == nil {
			windows[row.Date] = map[int64][]minuteWindow{}
		}
		windows[row.Date][row.StudentID] = append(windows[row.Date][row.StudentID], actualWindow(row, row.Date, in.today, in.now))
	}
	return windows
}

// actualWindow maps one attendance row onto the wall clock of its day.
func actualWindow(row *ports.Attendance, day, today calendar.Date, now time.Time) minuteWindow {
	start := minutesOnDay(row.CheckInTime, day)
	var end int
	switch {
	case row.CheckOutTime != nil:
		end = minutesOnDay(*row.CheckOutTime, day)
	case day == today:
		end = minutesOnDay(now, day)
	default:
		end = start
	}
	// A check-out in the same minute, or an open row of an earlier day, still
	// puts the child into its check-in slot.
	if end <= start {
		end = start + 1
	}
	return minuteWindow{start: start, end: end}
}

// minutesOnDay is the Berlin wall-clock minute of t, clamped to the day: an
// instant before the day is 0, one after it is the end of the day.
func minutesOnDay(t time.Time, day calendar.Date) int {
	berlin := t.In(calendar.Berlin)
	date := calendar.DateFromTime(berlin)
	switch {
	case date.Before(day):
		return 0
	case date.After(day):
		return 24 * 60
	}
	return berlin.Hour()*60 + berlin.Minute()
}

func widenAxis(first, last int, slots daySlots) (int, int) {
	for slot, count := range slots {
		if count == 0 {
			continue
		}
		if slot < first {
			first = slot
		}
		if slot > last {
			last = slot
		}
	}
	return first, last
}

// averageAndMax rounds the mean to one decimal; both are nil without days.
func averageAndMax(days []daySlots, slot int) (*float64, *int) {
	if len(days) == 0 {
		return nil, nil
	}
	sum, peak := 0, 0
	for _, d := range days {
		sum += d[slot]
		if d[slot] > peak {
			peak = d[slot]
		}
	}
	avg := float64(int64(float64(sum)/float64(len(days))*10+0.5)) / 10
	return &avg, &peak
}

// isoWeekday maps Sunday to 7.
func isoWeekday(day calendar.Date) int {
	weekday := int(day.Weekday())
	if weekday == 0 {
		return 7
	}
	return weekday
}

func filterStudentsByClass(students []*ports.StatisticsStudent, classes []string) []*ports.StatisticsStudent {
	if len(classes) == 0 {
		return students
	}
	wanted := make(map[string]bool, len(classes))
	for _, class := range classes {
		wanted[class] = true
	}
	out := make([]*ports.StatisticsStudent, 0, len(students))
	for _, st := range students {
		if st != nil && wanted[st.SchoolClass] {
			out = append(out, st)
		}
	}
	return out
}
