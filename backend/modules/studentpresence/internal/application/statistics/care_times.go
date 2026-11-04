package statistics

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence/internal/ports"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// Soll und Ist je Kind (#4774). The definitions are documented on
// studentpresence.CareTimeToleranceMinutes; this file applies them to the
// care plan (Soll, through ports.PlannedCareDays, the same read the occupancy
// by time of day uses) and the attendance (Ist, ports.AttendanceIntervals).
// Like the occupancy it reads no further back than the attendance and
// status-day retention: older days may have lost their check-ins or sick
// notes and would read as false shortfalls.

var errCareTimesNotConfigured = errors.New("statistics: care time sources are not configured")

func (s *service) StudentCareTimes(ctx context.Context, studentID int64, from, to calendar.Date) (*studentpresence.StudentCareTimes, error) {
	today := calendar.DateFromTime(s.cfg.Now())
	if err := s.validate(studentpresence.StatisticsFilters{From: from, To: to}, today); err != nil {
		return nil, err
	}
	student, err := s.careTimeStudent(ctx, studentID, from, to, today)
	if err != nil {
		return nil, err
	}
	compared, dataFrom, err := s.compareCareTimes(ctx, []*ports.StatisticsStudent{student}, from, to, today)
	if err != nil {
		return nil, err
	}
	result := compared[studentID]
	return &studentpresence.StudentCareTimes{
		StudentID: studentID, From: from, To: to, DataFrom: dataFrom,
		ToleranceMinutes: studentpresence.CareTimeToleranceMinutes,
		Days:             result.days, Summary: result.summary,
	}, nil
}

func (s *service) CareTimeOverruns(ctx context.Context, filters studentpresence.CareTimeFilters, actor studentpresence.StatisticsActor) (*studentpresence.CareTimeOverruns, error) {
	result, err := s.careTimeOverruns(ctx, filters)
	if err != nil {
		return nil, err
	}
	if err := s.recordAccess(ctx, careTimeAccessScope(filters), actor, "view", "", true); err != nil {
		return nil, err
	}
	return result, nil
}

func (s *service) CareTimeOverrunsForExport(ctx context.Context, filters studentpresence.CareTimeFilters, actor studentpresence.StatisticsActor, format string) (*studentpresence.CareTimeOverruns, error) {
	result, err := s.careTimeOverruns(ctx, filters)
	if err != nil {
		return nil, err
	}
	if err := s.recordAccess(ctx, careTimeAccessScope(filters), actor, "export", format, false); err != nil {
		return nil, err
	}
	return result, nil
}

func careTimeAccessScope(filters studentpresence.CareTimeFilters) studentpresence.StatisticsFilters {
	return studentpresence.StatisticsFilters{
		From: filters.From, To: filters.To, GroupIDs: filters.GroupIDs, SchoolClasses: filters.SchoolClasses,
		Sections: []studentpresence.StatisticsSection{studentpresence.StatisticsSectionCareTimes},
	}
}

func (s *service) careTimeOverruns(ctx context.Context, filters studentpresence.CareTimeFilters) (*studentpresence.CareTimeOverruns, error) {
	today := calendar.DateFromTime(s.cfg.Now())
	if err := s.validate(studentpresence.StatisticsFilters{From: filters.From, To: filters.To}, today); err != nil {
		return nil, err
	}
	students, err := s.cfg.Students.FindOverlappingWithGroups(ctx, filters.From, filters.To, today)
	if err != nil {
		return nil, fmt.Errorf("load students: %w", err)
	}
	students = enrolledStudents(filterStudentsByClass(filterStudentsByGroup(students, filters.GroupIDs), filters.SchoolClasses))
	compared, dataFrom, err := s.compareCareTimes(ctx, students, filters.From, filters.To, today)
	if err != nil {
		return nil, err
	}
	rows := make([]studentpresence.CareTimeOverrunRow, 0)
	for _, student := range students {
		summary := compared[student.ID].summary
		if summary.OverrunCount() == 0 {
			continue
		}
		rows = append(rows, studentpresence.CareTimeOverrunRow{
			StudentID: student.ID, FirstName: student.FirstName, LastName: student.LastName,
			SchoolClass: student.SchoolClass, GroupName: student.GroupName, Summary: summary,
		})
	}
	sort.SliceStable(rows, func(i, j int) bool {
		if a, b := rows[i].Summary.OverrunCount(), rows[j].Summary.OverrunCount(); a != b {
			return a > b
		}
		if c := compareStudentName(rows[i].LastName, rows[i].FirstName, rows[j].LastName, rows[j].FirstName); c != 0 {
			return c < 0
		}
		return rows[i].StudentID < rows[j].StudentID
	})
	return &studentpresence.CareTimeOverruns{
		From: filters.From, To: filters.To, DataFrom: dataFrom,
		ToleranceMinutes: studentpresence.CareTimeToleranceMinutes, Students: rows,
	}, nil
}

