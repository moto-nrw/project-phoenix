package api

import (
	"context"
	"time"

	peopleValues "github.com/moto-nrw/project-phoenix/modules/peopledirectory"

	auditModels "github.com/moto-nrw/project-phoenix/modules/auditlog/records"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/facilities"

	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar"
	presenceCompose "github.com/moto-nrw/project-phoenix/modules/studentpresence/compose"
	timezone "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

type StatisticsAuditLog struct {
	Log accessLogLedger
}

func (a StatisticsAuditLog) RecordStatisticsAccess(ctx context.Context, event presenceCompose.StatisticsAccessEvent) error {
	entry := &auditModels.DataAccessLog{
		ActorAccountID: event.ActorAccountID, ActorRole: event.ActorRole,
		ResourceType: auditModels.ResourceTypeAttendanceStatistics,
		RangeStart:   event.RangeStart, RangeEnd: event.RangeEnd, AccessedAt: event.AccessedAt,
	}
	for key, value := range event.Metadata {
		entry.SetMetadata(key, value)
	}
	return a.Log.Create(ctx, entry)
}

func (a StatisticsAuditLog) SeenStatisticsAccessSince(ctx context.Context, actorID int64, metadata map[string]string, since time.Time) (bool, error) {
	return a.Log.ExistsSince(ctx, actorID, auditModels.ResourceTypeAttendanceStatistics, metadata, since)
}

type StatisticsReportRooms struct {
	Rooms interface {
		ListRooms(context.Context, facilities.RoomFilter) ([]facilities.Room, error)
	}
}

func (a StatisticsReportRooms) StatisticsRooms(ctx context.Context) ([]presenceCompose.StatisticsRoom, error) {
	values, err := a.Rooms.ListRooms(ctx, facilities.RoomFilter{})
	if err != nil {
		return nil, err
	}
	result := make([]presenceCompose.StatisticsRoom, 0, len(values))
	for _, value := range values {
		result = append(result, presenceCompose.StatisticsRoom{ID: value.ID, Name: value.Name, Capacity: value.Capacity})
	}
	return result, nil
}

type StatisticsReportPeriods struct {
	Periods interface {
		ListCalendarPeriods(context.Context, schoolcalendar.CalendarPeriodFilter) ([]schoolcalendar.CalendarPeriod, error)
	}
}

func (a StatisticsReportPeriods) StatisticsHolidayPeriods(ctx context.Context, from, to timezone.Date) ([]presenceCompose.HolidayPeriod, error) {
	values, err := a.Periods.ListCalendarPeriods(ctx, schoolcalendar.HolidaysOverlapping(from.String(), to.String()))
	if err != nil {
		return nil, err
	}
	result := make([]presenceCompose.HolidayPeriod, 0, len(values))
	for _, value := range values {
		result = append(result, presenceCompose.HolidayPeriod{StartDate: timezone.Date(value.StartDate), EndDate: timezone.Date(value.EndDate)})
	}
	return result, nil
}

// statisticsStudentRows are the student rows the report's roster reads.
type statisticsStudentRows interface {
	FindOverlappingWithGroups(context.Context, timezone.Date, timezone.Date, timezone.Date) ([]*peopleValues.StudentRosterEntry, error)
}

type StatisticsReportStudents struct {
	Students statisticsStudentRows
}

func (r StatisticsReportStudents) FindOverlappingWithGroups(ctx context.Context, from, to, today timezone.Date) ([]*presenceCompose.StatisticsStudent, error) {
	values, err := r.Students.FindOverlappingWithGroups(ctx, from, to, today)
	if err != nil {
		return nil, err
	}
	result := make([]*presenceCompose.StatisticsStudent, 0, len(values))
	for _, value := range values {
		if value == nil || value.Record == nil {
			continue
		}
		student := value.Record
		row := &presenceCompose.StatisticsStudent{
			ID: student.ID, SchoolClass: student.SchoolClass, GroupID: student.GroupID, GroupName: value.GroupName,
			EnrolledOn: func(day, reportToday timezone.Date) bool { return peopleValues.EnrolledOn(student, day, reportToday) },
		}
		if student.Person != nil {
			row.FirstName = student.Person.FirstName
			row.LastName = student.Person.LastName
		}
		result = append(result, row)
	}
	return result, nil
}

type StatisticsStatusDays struct {
	Query careplan.StudentStatusDaysQuery
}

func (a StatisticsStatusDays) StatusDays(ctx context.Context, from, to timezone.Date) ([]presenceCompose.StatusDay, error) {
	values, err := a.Query.ListStatusDaySummaries(ctx, careplan.Date(from), careplan.Date(to))
	if err != nil {
		return nil, err
	}
	result := make([]presenceCompose.StatusDay, 0, len(values))
	for _, value := range values {
		result = append(result, presenceCompose.StatusDay{StudentID: value.StudentID, Date: timezone.Date(value.Date), Status: value.Status})
	}
	return result, nil
}

// accessLogLedger is the Audit Platform's data access log a report appends
// to and checks for a recent entry.
type accessLogLedger interface {
	Create(context.Context, *auditModels.DataAccessLog) error
	ExistsSince(context.Context, int64, string, map[string]string, time.Time) (bool, error)
}
