package application

import (
	"context"
	"strings"

	"github.com/moto-nrw/project-phoenix/internal/schoolclass"
	"github.com/moto-nrw/project-phoenix/modules/classday"
	"github.com/moto-nrw/project-phoenix/modules/classday/internal/ports"
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
}

// classDayService is the capability: the caller's classes, the day report
// and supervision sheet (dayReports) and the arrival-exception seam.
type classDayService struct {
	caller ports.Caller
	*dayReports
	arrivalExceptions
}

// NewClassDay creates the school-portal capability. Caller and Rosters are
// required: without them no class can be resolved and no sheet served.
func NewClassDay(deps ClassDayDependencies) classday.ClassDay {
	if deps.Caller == nil || deps.Rosters == nil {
		panic("classday application.NewClassDay: Caller and Rosters dependencies are required")
	}
	return &classDayService{
		caller:     deps.Caller,
		dayReports: newDayReports(deps),
		arrivalExceptions: arrivalExceptions{
			schedule: deps.ClassArrivalExceptions, writeScope: deps.WriteScope,
			blockStarts: deps.BlockStarts, announcer: deps.Announcer,
		},
	}
}

func (s *classDayService) AssignedClasses(ctx context.Context) ([]string, error) {
	return s.caller.AssignedClasses(ctx)
}

// ResolveClass picks the class to show: the requested one when it is one of
// the caller's assignments (normalized comparison), otherwise the first
// assigned class when nothing was requested.
func (s *classDayService) ResolveClass(ctx context.Context, requested string) (string, error) {
	assigned, err := s.caller.AssignedClasses(ctx)
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
		if schoolclass.Normalize(class) == schoolclass.Normalize(requested) {
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

func (s *classDayService) CurrentStaffID(ctx context.Context) (int64, error) {
	return s.caller.StaffID(ctx)
}

// invalidFilterError refuses a report request before anything is read. Its
// text is the one the enrollment report put on the wire before the day
// report moved here; it answers errors.Is for the projection's sentinel.
type invalidFilterError struct{}

func (invalidFilterError) Error() string { return "enrollment report filter is invalid" }

func (invalidFilterError) Is(target error) bool { return target == classday.ErrInvalidReportFilter }

// errInvalidFilter is the refusal the reports wrap.
var errInvalidFilter error = invalidFilterError{}
