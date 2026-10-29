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
// care plan (Soll, through ports.CareTimeTargets) and the attendance (Ist).

var errCareTimesNotConfigured = errors.New("statistics: care time sources are not configured")

func (s *service) StudentCareTimes(ctx context.Context, studentID int64, from, to calendar.Date) (*studentpresence.StudentCareTimes, error) {
	today := s.today()
	if err := s.validate(studentpresence.StatisticsFilters{From: from, To: to}, today); err != nil {
		return nil, err
	}
	student, err := s.careTimeStudent(ctx, studentID, from, to, today)
	if err != nil {
		return nil, err
	}
	compared, err := s.compareCareTimes(ctx, []*ports.StatisticsStudent{student}, from, to, today)
	if err != nil {
		return nil, err
	}
	result := compared[studentID]
	return &studentpresence.StudentCareTimes{
		StudentID: studentID, From: from, To: to,
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
		From: filters.From, To: filters.To, GroupIDs: filters.GroupIDs,
		Sections: []studentpresence.StatisticsSection{studentpresence.StatisticsSectionCareTimes},
	}
}

func (s *service) careTimeOverruns(ctx context.Context, filters studentpresence.CareTimeFilters) (*studentpresence.CareTimeOverruns, error) {
	today := s.today()
	if err := s.validate(studentpresence.StatisticsFilters{From: filters.From, To: filters.To}, today); err != nil {
		return nil, err
	}
	students, err := s.cfg.Students.FindOverlappingWithGroups(ctx, filters.From, filters.To, today)
	if err != nil {
		return nil, fmt.Errorf("load students: %w", err)
	}
	students = enrolledStudents(filterStudentsByGroup(students, filters.GroupIDs))
	compared, err := s.compareCareTimes(ctx, students, filters.From, filters.To, today)
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
		From: filters.From, To: filters.To,
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
	targets  map[int64]map[calendar.Date]studentpresence.CareTimeTarget
	spans    map[dayKey]ports.AttendanceSpan
	status   map[dayKey]string
}

func (s *service) compareCareTimes(ctx context.Context, students []*ports.StatisticsStudent, from, to, today calendar.Date) (map[int64]comparedStudent, error) {
	result := make(map[int64]comparedStudent, len(students))
	if len(students) == 0 {
		return result, nil
	}
	inputs, err := s.loadCareTimeInputs(ctx, students, from, to)
	if err != nil {
		return nil, err
	}
	for _, student := range students {
		var compared comparedStudent
		for date := from; !date.After(to); date = date.AddDays(1) {
			if !student.EnrolledOn(date, today) {
				continue
			}
			key := dayKey{student.ID, date}
			var attended *ports.AttendanceSpan
			if span, ok := inputs.spans[key]; ok {
				attended = &span
			}
			day, listed := classifyCareTimeDay(date, inputs.targets[student.ID][date], attended, inputs.status[key], inputs.careDays[date])
			if !listed {
				continue
			}
			compared.days = append(compared.days, day)
			addToSummary(&compared.summary, day)
		}
		result[student.ID] = compared
	}
	return result, nil
}

func (s *service) loadCareTimeInputs(ctx context.Context, students []*ports.StatisticsStudent, from, to calendar.Date) (careTimeInputs, error) {
	if s.cfg.Attendance == nil || s.cfg.StatusDays == nil {
		return careTimeInputs{}, errCareTimesNotConfigured
	}
	ids := make([]int64, 0, len(students))
	for _, student := range students {
		ids = append(ids, student.ID)
	}
	var inputs careTimeInputs
	var err error
	if inputs.careDays, _, err = s.careDays(ctx, from, to); err != nil {
		return careTimeInputs{}, err
	}
	if inputs.targets, err = s.cfg.StatusDays.CareTimeTargets(ctx, ids, from, to); err != nil {
		return careTimeInputs{}, fmt.Errorf("load care times: %w", err)
	}
	spans, err := s.cfg.Attendance.ListAttendanceSpans(ctx, from, to, ids)
	if err != nil {
		return careTimeInputs{}, fmt.Errorf("load attendance spans: %w", err)
	}
	inputs.spans = make(map[dayKey]ports.AttendanceSpan, len(spans))
	for _, span := range spans {
		inputs.spans[dayKey{span.StudentID, span.Date}] = span
	}
	statusDays, err := s.cfg.StatusDays.StatusDays(ctx, from, to)
	if err != nil {
		return careTimeInputs{}, fmt.Errorf("load status days: %w", err)
	}
	inputs.status = classifyStatusDays(statusDays)
	return inputs, nil
}

// classifyCareTimeDay applies the definitions to one child-day. It reports
// false for a day that is neither booked nor attended.
func classifyCareTimeDay(date calendar.Date, target studentpresence.CareTimeTarget, span *ports.AttendanceSpan, status string, careDay bool) (studentpresence.CareTimeDay, bool) {
	day := studentpresence.CareTimeDay{Date: date}
	present := span != nil
	if present {
		day.ActualStart = wallClock(span.FirstCheckIn)
		if span.LastCheckOut != nil {
			day.ActualEnd = wallClock(*span.LastCheckOut)
		}
		day.ActualMinutes = spanMinutes(day.ActualStart, day.ActualEnd)
	}
	switch target.Status {
	case studentpresence.CareTimeScheduled:
		day.PlannedStart, day.PlannedEnd = target.Arrival, target.Pickup
		day.PlannedMinutes = spanMinutes(day.PlannedStart, day.PlannedEnd)
		if present {
			classifyAttendedBookedDay(&day)
			return day, true
		}
		classifyMissedBookedDay(&day, status, careDay)
		return day, true
	case studentpresence.CareTimeUnknown:
		day.Kind = studentpresence.CareTimeDayNoPlan
		return day, present
	case studentpresence.CareTimeCancelled:
		if !present {
			day.Kind = studentpresence.CareTimeDayCancelled
			return day, true
		}
	}
	if !present {
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