func enrolledStudents(students []*ports.StatisticsStudent) []*ports.StatisticsStudent {
	out := make([]*ports.StatisticsStudent, 0, len(students))
	for _, student := range students {
		if student != nil && student.EnrolledOn != nil {
			out = append(out, student)
		}
	}
	return out
}

// careTimeStudent finds the child in the caller's school; a child of another
// school or one not enrolled during the window is not found.
func (s *service) careTimeStudent(ctx context.Context, studentID int64, from, to, today calendar.Date) (*ports.StatisticsStudent, error) {
	students, err := s.cfg.Students.FindOverlappingWithGroups(ctx, from, to, today)
	if err != nil {
		return nil, fmt.Errorf("load students: %w", err)
	}
	for _, student := range enrolledStudents(students) {
		if student.ID == studentID {
			return student, nil
		}
	}
	return nil, studentpresence.ErrCareTimeStudentNotFound
}

type comparedStudent struct {
	days    []studentpresence.CareTimeDay
	summary studentpresence.CareTimeSummary
}

// careTimeInputs are the facts one comparison reads, each once for the
// whole population.
type careTimeInputs struct {
	careDays map[calendar.Date]bool
	planned  map[int64]map[calendar.Date]studentpresence.PlannedCareDay
	spans    map[dayKey]attendanceSpan
	status   map[dayKey]string
}

// attendanceSpan is the Ist of one child-day: the first check-in and the last
// check-out, nil while any attendance row of the day is still open.
type attendanceSpan struct {
	firstCheckIn time.Time
	lastCheckOut *time.Time
	open         bool
}

// compareCareTimes compares every child-day from the later of from and the
// retention cutoff; it returns that first compared day.
func (s *service) compareCareTimes(ctx context.Context, students []*ports.StatisticsStudent, from, to, today calendar.Date) (map[int64]comparedStudent, calendar.Date, error) {
	dataFrom, err := s.careTimeDataFrom(ctx, from, today)
	if err != nil {
		return nil, calendar.Date(""), err
	}
	result := make(map[int64]comparedStudent, len(students))
	if len(students) == 0 || to.Before(dataFrom) {
		return result, dataFrom, nil
	}
	inputs, err := s.loadCareTimeInputs(ctx, students, dataFrom, to)
	if err != nil {
		return nil, calendar.Date(""), err
	}
	for _, student := range students {
		result[student.ID] = compareStudentDays(student, inputs, dataFrom, to, today)
	}
	return result, dataFrom, nil
}

// compareStudentDays classifies the days of one child its enrollment covers.
func compareStudentDays(student *ports.StatisticsStudent, inputs careTimeInputs, from, to, today calendar.Date) comparedStudent {
	var compared comparedStudent
	for date := from; !date.After(to); date = date.AddDays(1) {
		if !student.EnrolledOn(date, today) {
			continue
		}
		key := dayKey{student.ID, date}
		var attended *attendanceSpan
		if span, ok := inputs.spans[key]; ok {
			attended = &span
		}
		day, listed := classifyCareTimeDay(date, inputs.planned[student.ID][date], attended, inputs.status[key], inputs.careDays[date])
		// Today is not over: a child who has not checked in yet may still come.
		if !listed || (date == today && day.Kind == studentpresence.CareTimeDayAbsent) {
			continue
		}
		compared.days = append(compared.days, day)
		addToSummary(&compared.summary, day)
	}
	return compared
}

