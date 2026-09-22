// Package statistics computes the Statistik report (#2606): attendance
// and absence quotas per child, group and period, plus room utilization
// over the same window. Everything is derived from data other modules
// already record — active.attendance, active.student_status_days and
// active.visits — nothing here writes business rows.
//
// Definitions (binding, mirrored on the screen):
//
//   - care day        = Monday..Friday inside [from, to] minus public
//     holidays, tenant closing days and holiday calendar periods; counted per
//     child, so only the days the child is enrolled on (users.EnrolledOn)
//     land in that child's denominator
//   - present day     = care day with at least one attendance row
//   - absence day     = care day without attendance; classified by the
//     status day on that date: sick beats excused, class_trip counts as
//     excused, none = unexplained
//   - attendance rate = present days / care days
//   - group           = the child's current education group (no history)
package statistics

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence/internal/ports"
)

// MaxRangeDays caps a single report window (a school year plus a day).
const MaxRangeDays = 366

// The planned statuses Care Plan records on active.student_status_days.
const (
	statusSick      = "sick"
	statusExcused   = "excused"
	statusClassTrip = "class_trip"
)

// Config wires the service dependencies.
type Config struct {
	Statistics      ports.RoomUtilizations
	Attendance      ports.AttendanceDays
	StatusDays      ports.StatusDays
	Courses         ports.CourseStatistics
	Holidays        ports.HolidayDates
	ClosingDays     ports.ClosingDayDates
	Periods         ports.HolidayPeriods
	Students        ports.StatisticsStudents
	Rooms           ports.StatisticsRooms
	AccessLog       ports.StatisticsAccessLog
	Retention       ports.StatisticsRetention
	PrivacyConsents ports.RetentionSettings
	Logger          *slog.Logger
	// Now is injectable for tests; nil means time.Now.
	Now func() time.Time
}

type service struct {
	cfg Config
}

// NewService creates the statistics service.
func NewService(cfg Config) studentpresence.StatisticsReports {
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	return &service{cfg: cfg}
}

// viewDedupWindow collapses repeated report views of the same window into
// one audit row, mirroring the polling readers elsewhere.
const viewDedupWindow = 15 * time.Minute

func (s *service) Report(ctx context.Context, filters studentpresence.StatisticsFilters, actor studentpresence.StatisticsActor) (*studentpresence.StatisticsReport, error) {
	report, err := s.compute(ctx, filters)
	if err != nil {
		return nil, err
	}
	if err := s.recordAccess(ctx, filters, actor, "view", "", true); err != nil {
		return nil, err
	}
	return report, nil
}

func (s *service) ReportForExport(ctx context.Context, filters studentpresence.StatisticsFilters, actor studentpresence.StatisticsActor, format string) (*studentpresence.StatisticsReport, error) {
	report, err := s.compute(ctx, filters)
	if err != nil {
		return nil, err
	}
	if err := s.recordAccess(ctx, filters, actor, "export", format, false); err != nil {
		return nil, err
	}
	return report, nil
}

func (s *service) today() timezone.Date {
	return timezone.DateFromTime(s.cfg.Now())
}

func (s *service) validate(filters studentpresence.StatisticsFilters, today timezone.Date) error {
	if filters.From.IsZero() || filters.To.IsZero() {
		return fmt.Errorf("%w: from and to are required", studentpresence.ErrInvalidStatisticsRange)
	}
	if filters.To.Before(filters.From) {
		return fmt.Errorf("%w: from must not be after to", studentpresence.ErrInvalidStatisticsRange)
	}
	if filters.To.After(today) {
		return fmt.Errorf("%w: to must not be in the future", studentpresence.ErrInvalidStatisticsRange)
	}
	if filters.From.DaysUntil(filters.To)+1 > MaxRangeDays {
		return fmt.Errorf("%w: window exceeds %d days", studentpresence.ErrInvalidStatisticsRange, MaxRangeDays)
	}
	return nil
}

