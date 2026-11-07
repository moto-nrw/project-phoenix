package owners

import (
	"context"
	"time"

	peopleValues "github.com/moto-nrw/project-phoenix/modules/peopledirectory"

	auditModels "github.com/moto-nrw/project-phoenix/modules/auditlog/records"
	"github.com/moto-nrw/project-phoenix/modules/carecontract"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/modules/facilities"

	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	presenceCompose "github.com/moto-nrw/project-phoenix/modules/studentpresence/compose"
	timezone "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/tenant"
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
	FindByIDs(context.Context, []int64) (map[int64]*peopleValues.StudentState, error)
	FindEnrolledStudentIDByNameAndBirthday(ctx context.Context, tenantID int64, firstName, lastName string, birthday timezone.Date) (*int64, error)
}

// statisticsPersons are the persons behind the children, with their date of
// birth.
type statisticsPersons interface {
	FindByIDs(context.Context, []int64) (map[int64]*peopleValues.DirectoryPersonSnapshot, error)
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

// StatisticsHolidayStudentIdentities reads the names and the date of birth of
// the school's children in two reads of the People Directory, so Enrollment
// finds the children its own holiday bookings name (#5994).
type StatisticsHolidayStudentIdentities struct {
	Students statisticsStudentRows
	Persons  statisticsPersons
}

func (a StatisticsHolidayStudentIdentities) HolidayStudentIdentities(ctx context.Context, studentIDs []int64) (map[int64]enrollment.HolidayStudentIdentity, error) {
	students, err := a.Students.FindByIDs(ctx, studentIDs)
	if err != nil {
		return nil, err
	}
	personIDs := make([]int64, 0, len(students))
	for _, student := range students {
		if student != nil {
			personIDs = append(personIDs, student.PersonID)
		}
	}
	persons, err := a.Persons.FindByIDs(ctx, personIDs)
	if err != nil {
		return nil, err
	}
	result := make(map[int64]enrollment.HolidayStudentIdentity, len(students))
	for studentID, student := range students {
		if student == nil || persons[student.PersonID] == nil || persons[student.PersonID].Birthday == nil {
			continue
		}
		person := persons[student.PersonID]
		result[studentID] = enrollment.HolidayStudentIdentity{FirstName: person.FirstName, LastName: person.LastName, BirthDate: *person.Birthday}
	}
	return result, nil
}

// UniqueHolidayStudents finds for each identity the one enrolled child of
// the school with it, by the People Directory's case-insensitive match
// (#1663); none or several leave the identity out.
func (a StatisticsHolidayStudentIdentities) UniqueHolidayStudents(ctx context.Context, identities []enrollment.HolidayStudentIdentity) (map[int]int64, error) {
	school, err := tenant.TenantFromContext(ctx)
	if err != nil {
		return nil, err
	}
	result := make(map[int]int64, len(identities))
	for i, identity := range identities {
		id, err := a.Students.FindEnrolledStudentIDByNameAndBirthday(ctx, school.Int64(), identity.FirstName, identity.LastName, identity.BirthDate)
		if err != nil {
			return nil, err
		}
		if id != nil {
			result[i] = *id
		}
	}
	return result, nil
}
