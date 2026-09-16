package services

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	configModels "github.com/moto-nrw/project-phoenix/models/config"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/facilities"
	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence/legacy/statistics"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
)

type statisticsAuditLog struct {
	log interface {
		Create(context.Context, *auditModels.DataAccessLog) error
		ExistsSince(context.Context, int64, string, map[string]string, time.Time) (bool, error)
	}
}

func (a statisticsAuditLog) RecordStatisticsAccess(ctx context.Context, event statistics.AccessEvent) error {
	entry := &auditModels.DataAccessLog{
		ActorAccountID: event.ActorAccountID, ActorRole: event.ActorRole,
		ResourceType: auditModels.ResourceTypeAttendanceStatistics,
		RangeStart:   event.RangeStart, RangeEnd: event.RangeEnd, AccessedAt: event.AccessedAt,
	}
	for key, value := range event.Metadata {
		entry.SetMetadata(key, value)
	}
	return a.log.Create(ctx, entry)
}

func (a statisticsAuditLog) SeenStatisticsAccessSince(ctx context.Context, actorID int64, metadata map[string]string, since time.Time) (bool, error) {
	return a.log.ExistsSince(ctx, actorID, auditModels.ResourceTypeAttendanceStatistics, metadata, since)
}

type statisticsReportRooms struct {
	rooms interface {
		ListRooms(context.Context, facilities.RoomFilter) ([]facilities.Room, error)
	}
}

func (a statisticsReportRooms) StatisticsRooms(ctx context.Context) ([]statistics.ReportRoom, error) {
	values, err := a.rooms.ListRooms(ctx, facilities.RoomFilter{})
	if err != nil {
		return nil, err
	}
	result := make([]statistics.ReportRoom, 0, len(values))
	for _, value := range values {
		result = append(result, statistics.ReportRoom{ID: value.ID, Name: value.Name, Capacity: value.Capacity})
	}
	return result, nil
}

type statisticsReportPeriods struct {
	periods interface {
		ListCalendarPeriods(context.Context, schoolcalendar.CalendarPeriodFilter) ([]schoolcalendar.CalendarPeriod, error)
	}
}

func (a statisticsReportPeriods) StatisticsHolidayPeriods(ctx context.Context, from, to timezone.Date) ([]statistics.HolidayPeriod, error) {
	values, err := a.periods.ListCalendarPeriods(ctx, schoolcalendar.CalendarPeriodFilter{
		PeriodType: schoolcalendar.PeriodTypeHoliday, ActiveOnly: true, OverlappingFrom: from.String(), OverlappingTo: to.String(),
	})
	if err != nil {
		return nil, err
	}
	result := make([]statistics.HolidayPeriod, 0, len(values))
	for _, value := range values {
		result = append(result, statistics.HolidayPeriod{StartDate: timezone.Date(value.StartDate), EndDate: timezone.Date(value.EndDate)})
	}
	return result, nil
}

type statisticsReportCourses struct {
	courses interface {
		CourseInstances(context.Context, string, string, string) ([]timetable.CourseInstanceRow, error)
		CourseParticipation(context.Context, string, string, string) ([]timetable.CourseParticipationRow, error)
	}
}

func (a statisticsReportCourses) CourseInstances(ctx context.Context, from, to, today timezone.Date) ([]statistics.CourseInstance, error) {
	values, err := a.courses.CourseInstances(ctx, from.String(), to.String(), today.String())
	if err != nil {
		return nil, err
	}
	result := make([]statistics.CourseInstance, 0, len(values))
	for _, value := range values {
		result = append(result, statistics.CourseInstance(value))
	}
	return result, nil
}

func (a statisticsReportCourses) CourseParticipation(ctx context.Context, from, to, today timezone.Date) ([]statistics.CourseParticipation, error) {
	values, err := a.courses.CourseParticipation(ctx, from.String(), to.String(), today.String())
	if err != nil {
		return nil, err
	}
	result := make([]statistics.CourseParticipation, 0, len(values))
	for _, value := range values {
		result = append(result, statistics.CourseParticipation(value))
	}
	return result, nil
}

type statisticsRetention struct {
	settings interface {
		ResolveInt(context.Context, string) (int, error)
	}
}