func (s *service) compute(ctx context.Context, filters studentpresence.StatisticsFilters) (*studentpresence.StatisticsReport, error) {
	// One clock read for the whole report: enrollment eligibility, the room
	// clamp and the retention cutoff must all agree on "today" even when the
	// request spans Berlin midnight (see users.EnrolledOn).
	today := s.today()
	if err := s.validate(filters, today); err != nil {
		return nil, err
	}

	// The child population is shared by all three sections — every one of
	// them reports children, so it is loaded unconditionally. The care-day
	// and attendance reads behind it are the attendance section's alone and
	// stay unread when it was not asked for.
	students, err := s.cfg.Students.FindOverlappingWithGroups(ctx, filters.From, filters.To, today)
	if err != nil {
		return nil, fmt.Errorf("load students: %w", err)
	}
	students = filterStudentsByGroup(students, filters.GroupIDs)

	var inputs attendanceInputs
	if filters.Wants(studentpresence.StatisticsSectionAttendance) {
		if inputs, err = s.loadAttendanceInputs(ctx, filters.From, filters.To); err != nil {
			return nil, err
		}
	}

	report := &studentpresence.StatisticsReport{
		From: filters.From,
		To:   filters.To,
	}
	// Without the attendance section the rows carry identity only (zero care
	// days, zero counters) and serve as the shared population; they are not
	// put on the wire, where they would read as "every child was absent".
	studentRows := buildStudentRows(students, inputs.careDays, inputs.attendance, inputs.statusDays, today)
	if filters.Wants(studentpresence.StatisticsSectionAttendance) {
		report.CareDays = len(inputs.careDays)
		report.ExcludedDays = inputs.excluded
		report.Students = studentRows
		report.Groups = buildGroupRows(studentRows)
		report.Totals = buildTotals(studentRows)
	}
	if filters.Wants(studentpresence.StatisticsSectionRooms) {
		if err := s.fillRoomSection(ctx, report, filters, today, students, studentRows); err != nil {
			return nil, err
		}
	}
	if filters.Wants(studentpresence.StatisticsSectionCourses) {
		if err := s.fillCourseSection(ctx, report, filters, today, studentRows); err != nil {
			return nil, err
		}
	}
	return report, nil
}

// attendanceInputs are the reads only the attendance section needs.
type attendanceInputs struct {
	careDays   map[timezone.Date]bool
	excluded   studentpresence.StatisticsExcludedDays
	attendance []ports.AttendanceDay
	statusDays []ports.StatusDay
}

func (s *service) loadAttendanceInputs(ctx context.Context, from, to timezone.Date) (attendanceInputs, error) {
	var (
		inputs attendanceInputs
		err    error
	)
	inputs.careDays, inputs.excluded, err = s.careDays(ctx, from, to)
	if err != nil {
		return attendanceInputs{}, err
	}
	inputs.attendance, err = s.cfg.Attendance.ListAttendanceDays(ctx, from, to)
	if err != nil {
		return attendanceInputs{}, fmt.Errorf("load attendance days: %w", err)
	}
	inputs.statusDays, err = s.cfg.StatusDays.StatusDays(ctx, from, to)
	if err != nil {
		return attendanceInputs{}, fmt.Errorf("load status days: %w", err)
	}
	return inputs, nil
}

func (s *service) fillRoomSection(ctx context.Context, report *studentpresence.StatisticsReport, filters studentpresence.StatisticsFilters, today timezone.Date, students []*ports.StatisticsStudent, studentRows []studentpresence.StatisticsStudentRow) error {
	rooms, err := s.roomRows(ctx, filters, today, students)
	if err != nil {
		return err
	}
	report.Rooms = rooms
	report.RoomDataDays, err = s.roomRetentionDays(ctx, studentRows)
	if err != nil {
		return err
	}
	report.RoomDataFrom = today.AddDays(-report.RoomDataDays)
	return nil
}

func (s *service) fillCourseSection(ctx context.Context, report *studentpresence.StatisticsReport, filters studentpresence.StatisticsFilters, today timezone.Date, studentRows []studentpresence.StatisticsStudentRow) error {
	var err error
	report.CourseDataDays, err = s.courseRetentionDays(ctx)
	if err != nil {
		return err
	}
	report.CourseDataFrom = today.AddDays(-report.CourseDataDays)
	// Read no further back than the cutoff the screen names, whether or
	// not the cleanup job has caught up with it yet.
	courseFrom := filters.From
	if courseFrom.Before(report.CourseDataFrom) {
		courseFrom = report.CourseDataFrom
	}
	courses, courseStudents, courseTotals, err := s.courseSection(ctx, filters, courseFrom, filters.To, today, studentRows)
	if err != nil {
		return err
	}
	report.Courses = courses
	report.CourseStudents = courseStudents
	report.CourseTotals = courseTotals
	return nil
}

