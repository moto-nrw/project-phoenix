package owners

import (
	"context"
	"time"

	peopleValues "github.com/moto-nrw/project-phoenix/modules/peopledirectory"

	auditModels "github.com/moto-nrw/project-phoenix/modules/auditlog/records"
	"github.com/moto-nrw/project-phoenix/modules/carecontract"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/facilities"

	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	presenceCompose "github.com/moto-nrw/project-phoenix/modules/studentpresence/compose"
	timezone "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

type StatisticsAuditLog struct {
	Log AccessLogLedger
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
			EnrolledOn: func(day, reportToday timezone.Date) bool {
				return peopleValues.EnrolledOn(student, day, reportToday)
			},
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

// AccessLogLedger is the Audit Platform's data access log a report appends
// to and checks for a recent entry.
type AccessLogLedger interface {
	Create(context.Context, *auditModels.DataAccessLog) error
	ExistsSince(context.Context, int64, string, map[string]string, time.Time) (bool, error)
}

// StatisticsPlannedCareDays reads the Soll for the occupancy by time of day
// (#4764) and Soll und Ist (#4774): Care Plan's verdict with the planned
// clock times, and where a care contract of the Träger is in force on a day,
// the contract's booked services instead (#4683, ADR 0154). The contract
// decides whether the day is booked and its latest module end is the pickup;
// it names no arrival, so the care plan's arrival and a cancellation of the
// day ("Kommt heute nicht") stay. Contracts nil reads the care plan alone.
type StatisticsPlannedCareDays struct {
	CareDays  careplan.CareDayQuery
	Contracts carecontract.BookedCare
}

func (a StatisticsPlannedCareDays) PlannedCareDays(ctx context.Context, studentIDs []int64, from, to timezone.Date) (map[int64]map[timezone.Date]studentpresence.PlannedCareDay, error) {
	values, err := a.CareDays.ResolvePlannedForRange(ctx, studentIDs, from, to)
	if err != nil {
		return nil, err
	}
	result := make(map[int64]map[timezone.Date]studentpresence.PlannedCareDay, len(values))
	for studentID, days := range values {
		byDate := make(map[timezone.Date]studentpresence.PlannedCareDay, len(days))
		for date, day := range days {
			byDate[date] = studentpresence.PlannedCareDay{
				Expected: day.Status.Expected(), NoPlan: day.Status == careplan.CareDayUnknown,
				Cancelled: day.Status == careplan.CareDayCancelled, Arrival: day.Arrival, Pickup: day.Pickup,
				Source: studentpresence.PlannedCareSourceCarePlan,
			}
		}
		result[studentID] = byDate
	}
	if a.Contracts == nil {
		return result, nil
	}
	contracts, err := a.Contracts.BookedCare(ctx, studentIDs, from, to)
	if err != nil {
		return nil, err
	}
	for studentID, contract := range contracts {
		if result[studentID] == nil {
			result[studentID] = map[timezone.Date]studentpresence.PlannedCareDay{}
		}
		for date := from; !date.After(to); date = date.AddDays(1) {
			booked := contract.Day(date)
			if booked.InForce {
				result[studentID][date] = contractCareDay(booked, result[studentID][date])
			}
		}
	}
	return result, nil
}

// contractCareDay is the Soll of a day the contract covers, over the care
// plan's verdict of that day.
func contractCareDay(booked carecontract.BookedDay, plan studentpresence.PlannedCareDay) studentpresence.PlannedCareDay {
	day := studentpresence.PlannedCareDay{Source: studentpresence.PlannedCareSourceContract}
	if !booked.Booked {
		return day
	}
	day.Expected, day.Cancelled = !plan.Cancelled, plan.Cancelled
	day.Arrival, day.Pickup = plan.Arrival, booked.EndsAt
	return day
}