func (r statisticsRetention) RoomRetentionDays(ctx context.Context) (int, error) {
	return r.settings.ResolveInt(ctx, configModels.KeyPrivacyConsentRetentionDays)
}
func (r statisticsRetention) CourseRetentionDays(ctx context.Context) (int, error) {
	return r.settings.ResolveInt(ctx, configModels.KeyGDPRTimetableRetentionDays)
}

type statisticsReportStudents struct {
	students interface {
		FindOverlappingWithGroups(context.Context, timezone.Date, timezone.Date, timezone.Date) ([]*userModels.StudentWithGroupInfo, error)
	}
}

func (r statisticsReportStudents) FindOverlappingWithGroups(ctx context.Context, from, to, today timezone.Date) ([]*statistics.ReportStudent, error) {
	values, err := r.students.FindOverlappingWithGroups(ctx, from, to, today)
	if err != nil {
		return nil, err
	}
	result := make([]*statistics.ReportStudent, 0, len(values))
	for _, value := range values {
		if value == nil || value.Student == nil {
			continue
		}
		student := value.Student
		row := &statistics.ReportStudent{
			ID: student.ID, SchoolClass: student.SchoolClass, GroupID: student.GroupID, GroupName: value.GroupName,
			EnrolledOn: func(day, reportToday timezone.Date) bool { return userModels.EnrolledOn(student, day, reportToday) },
		}
		if student.Person != nil {
			row.FirstName = student.Person.FirstName
			row.LastName = student.Person.LastName
		}
		result = append(result, row)
	}
	return result, nil
}

type statisticsRetentionSettings struct {
	consents interface {
		ListAcceptedRetentionSettings(context.Context) ([]studentpresence.StudentRetentionSetting, error)
	}
}

func (r statisticsRetentionSettings) ListAcceptedRetentionSettings(ctx context.Context) ([]statistics.RetentionSetting, error) {
	values, err := r.consents.ListAcceptedRetentionSettings(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]statistics.RetentionSetting, 0, len(values))
	for _, value := range values {
		result = append(result, statistics.RetentionSetting(value))
	}
	return result, nil
}

type statisticsAttendance struct {
	query interface {
		ListAttendanceDays(context.Context, string, string) ([]studentpresence.AttendanceDay, error)
	}
}

func (a statisticsAttendance) AttendanceDays(ctx context.Context, from, to timezone.Date) ([]statistics.AttendanceDay, error) {
	values, err := a.query.ListAttendanceDays(ctx, from.String(), to.String())
	if err != nil {
		return nil, err
	}
	result := make([]statistics.AttendanceDay, 0, len(values))
	for _, value := range values {
		result = append(result, statistics.AttendanceDay{StudentID: value.StudentID, Date: timezone.Date(value.Date)})
	}
	return result, nil
}

type statisticsStatusDays struct {
	query careplan.StudentStatusDaysQuery
}

func (a statisticsStatusDays) StatusDays(ctx context.Context, from, to timezone.Date) ([]statistics.StatusDay, error) {
	values, err := a.query.ListStatusDaySummaries(ctx, careplan.Date(from), careplan.Date(to))
	if err != nil {
		return nil, err
	}
	result := make([]statistics.StatusDay, 0, len(values))
	for _, value := range values {
		result = append(result, statistics.StatusDay{StudentID: value.StudentID, Date: timezone.Date(value.Date), Status: value.Status})
	}
	return result, nil
}

type statisticsRoomUtilization struct {
	query interface {
		RoomUtilization(context.Context, []studentpresence.StudentVisitWindow) ([]studentpresence.RoomUtilization, error)
	}
}

func (a statisticsRoomUtilization) RoomUtilization(ctx context.Context, windows []statistics.StudentVisitWindow) ([]statistics.RoomUtilization, error) {
	input := make([]studentpresence.StudentVisitWindow, 0, len(windows))
	for _, window := range windows {
		input = append(input, studentpresence.StudentVisitWindow(window))
	}
	rows, err := a.query.RoomUtilization(ctx, input)
	if err != nil {
		return nil, err
	}
	result := make([]statistics.RoomUtilization, 0, len(rows))
	for _, row := range rows {
		result = append(result, statistics.RoomUtilization(row))
	}
	return result, nil
}