// careDays returns the ordered set of care days in the window and the
// exclusion breakdown.
func (s *service) careDays(ctx context.Context, from, to timezone.Date) (map[timezone.Date]bool, studentpresence.StatisticsExcludedDays, error) {
	var excluded studentpresence.StatisticsExcludedDays
	holidays, err := s.holidayDates(ctx, from, to)
	if err != nil {
		return nil, excluded, err
	}
	closing, err := s.closingDayDates(ctx, from, to)
	if err != nil {
		return nil, excluded, err
	}
	vacation, err := s.holidayPeriodDates(ctx, from, to)
	if err != nil {
		return nil, excluded, err
	}

	care := map[timezone.Date]bool{}
	for d := from; !d.After(to); d = d.AddDays(1) {
		if wd := d.Weekday(); wd == time.Saturday || wd == time.Sunday {
			continue
		}
		if countExclusions(&excluded, holidays[d], closing[d], vacation[d]) {
			continue
		}
		care[d] = true
	}
	return care, excluded, nil
}

// countExclusions adds a weekday to every exclusion bucket it falls into and
// reports whether it is excluded at all; the total counts the union.
func countExclusions(excluded *studentpresence.StatisticsExcludedDays, holiday, closing, vacation bool) bool {
	if holiday {
		excluded.PublicHolidays++
	}
	if closing {
		excluded.ClosingDays++
	}
	if vacation {
		excluded.HolidayPeriods++
	}
	off := holiday || closing || vacation
	if off {
		excluded.Total++
	}
	return off
}

func (s *service) holidayDates(ctx context.Context, from, to timezone.Date) (map[timezone.Date]bool, error) {
	if s.cfg.Holidays == nil {
		return map[timezone.Date]bool{}, nil
	}
	set, err := s.cfg.Holidays.HolidayDates(ctx, from, to)
	if err != nil {
		return nil, fmt.Errorf("load public holidays: %w", err)
	}
	return set, nil
}

func (s *service) closingDayDates(ctx context.Context, from, to timezone.Date) (map[timezone.Date]bool, error) {
	if s.cfg.ClosingDays == nil {
		return map[timezone.Date]bool{}, nil
	}
	set, err := s.cfg.ClosingDays.ClosingDayDates(ctx, from, to)
	if err != nil {
		return nil, fmt.Errorf("load closing days: %w", err)
	}
	return set, nil
}

func (s *service) holidayPeriodDates(ctx context.Context, from, to timezone.Date) (map[timezone.Date]bool, error) {
	vacation := map[timezone.Date]bool{}
	if s.cfg.Periods == nil {
		return vacation, nil
	}
	periods, err := s.cfg.Periods.StatisticsHolidayPeriods(ctx, from, to)
	if err != nil {
		return nil, fmt.Errorf("load holiday periods: %w", err)
	}
	for _, p := range periods {
		// Clamp to the report window: a period may span years, and only
		// its overlap with [from, to] can ever be a care day.
		start, end := p.StartDate, p.EndDate
		if start.Before(from) {
			start = from
		}
		if end.After(to) {
			end = to
		}
		for d := start; !d.After(end); d = d.AddDays(1) {
			vacation[d] = true
		}
	}
	return vacation, nil
}

func filterStudentsByGroup(students []*ports.StatisticsStudent, groupIDs []int64) []*ports.StatisticsStudent {
	if len(groupIDs) == 0 {
		return students
	}
	wanted := make(map[int64]bool, len(groupIDs))
	for _, id := range groupIDs {
		wanted[id] = true
	}
	out := make([]*ports.StatisticsStudent, 0, len(students))
	for _, st := range students {
		if st == nil || st.EnrolledOn == nil {
			continue
		}
		if (st.GroupID == nil && wanted[0]) || (st.GroupID != nil && wanted[*st.GroupID]) {
			out = append(out, st)
		}
	}
	return out
}

type dayKey struct {
	studentID int64
	date      timezone.Date
}

