package owners

import (
	"context"
	"time"

	peopleValues "github.com/moto-ogs/project-phoenix/modules/peopledirectory"

	auditModels "github.com/moto-ogs/project-phoenix/modules/auditlog/records"
	"github.com/moto-ogs/project-phoenix/modules/carecontract"
	"github.com/moto-ogs/project-phoenix/modules/careplan"
	"github.com/moto-ogs/project-phoenix/modules/enrollment"
	"github.com/moto-ogs/project-phoenix/modules/facilities"

	"github.com/moto-ogs/project-phoenix/modules/schoolcalendar"
	"github.com/moto-ogs/project-phoenix/modules/studentpresence"
	presenceCompose "github.com/moto-ogs/project-phoenix/modules/studentpresence/compose"
	timezone "github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
)

type StatisticsAuditLog struct {
	Log AccessLogLedger
}

func (a StatisticsAuditLog) RecordStatisticsAccess(ctx context.Context, event presenceCompose.StatisticsAccessEvent) error {
	entry := &auditModels.DataAccessLog{
		ActorAccountID: event.ActorAccountID, ActorRole: event.ActorRole, StudentID: event.StudentID,
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

// statisticsBirthdays are the children of the school by date of birth, also
// those whose care has ended.
type statisticsBirthdays interface {
	FindBornOn(context.Context, []timezone.Date) ([]peopleValues.StudentBorn, error)
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

// StatisticsPlannedCareDays reads Care Plan's verdict with the planned clock
// times for the occupancy by time of day (#4764) and Soll und Ist (#4774).
type StatisticsPlannedCareDays struct {
	CareDays careplan.CareDayQuery
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
	return result, nil
}

// StatisticsContractCareDays reads the days the care contracts of the
// Träger cover at the school, the Soll of Soll und Ist where a contract is
// in force (#4683, ADR 0156).
type StatisticsContractCareDays struct {
	Contracts carecontract.BookedCare
}

func (a StatisticsContractCareDays) ContractCareDays(ctx context.Context, studentIDs []int64, from, to timezone.Date) (map[int64]map[timezone.Date]studentpresence.ContractCareDay, error) {
	contracts, err := a.Contracts.BookedCare(ctx, studentIDs, from, to)
	if err != nil {
		return nil, err
	}
	result := make(map[int64]map[timezone.Date]studentpresence.ContractCareDay, len(contracts))
	for studentID, contract := range contracts {
		days := map[timezone.Date]studentpresence.ContractCareDay{}
		for date := from; !date.After(to); date = date.AddDays(1) {
			if booked := contract.Day(date); booked.InForce {
				days[date] = studentpresence.ContractCareDay{Booked: booked.Booked, Pickup: booked.EndsAt}
			}
		}
		result[studentID] = days
	}
	return result, nil
}

// StatisticsHolidayCareDays reads the days the confirmed holiday bookings of
// the school's holiday guests cover, the Soll of Soll und Ist on those days
// (#5994).
type StatisticsHolidayCareDays struct {
	Bookings enrollment.HolidayGuestBookedCare
}

func (a StatisticsHolidayCareDays) HolidayCareDays(ctx context.Context, studentIDs []int64, from, to timezone.Date) (map[int64]map[timezone.Date]studentpresence.HolidayCareDay, error) {
	booked, err := a.Bookings.HolidayGuestBookedDays(ctx, studentIDs, from, to)
	if err != nil {
		return nil, err
	}
	result := make(map[int64]map[timezone.Date]studentpresence.HolidayCareDay, len(booked))
	for studentID, days := range booked {
		byDate := make(map[timezone.Date]studentpresence.HolidayCareDay, len(days))
		for date, day := range days {
			byDate[date] = studentpresence.HolidayCareDay{Arrival: day.StartsAt, Pickup: day.EndsAt, Ambiguous: day.Ambiguous}
		}
		result[studentID] = byDate
	}
	return result, nil
}

// StatisticsHolidayStudents reads every child of the school born on the
// dates of the own holiday bookings, with its membership interval, in one
// read of the People Directory (#5994): Enrollment finds the child of a
// booked day among the children that belonged to the school on that day.
type StatisticsHolidayStudents struct {
	Students statisticsBirthdays
}

func (a StatisticsHolidayStudents) HolidayStudentsBornOn(ctx context.Context, birthDates []timezone.Date) ([]enrollment.HolidayStudentCandidate, error) {
	born, err := a.Students.FindBornOn(ctx, birthDates)
	if err != nil {
		return nil, err
	}
	// Only an equal date counts: a child born on the same day of another
	// year is another child.
	wanted := make(map[timezone.Date]bool, len(birthDates))
	for _, date := range birthDates {
		wanted[date] = true
	}
	result := make([]enrollment.HolidayStudentCandidate, 0, len(born))
	for _, child := range born {
		if !wanted[child.Birthday] {
			continue
		}
		result = append(result, enrollment.HolidayStudentCandidate{StudentID: child.StudentID,
			Identity:     enrollment.HolidayStudentIdentity{FirstName: child.FirstName, LastName: child.LastName, BirthDate: child.Birthday},
			EnrolledFrom: child.EnrolledFrom, EnrolledUntil: child.EnrolledUntil})
	}
	return result, nil
}
