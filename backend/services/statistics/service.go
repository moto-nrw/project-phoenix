// Package statistics computes the Statistik report (#2606): attendance
// and absence quotas per child, group and period, plus room utilization
// over the same window. Everything is derived from data other modules
// already record — active.attendance, active.student_status_days and
// active.visits — nothing here writes business rows.
//
// Definitions (binding, mirrored on the screen):
//
//   - care day        = Monday..Friday inside [from, to] minus public
//     holidays, tenant closing days and holiday calendar periods
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
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	facilityModels "github.com/moto-nrw/project-phoenix/models/facilities"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	configService "github.com/moto-nrw/project-phoenix/services/config"
)

// MaxRangeDays caps a single report window (a school year plus a day).
const MaxRangeDays = 366

var (
	// ErrInvalidRange is returned for from > to, a window ending in the
	// future, or a window longer than MaxRangeDays.
	ErrInvalidRange = errors.New("invalid statistics range")
	// ErrAuditFailed wraps a failed data-access log write; the report is
	// withheld rather than served unlogged.
	ErrAuditFailed = errors.New("statistics audit write failed")
)

// Filters selects the report window and an optional group restriction.
type Filters struct {
	From     timezone.Date
	To       timezone.Date
	GroupIDs []int64
}

// Actor identifies who requests the report for the GDPR access log.
type Actor struct {
	AccountID int64
	Role      string
}

// StudentRow is one child in the report.
type StudentRow struct {
	StudentID       int64
	FirstName       string
	LastName        string
	SchoolClass     string
	GroupID         *int64
	GroupName       string
	PresentDays     int
	SickDays        int
	ExcusedDays     int
	UnexplainedDays int
	CareDays        int
	// AttendanceRate is present/care in percent; nil when there are no
	// care days in the window.
	AttendanceRate *float64
}

// GroupRow aggregates the children of one education group. GroupID 0 is
// the pseudo group for children without a group.
type GroupRow struct {
	GroupID         int64
	Name            string
	StudentCount    int
	PresentDays     int
	SickDays        int
	ExcusedDays     int
	UnexplainedDays int
	AttendanceRate  *float64
}

// RoomRow is the utilization of one room over the window.
type RoomRow struct {
	RoomID           int64
	Name             string
	Capacity         *int
	DaysUsed         int
	DistinctStudents int
	StudentMinutes   int
	PeakOccupancy    int
	// PeakUtilizationPercent is peak / capacity; nil without a capacity.
	PeakUtilizationPercent *float64
}

// ExcludedDays explains why weekdays were removed from the care-day count.
// The buckets may overlap (a closing day inside the holidays); Total is
// the size of the union.
type ExcludedDays struct {
	Total          int
	PublicHolidays int
	ClosingDays    int
	HolidayPeriods int
}

// Report is the full statistics result.
type Report struct {
	From         timezone.Date
	To           timezone.Date
	CareDays     int
	ExcludedDays ExcludedDays
	Students     []StudentRow
	Groups       []GroupRow
	Rooms        []RoomRow
	// RoomDataDays is the longest active visit-retention window in days.
	// Individual children may have shorter windows.
	RoomDataDays int
	// RoomDataFrom is the earliest date room data can still exist for.
	RoomDataFrom timezone.Date
	Totals       GroupRow
}

// Service is the statistics use case.
type Service interface {
	// Report computes the statistics for the window. It records one
	// data-access log row per actor and window (deduplicated for 15 min).
	Report(ctx context.Context, filters Filters, actor Actor) (*Report, error)
	// ReportForExport computes the report and records an export access
	// row every time (no deduplication — every download is evidence).
	ReportForExport(ctx context.Context, filters Filters, actor Actor, format string) (*Report, error)
}

type holidayDates interface {
	HolidayDates(ctx context.Context, from, to timezone.Date) (map[timezone.Date]bool, error)
}

type closingDayDates interface {
	ClosingDayDates(ctx context.Context, from, to timezone.Date) (map[timezone.Date]bool, error)
}

type calendarPeriods interface {
	FindActiveOverlappingByType(ctx context.Context, periodType string, start, end timezone.Date, excludeID int64) ([]*scheduleModels.CalendarPeriod, error)
}

type studentReader interface {
	FindOverlappingWithGroups(ctx context.Context, from, to timezone.Date) ([]*userModels.StudentWithGroupInfo, error)
}

type roomReader interface {
	List(ctx context.Context, filters map[string]any) ([]*facilityModels.Room, error)
}

type accessLog interface {
	Create(ctx context.Context, entry *auditModels.DataAccessLog) error
	ExistsSince(ctx context.Context, actorAccountID int64, resourceType string, metadata map[string]string, since time.Time) (bool, error)
}