func buildStudentRows(students []*ports.StatisticsStudent, careDays map[timezone.Date]bool, attendance []ports.AttendanceDay, statusDays []ports.StatusDay, today timezone.Date) []studentpresence.StatisticsStudentRow {
	present := make(map[dayKey]bool, len(attendance))
	for _, row := range attendance {
		present[dayKey{row.StudentID, row.Date}] = true
	}
	status := classifyStatusDays(statusDays)

	rows := make([]studentpresence.StatisticsStudentRow, 0, len(students))
	for _, st := range students {
		if st == nil || st.EnrolledOn == nil {
			continue
		}
		rows = append(rows, buildStudentRow(st, careDays, present, status, today))
	}
	sort.SliceStable(rows, func(i, j int) bool {
		if c := compareStudentName(rows[i].LastName, rows[i].FirstName, rows[j].LastName, rows[j].FirstName); c != 0 {
			return c < 0
		}
		return rows[i].StudentID < rows[j].StudentID
	})
	return rows
}

// classifyStatusDays keeps one status per child and day: sick beats excused;
// class_trip counts as excused.
func classifyStatusDays(statusDays []ports.StatusDay) map[dayKey]string {
	status := make(map[dayKey]string, len(statusDays))
	for _, row := range statusDays {
		key := dayKey{row.StudentID, row.Date}
		switch row.Status {
		case statusSick:
			status[key] = statusSick
		case statusExcused, statusClassTrip:
			if status[key] != statusSick {
				status[key] = statusExcused
			}
		}
	}
	return status
}

// buildStudentRow counts one child's care days inside their enrollment.
func buildStudentRow(st *ports.StatisticsStudent, careDays map[timezone.Date]bool, present map[dayKey]bool, status map[dayKey]string, today timezone.Date) studentpresence.StatisticsStudentRow {
	row := studentpresence.StatisticsStudentRow{
		StudentID:   st.ID,
		SchoolClass: st.SchoolClass,
		GroupID:     st.GroupID,
		GroupName:   st.GroupName,
	}
	row.FirstName = st.FirstName
	row.LastName = st.LastName
	for day := range careDays {
		if !st.EnrolledOn(day, today) {
			continue
		}
		row.CareDays++
		key := dayKey{st.ID, day}
		switch {
		case present[key]:
			row.PresentDays++
		case status[key] == statusSick:
			row.SickDays++
		case status[key] == statusExcused:
			row.ExcusedDays++
		default:
			row.UnexplainedDays++
		}
	}
	row.AttendanceRate = rate(row.PresentDays, row.CareDays)
	return row
}

// NoGroupName labels the pseudo group of children without a group.
const NoGroupName = "Ohne Gruppe"

func buildGroupRows(students []studentpresence.StatisticsStudentRow) []studentpresence.StatisticsGroupRow {
	byGroup := map[int64]*studentpresence.StatisticsGroupRow{}
	for _, st := range students {
		id := int64(0)
		name := NoGroupName
		if st.GroupID != nil {
			id = *st.GroupID
			name = st.GroupName
		}
		row, ok := byGroup[id]
		if !ok {
			row = &studentpresence.StatisticsGroupRow{GroupID: id, Name: name}
			byGroup[id] = row
		}
		row.StudentCount++
		row.PresentDays += st.PresentDays
		row.SickDays += st.SickDays
		row.ExcusedDays += st.ExcusedDays
		row.UnexplainedDays += st.UnexplainedDays
	}
	rows := make([]studentpresence.StatisticsGroupRow, 0, len(byGroup))
	for _, row := range byGroup {
		row.AttendanceRate = rate(row.PresentDays, groupCareDays(students, row.GroupID))
		rows = append(rows, *row)
	}
	sort.SliceStable(rows, func(i, j int) bool {
		// "Ohne Gruppe" last, otherwise by name.
		if (rows[i].GroupID == 0) != (rows[j].GroupID == 0) {
			return rows[j].GroupID == 0
		}
		return sortKey(rows[i].Name) < sortKey(rows[j].Name)
	})
	return rows
}

// groupCareDays sums the care days of the group's children; group 0 is the
// pseudo group of children without a group.
func groupCareDays(students []studentpresence.StatisticsStudentRow, groupID int64) int {
	careDays := 0
	for _, student := range students {
		if (student.GroupID == nil && groupID == 0) || (student.GroupID != nil && *student.GroupID == groupID) {
			careDays += student.CareDays
		}
	}
	return careDays
}