// careTimeDataFrom is the first day both the attendance and the status days
// are still kept for, never before from.
func (s *service) careTimeDataFrom(ctx context.Context, from, today calendar.Date) (calendar.Date, error) {
	if s.cfg.PlannedCareDays == nil || s.cfg.AttendanceIntervals == nil || s.cfg.StatusDays == nil || s.cfg.Retention == nil {
		return calendar.Date(""), errCareTimesNotConfigured
	}
	planDays, actualDays, err := s.occupancyRetention(ctx)
	if err != nil {
		return calendar.Date(""), err
	}
	dataFrom := from
	for _, cutoff := range []calendar.Date{today.AddDays(-planDays), today.AddDays(-actualDays)} {
		if dataFrom.Before(cutoff) {
			dataFrom = cutoff
		}
	}
	return dataFrom, nil
}

func (s *service) loadCareTimeInputs(ctx context.Context, students []*ports.StatisticsStudent, from, to calendar.Date) (careTimeInputs, error) {
	ids := make([]int64, 0, len(students))
	for _, student := range students {
		ids = append(ids, student.ID)
	}
	var inputs careTimeInputs
	var err error
	if inputs.careDays, _, err = s.careDays(ctx, from, to); err != nil {
		return careTimeInputs{}, err
	}
	if inputs.planned, err = s.cfg.PlannedCareDays.PlannedCareDays(ctx, ids, from, to); err != nil {
		return careTimeInputs{}, fmt.Errorf("load planned care days: %w", err)
	}
	rows, err := s.loadAttendanceRows(ctx, ids, from, to, from)
	if err != nil {
		return careTimeInputs{}, err
	}
	inputs.spans = attendanceSpans(rows)
	statusDays, err := s.cfg.StatusDays.StatusDays(ctx, from, to)
	if err != nil {
		return careTimeInputs{}, fmt.Errorf("load status days: %w", err)
	}
	inputs.status = classifyStatusDays(statusDays)
	return inputs, nil
}

// attendanceSpans folds the attendance rows of a day into its first check-in
// and last check-out.
func attendanceSpans(rows []*ports.Attendance) map[dayKey]attendanceSpan {
	spans := make(map[dayKey]attendanceSpan, len(rows))
	for _, row := range rows {
		if row == nil {
			continue
		}
		key := dayKey{row.StudentID, row.Date}
		span, seen := spans[key]
		if !seen || row.CheckInTime.Before(span.firstCheckIn) {
			span.firstCheckIn = row.CheckInTime
		}
		switch {
		case row.CheckOutTime == nil:
			span.open = true
		case span.lastCheckOut == nil || row.CheckOutTime.After(*span.lastCheckOut):
			checkOut := *row.CheckOutTime
			span.lastCheckOut = &checkOut
		}
		spans[key] = span
	}
	return spans
}

// classifyCareTimeDay applies the definitions to one child-day. It reports
// false for a day that is neither booked nor attended.
func classifyCareTimeDay(date calendar.Date, planned studentpresence.PlannedCareDay, span *attendanceSpan, status string, careDay bool) (studentpresence.CareTimeDay, bool) {
	day := studentpresence.CareTimeDay{Date: date}
	present := span != nil
	if present {
		day.ActualStart = wallClock(span.firstCheckIn)
		if span.lastCheckOut != nil && !span.open {
			day.ActualEnd = wallClock(*span.lastCheckOut)
		}
		day.ActualMinutes = spanMinutes(day.ActualStart, day.ActualEnd)
	}
	switch {
	case planned.NoPlan, planned.Expected && planned.Arrival == nil && planned.Pickup == nil && present:
		// Nothing to compare against: no plan at all, or a booking that
		// names neither an arrival nor a pickup time.
		day.Kind = studentpresence.CareTimeDayNoPlan
		return day, present
	case planned.Expected:
		day.PlannedStart, day.PlannedEnd = planned.Arrival, planned.Pickup
		day.PlannedMinutes = spanMinutes(day.PlannedStart, day.PlannedEnd)
		if present {
			classifyAttendedBookedDay(&day)
		} else {
			classifyMissedBookedDay(&day, status, careDay)
		}
		return day, true
	case planned.Cancelled && !present:
		day.Kind = studentpresence.CareTimeDayCancelled
		return day, true
	case !present:
		return day, false
	}
	day.Kind = studentpresence.CareTimeDayUnbooked
	zero := 0
	day.PlannedMinutes = &zero
	day.DeviationMinutes = day.ActualMinutes
	return day, true
}

