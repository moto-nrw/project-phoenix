package application

import (
	"context"
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/classday"
	"github.com/moto-nrw/project-phoenix/modules/classday/internal/ports"
	"github.com/moto-nrw/project-phoenix/modules/schoolstructure"
)

// ClassDayDependencies wires the school-portal capability of the projection.
// Caller and Rosters are required. StatusDays, Times and CareDays are
// required by the day report, which fails fast without them. Companions,
// Students, EmergencyContacts and AccessLog are optional: without them a
// departure names no companion, no sheet is served, the sheet lists no
// contact and no read is logged. ClassArrivalExceptions is the one write
// seam of moto schule (#2970); without it the exception capabilities answer
// classday.ErrArrivalExceptionsNotConfigured and the write verdict stays
// false, so the HTTP route table is stable across wirings.
type ClassDayDependencies struct {
	Caller                 ports.Caller
	Rosters                DayRosters
	StatusDays             StatusDays
	Times                  EffectiveDayTimes
	CareDays               ports.CareDays
	Companions             Companions
	Students               SheetStudents
	EmergencyContacts      EmergencyContacts
	AccessLog              AccessLog
	ClassArrivalExceptions ClassArrivalExceptions
	WriteScope             ArrivalWriteScope
	BlockStarts            BlockStarts
	Announcer              ArrivalScheduleAnnouncer
	// TripScope, TripStatusDays and StudentArrivals are the class-trip write
	// seam (#4799); without the two writes it answers "not configured".
	TripScope       ClassTripWriteScope
	TripStatusDays  ClassTripStatusDays
	StudentArrivals studentArrivals
	// MorningAttendanceDependencies are the morning attendance seam (#4790).
	MorningAttendanceDependencies
	// HomeworkDependencies are the homework seam (#4796).
	HomeworkDependencies
	// ClassHandoverDependencies are the class handover seam (#4794).
	ClassHandoverDependencies
}

// classDayService is the capability: the caller's classes and staff row
// (promoted from the Caller port), the day report and supervision sheet
// (dayReports), the arrival-exception and the class-trip seams. Its own
// reads go through caller, the same value.
type classDayService struct {
	ports.Caller
	caller ports.Caller
	*dayReports
	arrivalExceptions
	classTrips
	morningAttendance
	homework
	classHandover
}

// NewClassDay creates the school-portal capability. Caller and Rosters are
// required: without them no class can be resolved and no sheet served.
func NewClassDay(deps ClassDayDependencies) classday.ClassDay {
	if deps.Caller == nil || deps.Rosters == nil {
		panic("classday application.NewClassDay: Caller and Rosters dependencies are required")
	}
	return &classDayService{
		Caller:     deps.Caller,
		caller:     deps.Caller,
		dayReports: newDayReports(deps),
		arrivalExceptions: arrivalExceptions{
			schedule: deps.ClassArrivalExceptions, writeScope: deps.WriteScope,
			blockStarts: deps.BlockStarts, announcer: deps.Announcer,
		},
		classTrips: classTrips{
			rosters: deps.Rosters, writeScope: deps.TripScope, statusDays: deps.TripStatusDays,
			arrivals: deps.StudentArrivals, announcer: deps.Announcer,
		},
		morningAttendance: newMorningAttendance(deps),
		homework:          newHomework(deps),
		classHandover:     newClassHandover(deps),
	}
}

// ResolveClass picks the class to show: the requested one when it is one of
// the caller's assignments (normalized comparison), otherwise the first
// assigned class when nothing was requested.
func (s *classDayService) ResolveClass(ctx context.Context, requested string) (string, error) {
	assigned, err := s.caller.GetMySchoolClasses(ctx)
	if err != nil {
		return "", err
	}
	return resolveRequestedClass(requested, assigned)
}

func resolveRequestedClass(requested string, assigned []string) (string, error) {
	requested = strings.TrimSpace(requested)
	if requested == "" {
		if len(assigned) == 0 {
			return "", classday.ErrClassNotAssigned
		}
		return assigned[0], nil
	}
	for _, class := range assigned {
		if schoolstructure.NormalizeClass(class) == schoolstructure.NormalizeClass(requested) {
			return class, nil
		}
	}
	return "", classday.ErrClassNotAssigned
}

func (s *classDayService) DayReport(ctx context.Context, schoolClass string, date classday.Date, actor classday.Actor) (*classday.DayReport, error) {
	day, err := parseDate(date)
	if err != nil {
		return nil, err
	}
	return s.classDay(ctx, schoolClass, day, actor)
}

// invalidFilterError refuses a report request before anything is read. Its
// text is the one the enrollment report put on the wire before the day
// report moved here; it answers errors.Is for the projection's sentinel.
type invalidFilterError struct{}

func (invalidFilterError) Error() string { return "enrollment report filter is invalid" }

func (invalidFilterError) Is(target error) bool { return target == classday.ErrInvalidReportFilter }

// errInvalidFilter is the refusal the reports wrap.
var errInvalidFilter error = invalidFilterError{}