// sortKey folds case and German umlauts so "Bärengruppe" sorts before
// "Blumengruppe" (byte order would put every umlaut after z).
func sortKey(s string) string {
	return umlautFolder.Replace(strings.ToLower(s))
}

var umlautFolder = strings.NewReplacer("ä", "a", "ö", "o", "ü", "u", "ß", "ss")

// totalsRowName labels the aggregate row of every section.
const totalsRowName = "Gesamt"

func buildTotals(students []studentpresence.StatisticsStudentRow) studentpresence.StatisticsGroupRow {
	total := studentpresence.StatisticsGroupRow{Name: totalsRowName}
	careDays := 0
	for _, st := range students {
		total.StudentCount++
		total.PresentDays += st.PresentDays
		total.SickDays += st.SickDays
		total.ExcusedDays += st.ExcusedDays
		total.UnexplainedDays += st.UnexplainedDays
		careDays += st.CareDays
	}
	total.AttendanceRate = rate(total.PresentDays, careDays)
	return total
}

// rate returns numerator/denominator in percent rounded to one decimal.
func rate(numerator, denominator int) *float64 {
	if denominator <= 0 {
		return nil
	}
	v := float64(numerator) * 1000 / float64(denominator)
	v = float64(int64(v+0.5)) / 10
	return &v
}

func (s *service) roomRows(ctx context.Context, filters studentpresence.StatisticsFilters, today timezone.Date, students []*ports.StatisticsStudent) ([]studentpresence.StatisticsRoomRow, error) {
	agg, err := s.cfg.Statistics.RoomUtilization(ctx, visitWindows(students, filters.From, filters.To, today))
	if err != nil {
		return nil, fmt.Errorf("load room utilization: %w", err)
	}
	rooms, err := s.cfg.Rooms.StatisticsRooms(ctx)
	if err != nil {
		return nil, fmt.Errorf("load rooms: %w", err)
	}
	byID := make(map[int64]ports.RoomUtilization, len(agg))
	for _, row := range agg {
		byID[row.RoomID] = row
	}
	out := make([]studentpresence.StatisticsRoomRow, 0, len(rooms))
	for _, room := range rooms {
		row := studentpresence.StatisticsRoomRow{RoomID: room.ID, Name: room.Name, Capacity: room.Capacity}
		if a, ok := byID[room.ID]; ok {
			row.DaysUsed = a.DaysUsed
			row.DistinctStudents = a.DistinctStudents
			row.StudentMinutes = a.StudentMinutes
			row.PeakOccupancy = a.PeakOccupancy
		}
		if room.Capacity != nil && *room.Capacity > 0 {
			row.PeakUtilizationPercent = rate(row.PeakOccupancy, *room.Capacity)
		}
		out = append(out, row)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].DaysUsed != out[j].DaysUsed {
			return out[i].DaysUsed > out[j].DaysUsed
		}
		return sortKey(out[i].Name) < sortKey(out[j].Name)
	})
	return out, nil
}

// roomRetentionDays returns how far back the room section of THIS report can
// reach: the longest retention window among the children it covers.
//
// The scope matters twice. Per child, the room aggregate clamps to
// MIN(data_retention_days) over the accepted consents, so a child with a
// 7-day and a 30-day consent contributes 7 — taking the maximum over raw
// consent rows would advertise 30. Across children, a group filter narrows the
// population, and the tenant-wide maximum would then promise data for dates
// the filtered report can never populate.
//
// Children without an accepted consent contribute no room data at all (the
// aggregate joins the retention set), so they do not raise the cutoff. When
// none of the covered children has a consent, the configured default is the
// only honest statement about how long visits are kept.
func (s *service) roomRetentionDays(ctx context.Context, students []studentpresence.StatisticsStudentRow) (int, error) {
	if s.cfg.Retention == nil {
		return 0, errors.New("statistics retention policy is required")
	}
	defaultDays, err := s.cfg.Retention.RoomRetentionDays(ctx)
	if err != nil {
		return 0, fmt.Errorf("load room retention: %w", err)
	}
	if s.cfg.PrivacyConsents == nil {
		return defaultDays, nil
	}
	settings, err := s.cfg.PrivacyConsents.ListAcceptedRetentionSettings(ctx)
	if err != nil {
		return 0, fmt.Errorf("load visit retention settings: %w", err)
	}
	shortestPerStudent := make(map[int64]int, len(settings))
	for _, setting := range settings {
		if current, ok := shortestPerStudent[setting.StudentID]; !ok || setting.DataRetentionDays < current {
			shortestPerStudent[setting.StudentID] = setting.DataRetentionDays
		}
	}
	retentionDays := 0
	covered := false
	for _, student := range students {
		days, ok := shortestPerStudent[student.StudentID]
		if !ok {
			continue
		}
		covered = true
		if days > retentionDays {
			retentionDays = days
		}
	}
	if !covered {
		return defaultDays, nil
	}
	return retentionDays, nil
}