func classifyAttendedBookedDay(day *studentpresence.CareTimeDay) {
	if day.PlannedStart != nil {
		day.EarlyMinutes = max(0, clockMinutes(*day.PlannedStart)-clockMinutes(*day.ActualStart))
	}
	if day.PlannedEnd != nil && day.ActualEnd != nil {
		day.LateMinutes = max(0, clockMinutes(*day.ActualEnd)-clockMinutes(*day.PlannedEnd))
	}
	if day.PlannedMinutes != nil && day.ActualMinutes != nil {
		deviation := *day.ActualMinutes - *day.PlannedMinutes
		day.DeviationMinutes = &deviation
	}
	switch {
	case day.EarlyMinutes > studentpresence.CareTimeToleranceMinutes || day.LateMinutes > studentpresence.CareTimeToleranceMinutes:
		day.Kind = studentpresence.CareTimeDayOverrun
	case day.ActualEnd == nil:
		day.Kind = studentpresence.CareTimeDayOpen
	default:
		day.Kind = studentpresence.CareTimeDayWithinPlan
	}
}

// classifyMissedBookedDay names why a booked day was missed; only an
// unexplained absence on a care day counts as shorter than booked.
func classifyMissedBookedDay(day *studentpresence.CareTimeDay, status string, careDay bool) {
	switch {
	case status == statusSick:
		day.Kind = studentpresence.CareTimeDaySick
	case status == statusExcused:
		day.Kind = studentpresence.CareTimeDayExcused
	case !careDay:
		day.Kind = studentpresence.CareTimeDayClosed
	default:
		day.Kind = studentpresence.CareTimeDayAbsent
		zero := 0
		day.ActualMinutes = &zero
		if day.PlannedMinutes != nil {
			deviation := -*day.PlannedMinutes
			day.DeviationMinutes = &deviation
		}
	}
}

func addToSummary(summary *studentpresence.CareTimeSummary, day studentpresence.CareTimeDay) {
	switch day.Kind {
	case studentpresence.CareTimeDayOverrun:
		summary.OverrunDays++
		summary.ExtraMinutes += day.EarlyMinutes + day.LateMinutes
	case studentpresence.CareTimeDayUnbooked:
		summary.UnbookedDays++
		if day.ActualMinutes != nil {
			summary.ExtraMinutes += *day.ActualMinutes
		}
	case studentpresence.CareTimeDayAbsent:
		summary.AbsentDays++
	}
	if day.DeviationMinutes == nil {
		return
	}
	summary.PlannedMinutes += *day.PlannedMinutes
	summary.ActualMinutes += *day.ActualMinutes
	summary.DeviationMinutes += *day.DeviationMinutes
}

// wallClock is the Berlin wall clock of an attendance instant.
func wallClock(instant time.Time) *calendar.WallClock {
	clock := calendar.WallClockFromTime(instant.In(calendar.Berlin))
	return &clock
}

func clockMinutes(clock calendar.WallClock) int { return clock.Hour()*60 + clock.Minute() }

func spanMinutes(start, end *calendar.WallClock) *int {
	if start == nil || end == nil {
		return nil
	}
	length := max(0, clockMinutes(*end)-clockMinutes(*start))
	return &length
}