type intResolver interface {
	HasTenantOverride(ctx context.Context, key string) (bool, error)
	ResolveInt(ctx context.Context, key string) (int, error)
}

type retentionSettingsReader interface {
	ListAcceptedRetentionSettings(ctx context.Context) ([]userModels.StudentRetentionSetting, error)
}

// Config wires the service dependencies.
type Config struct {
	Statistics      activeModels.StatisticsRepository
	Holidays        holidayDates
	ClosingDays     closingDayDates
	Periods         calendarPeriods
	Students        studentReader
	Rooms           roomReader
	AccessLog       accessLog
	Settings        intResolver
	PrivacyConsents retentionSettingsReader
	Logger          *slog.Logger
	// Now is injectable for tests; nil means time.Now.
	Now func() time.Time
}

type service struct {
	cfg Config
}

// NewService creates the statistics service.
func NewService(cfg Config) Service {
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

func (s *service) Report(ctx context.Context, filters Filters, actor Actor) (*Report, error) {
	report, err := s.compute(ctx, filters)
	if err != nil {
		return nil, err
	}
	if err := s.recordAccess(ctx, filters, actor, "view", "", true); err != nil {
		return nil, err
	}
	return report, nil
}

func (s *service) ReportForExport(ctx context.Context, filters Filters, actor Actor, format string) (*Report, error) {
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

func (s *service) validate(filters Filters) error {
	if filters.From.IsZero() || filters.To.IsZero() {
		return fmt.Errorf("%w: from and to are required", ErrInvalidRange)
	}
	if filters.To.Before(filters.From) {
		return fmt.Errorf("%w: from must not be after to", ErrInvalidRange)
	}
	if filters.To.After(s.today()) {
		return fmt.Errorf("%w: to must not be in the future", ErrInvalidRange)
	}
	if filters.From.DaysUntil(filters.To)+1 > MaxRangeDays {
		return fmt.Errorf("%w: window exceeds %d days", ErrInvalidRange, MaxRangeDays)
	}
	return nil
}

func (s *service) compute(ctx context.Context, filters Filters) (*Report, error) {
	if err := s.validate(filters); err != nil {
		return nil, err
	}

	careDays, excluded, err := s.careDays(ctx, filters.From, filters.To)
	if err != nil {
		return nil, err
	}

	students, err := s.cfg.Students.FindOverlappingWithGroups(ctx, filters.From, filters.To)
	if err != nil {
		return nil, fmt.Errorf("load students: %w", err)
	}
	students = filterStudentsByGroup(students, filters.GroupIDs)

	attendance, err := s.cfg.Statistics.AttendanceDays(ctx, filters.From, filters.To)
	if err != nil {
		return nil, fmt.Errorf("load attendance days: %w", err)
	}
	statusDays, err := s.cfg.Statistics.StatusDays(ctx, filters.From, filters.To)
	if err != nil {
		return nil, fmt.Errorf("load status days: %w", err)
	}

	report := &Report{
		From:         filters.From,
		To:           filters.To,
		CareDays:     len(careDays),
		ExcludedDays: excluded,
	}
	report.Students = buildStudentRows(students, careDays, attendance, statusDays)
	report.Groups = buildGroupRows(report.Students)
	report.Totals = buildTotals(report.Students)

	rooms, err := s.roomRows(ctx, filters)
	if err != nil {
		return nil, err
	}
	report.Rooms = rooms
	report.RoomDataDays, err = s.roomRetentionDays(ctx)
	if err != nil {
		return nil, err
	}
	report.RoomDataFrom = s.today().AddDays(-report.RoomDataDays)
	return report, nil
}

// careDays returns the ordered set of care days in the window and the
// exclusion breakdown.
func (s *service) careDays(ctx context.Context, from, to timezone.Date) (map[timezone.Date]bool, ExcludedDays, error) {
	var excluded ExcludedDays
	holidays := map[timezone.Date]bool{}
	closing := map[timezone.Date]bool{}
	vacation := map[timezone.Date]bool{}

	if s.cfg.Holidays != nil {
		set, err := s.cfg.Holidays.HolidayDates(ctx, from, to)
		if err != nil {
			return nil, excluded, fmt.Errorf("load public holidays: %w", err)
		}
		holidays = set
	}
	if s.cfg.ClosingDays != nil {
		set, err := s.cfg.ClosingDays.ClosingDayDates(ctx, from, to)
		if err != nil {
			return nil, excluded, fmt.Errorf("load closing days: %w", err)
		}
		closing = set
	}
	if s.cfg.Periods != nil {
		periods, err := s.cfg.Periods.FindActiveOverlappingByType(ctx, scheduleModels.PeriodTypeHoliday, from, to, 0)
		if err != nil {
			return nil, excluded, fmt.Errorf("load holiday periods: %w", err)
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
	}

	care := map[timezone.Date]bool{}
	for d := from; !d.After(to); d = d.AddDays(1) {
		if wd := d.Weekday(); wd == time.Saturday || wd == time.Sunday {
			continue
		}
		off := false
		if holidays[d] {
			excluded.PublicHolidays++
			off = true
		}
		if closing[d] {
			excluded.ClosingDays++
			off = true
		}
		if vacation[d] {
			excluded.HolidayPeriods++
			off = true
		}
		if off {
			excluded.Total++
			continue
		}
		care[d] = true
	}
	return care, excluded, nil
}

func filterStudentsByGroup(students []*userModels.StudentWithGroupInfo, groupIDs []int64) []*userModels.StudentWithGroupInfo {
	if len(groupIDs) == 0 {
		return students
	}
	wanted := make(map[int64]bool, len(groupIDs))
	for _, id := range groupIDs {
		wanted[id] = true
	}
	out := make([]*userModels.StudentWithGroupInfo, 0, len(students))
	for _, st := range students {
		if st.Student == nil {
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

func buildStudentRows(students []*userModels.StudentWithGroupInfo, careDays map[timezone.Date]bool, attendance []activeModels.AttendanceDayRow, statusDays []activeModels.StatusDayRow) []StudentRow {
	present := make(map[dayKey]bool, len(attendance))
	for _, row := range attendance {
		present[dayKey{row.StudentID, row.Date}] = true
	}
	// sick beats excused; class_trip counts as excused.
	status := make(map[dayKey]string, len(statusDays))
	for _, row := range statusDays {
		key := dayKey{row.StudentID, row.Date}
		switch row.Status {
		case activeModels.StudentStatusDaySick:
			status[key] = activeModels.StudentStatusDaySick
		case activeModels.StudentStatusDayExcused, activeModels.StudentStatusDayClassTrip:
			if status[key] != activeModels.StudentStatusDaySick {
				status[key] = activeModels.StudentStatusDayExcused
			}
		}
	}

	rows := make([]StudentRow, 0, len(students))
	for _, st := range students {
		if st == nil || st.Student == nil {
			continue
		}
		row := StudentRow{
			StudentID:   st.ID,
			SchoolClass: st.SchoolClass,
			GroupID:     st.GroupID,
			GroupName:   st.GroupName,
		}
		if st.Person != nil {
			row.FirstName = st.Person.FirstName
			row.LastName = st.Person.LastName
		}
		for day := range careDays {
			if !studentEnrolledOn(st.Student, day) {
				continue
			}
			row.CareDays++
			key := dayKey{st.ID, day}
			switch {
			case present[key]:
				row.PresentDays++
			case status[key] == activeModels.StudentStatusDaySick:
				row.SickDays++
			case status[key] == activeModels.StudentStatusDayExcused:
				row.ExcusedDays++
			default:
				row.UnexplainedDays++
			}
		}
		row.AttendanceRate = rate(row.PresentDays, row.CareDays)
		rows = append(rows, row)
	}
	sort.SliceStable(rows, func(i, j int) bool {
		if c := strings.Compare(sortKey(rows[i].LastName), sortKey(rows[j].LastName)); c != 0 {
			return c < 0
		}
		if c := strings.Compare(sortKey(rows[i].FirstName), sortKey(rows[j].FirstName)); c != 0 {
			return c < 0
		}
		return rows[i].StudentID < rows[j].StudentID
	})
	return rows
}

func studentEnrolledOn(student *userModels.Student, day timezone.Date) bool {
	return (student.EnrolledFrom == nil || !day.Before(*student.EnrolledFrom)) &&
		(student.EnrolledUntil == nil || !day.After(*student.EnrolledUntil))
}

// NoGroupName labels the pseudo group of children without a group.
const NoGroupName = "Ohne Gruppe"

func buildGroupRows(students []StudentRow) []GroupRow {
	byGroup := map[int64]*GroupRow{}
	for _, st := range students {
		id := int64(0)
		name := NoGroupName
		if st.GroupID != nil {
			id = *st.GroupID
			name = st.GroupName
		}
		row, ok := byGroup[id]
		if !ok {
			row = &GroupRow{GroupID: id, Name: name}
			byGroup[id] = row
		}
		row.StudentCount++
		row.PresentDays += st.PresentDays
		row.SickDays += st.SickDays
		row.ExcusedDays += st.ExcusedDays
		row.UnexplainedDays += st.UnexplainedDays
	}
	rows := make([]GroupRow, 0, len(byGroup))
	for _, row := range byGroup {
		careDays := 0
		for _, student := range students {
			if (student.GroupID == nil && row.GroupID == 0) || (student.GroupID != nil && *student.GroupID == row.GroupID) {
				careDays += student.CareDays
			}
		}
		row.AttendanceRate = rate(row.PresentDays, careDays)
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

// sortKey folds case and German umlauts so "Bärengruppe" sorts before
// "Blumengruppe" (byte order would put every umlaut after z).
func sortKey(s string) string {
	return umlautFolder.Replace(strings.ToLower(s))
}

var umlautFolder = strings.NewReplacer("ä", "a", "ö", "o", "ü", "u", "ß", "ss")

func buildTotals(students []StudentRow) GroupRow {
	total := GroupRow{Name: "Gesamt"}
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

func (s *service) roomRows(ctx context.Context, filters Filters) ([]RoomRow, error) {
	start := filters.From.BerlinMidnight()
	end := filters.To.AddDays(1).BerlinMidnight()
	agg, err := s.cfg.Statistics.RoomUtilization(ctx, start, end, filters.GroupIDs)
	if err != nil {
		return nil, fmt.Errorf("load room utilization: %w", err)
	}
	rooms, err := s.cfg.Rooms.List(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("load rooms: %w", err)
	}
	byID := make(map[int64]activeModels.RoomUtilizationRow, len(agg))
	for _, row := range agg {
		byID[row.RoomID] = row
	}
	out := make([]RoomRow, 0, len(rooms))
	for _, room := range rooms {
		if room == nil {
			continue
		}
		row := RoomRow{RoomID: room.ID, Name: room.Name, Capacity: room.Capacity}
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

func (s *service) roomRetentionDays(ctx context.Context) (int, error) {
	defaultDays := userModels.DefaultDataRetentionDays
	if s.cfg.Settings != nil {
		defaultDays = configService.ResolveIntOrDefault(ctx, s.cfg.Settings, configModel.KeyPrivacyConsentRetentionDays, defaultDays, s.cfg.Logger)
	}
	if s.cfg.PrivacyConsents == nil {
		return defaultDays, nil
	}
	settings, err := s.cfg.PrivacyConsents.ListAcceptedRetentionSettings(ctx)
	if err != nil {
		return 0, fmt.Errorf("load visit retention settings: %w", err)
	}
	if len(settings) == 0 {
		return defaultDays, nil
	}
	retentionDays := 0
	for _, setting := range settings {
		if setting.DataRetentionDays > retentionDays {
			retentionDays = setting.DataRetentionDays
		}
	}
	return retentionDays, nil
}

func (s *service) recordAccess(ctx context.Context, filters Filters, actor Actor, action, format string, dedup bool) error {
	if s.cfg.AccessLog == nil {
		return fmt.Errorf("%w: access log not configured", ErrAuditFailed)
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
	if len(filters.GroupIDs) > 0 {
		groupIDs := append([]int64(nil), filters.GroupIDs...)
		sort.Slice(groupIDs, func(i, j int) bool { return groupIDs[i] < groupIDs[j] })
		meta["group_ids"] = strings.Trim(strings.Join(strings.Fields(fmt.Sprint(groupIDs)), ","), "[]")
	}
	now := s.cfg.Now()
	if dedup {
		exists, err := s.cfg.AccessLog.ExistsSince(ctx, actor.AccountID, auditModels.ResourceTypeAttendanceStatistics, meta, now.Add(-viewDedupWindow))
		if err != nil {
			return fmt.Errorf("%w: %v", ErrAuditFailed, err)
		}
		if exists {
			return nil
		}
	}
	entry := &auditModels.DataAccessLog{
		ActorAccountID: actor.AccountID,
		ActorRole:      role,
		ResourceType:   auditModels.ResourceTypeAttendanceStatistics,
		RangeStart:     filters.From.BerlinMidnight(),
		RangeEnd:       filters.To.EndOfDay(),
		AccessedAt:     now,
	}
	for k, v := range meta {
		entry.SetMetadata(k, v)
	}
	if format != "" {
		entry.SetMetadata("format", format)
	}
	if groupIDs, ok := meta["group_ids"]; ok {
		entry.SetMetadata("group_ids", groupIDs)
	}
	if err := s.cfg.AccessLog.Create(ctx, entry); err != nil {
		s.cfg.Logger.Error("statistics audit write failed",
			slog.String("action", action),
			slog.String("error", err.Error()),
		)
		return fmt.Errorf("%w: %v", ErrAuditFailed, err)
	}
	return nil
}