func (s *service) recordAccess(ctx context.Context, filters studentpresence.StatisticsFilters, actor studentpresence.StatisticsActor, action, format string, dedup bool) error {
	if s.cfg.AccessLog == nil {
		return fmt.Errorf("%w: access log not configured", studentpresence.ErrStatisticsAuditFailed)
	}
	role := actor.Role
	if role == "" {
		role = "unknown"
	}
	meta := map[string]string{
		"action": action,
		"from":   filters.From.String(),
		"to":     filters.To.String(),
	}
	if len(filters.Sections) > 0 {
		sections := make([]string, 0, len(filters.Sections))
		for _, section := range filters.Sections {
			sections = append(sections, string(section))
		}
		sort.Strings(sections)
		meta["sections"] = strings.Join(sections, ",")
	}
	if len(filters.GroupIDs) > 0 {
		groupIDs := append([]int64(nil), filters.GroupIDs...)
		sort.Slice(groupIDs, func(i, j int) bool { return groupIDs[i] < groupIDs[j] })
		meta["group_ids"] = strings.Trim(strings.Join(strings.Fields(fmt.Sprint(groupIDs)), ","), "[]")
	}
	now := s.cfg.Now()
	if dedup {
		exists, err := s.cfg.AccessLog.SeenStatisticsAccessSince(ctx, actor.AccountID, meta, now.Add(-viewDedupWindow))
		if err != nil {
			return fmt.Errorf("%w: %v", studentpresence.ErrStatisticsAuditFailed, err)
		}
		if exists {
			return nil
		}
	}
	entry := ports.StatisticsAccessEvent{
		ActorAccountID: actor.AccountID,
		ActorRole:      role,
		RangeStart:     filters.From.BerlinMidnight(),
		RangeEnd:       filters.To.EndOfDay(),
		AccessedAt:     now,
	}
	entry.Metadata = meta
	if format != "" {
		entry.Metadata["format"] = format
	}

	if err := s.cfg.AccessLog.RecordStatisticsAccess(ctx, entry); err != nil {
		s.cfg.Logger.Error("statistics audit write failed",
			slog.String("action", action),
			slog.String("error", err.Error()),
		)
		return fmt.Errorf("%w: %v", studentpresence.ErrStatisticsAuditFailed, err)
	}
	return nil
}

// visitWindows uses the owner's eligibility answer for every report date.
// Adjacent eligible days form one interval so visits spanning midnight are
// counted once and Berlin daylight-saving transitions preserve elapsed time.
func visitWindows(students []*ports.StatisticsStudent, from, to, today timezone.Date) []ports.StudentVisitWindow {
	windows := make([]ports.StudentVisitWindow, 0, len(students))
	for _, student := range students {
		if student == nil || student.EnrolledOn == nil {
			continue
		}
		windows = appendStudentVisitWindows(windows, student, from, to, today)
	}
	return windows
}

// appendStudentVisitWindows appends one child's eligible intervals in order.
func appendStudentVisitWindows(windows []ports.StudentVisitWindow, student *ports.StatisticsStudent, from, to, today timezone.Date) []ports.StudentVisitWindow {
	var current *ports.StudentVisitWindow
	for day := from; !day.After(to); day = day.AddDays(1) {
		if !student.EnrolledOn(day, today) {
			if current != nil {
				windows = append(windows, *current)
				current = nil
			}
			continue
		}
		if current == nil {
			current = &ports.StudentVisitWindow{StudentID: student.ID, StartAt: day.BerlinMidnight()}
		}
		current.EndAt = day.AddDays(1).BerlinMidnight()
	}
	if current != nil {
		windows = append(windows, *current)
	}
	return windows
}
