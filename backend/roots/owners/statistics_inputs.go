package owners

import (
	"context"
	"time"

	peopleValues "github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	peopleBirthday "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"

	auditModels "github.com/moto-nrw/project-phoenix/modules/auditlog/records"
	"github.com/moto-nrw/project-phoenix/modules/carecontract"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/enrollment"
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

// statisticsBirthdays are the children of the school by date of birth.
type statisticsBirthdays interface {
	FindBirthdaysOn(context.Context, []peopleBirthday.BirthdayMonthDay) ([]peopleBirthday.BirthdayEntry, error)
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
			byDate[date] = studentpresence.HolidayCareDay{Arrival: day.StartsAt, Pickup: day.EndsAt}
		}
		result[studentID] = byDate
	}
	return result, nil
}

// StatisticsHolidayStudents reads the children the school cares for that
// were born on the days of the own holiday bookings, in one read of the
// People Directory (#5994): Enrollment finds the child of a booking among
// them. The read is by day of the year; only an equal date counts.
type StatisticsHolidayStudents struct {
	Students statisticsBirthdays
}

func (a StatisticsHolidayStudents) HolidayStudentsBornOn(ctx context.Context, birthDates []timezone.Date) ([]enrollment.HolidayStudentCandidate, error) {
	if len(birthDates) == 0 {
		return nil, nil
	}
	wanted := make(map[timezone.Date]bool, len(birthDates))
	days := make([]peopleBirthday.BirthdayMonthDay, 0, len(birthDates))
	for _, date := range birthDates {
		wanted[date] = true
		days = append(days, peopleBirthday.BirthdayMonthDay{Month: date.Month(), Day: date.Day()})
	}
	entries, err := a.Students.FindBirthdaysOn(ctx, days)
	if err != nil {
		return nil, err
	}
	result := make([]enrollment.HolidayStudentCandidate, 0, len(entries))
	for _, entry := range entries {
		if wanted[entry.Birthday] {
			result = append(result, enrollment.HolidayStudentCandidate{StudentID: entry.ID,
				Identity: enrollment.HolidayStudentIdentity{FirstName: entry.FirstName, LastName: entry.LastName, BirthDate: entry.Birthday}})
		}
	}
	return result, nil
}
