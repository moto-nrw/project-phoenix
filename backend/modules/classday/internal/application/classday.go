package application

import (
	"context"
	"strings"

	"github.com/moto-nrw/project-phoenix/internal/schoolclass"
	"github.com/moto-nrw/project-phoenix/modules/classday"
	"github.com/moto-nrw/project-phoenix/modules/classday/internal/ports"
)

// ClassDayDependencies wires the school-portal capability of the projection.
// ArrivalExceptions is optional: without it the exception capabilities
// answer classday.ErrArrivalExceptionsNotConfigured and the write verdict
// stays false, so the HTTP route table is stable across wirings.
type ClassDayDependencies struct {
	Caller            ports.Caller
	Reports           ports.DayReports
	ArrivalExceptions ports.ArrivalExceptions
}

type classDayService struct {
	caller            ports.Caller
	reports           ports.DayReports
	arrivalExceptions ports.ArrivalExceptions
}

// NewClassDay creates the school-portal capability. Caller and Reports are
// required: without them no class can be resolved and no sheet served.
func NewClassDay(deps ClassDayDependencies) classday.ClassDay {
	if deps.Caller == nil || deps.Reports == nil {
		panic("classday application.NewClassDay: Caller and Reports dependencies are required")
	}
	return &classDayService{caller: deps.Caller, reports: deps.Reports, arrivalExceptions: deps.ArrivalExceptions}
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
	return s.reports.ClassDay(ctx, schoolClass, day, actor)
}

func (s *classDayService) CurrentStaffID(ctx context.Context) (int64, error) {
	return s.caller.StaffID(ctx)
}

func (s *classDayService) MayWriteArrivalExceptions(ctx context.Context) (bool, error) {
	if s.arrivalExceptions == nil {
		return false, classday.ErrArrivalExceptionsNotConfigured
	}
	return s.arrivalExceptions.SchoolMayWrite(ctx)
}

func (s *classDayService) ArrivalExceptions(ctx context.Context, schoolClass string, from, to classday.Date) ([]classday.ArrivalException, error) {
	if s.arrivalExceptions == nil {
		return nil, classday.ErrArrivalExceptionsNotConfigured
	}
	start, err := parseDate(from)
	if err != nil {
		return nil, err
	}
	end, err := parseDate(to)
	if err != nil {
		return nil, err
	}
	return s.arrivalExceptions.ListForClass(ctx, schoolClass, start, end)
}

func (s *classDayService) SetArrivalException(ctx context.Context, in classday.ArrivalExceptionWrite) (*classday.ArrivalException, error) {
	if s.arrivalExceptions == nil {
		return nil, classday.ErrArrivalExceptionsNotConfigured
	}
	day, err := parseDate(in.Date)
	if err != nil {
		return nil, err
	}
	return s.arrivalExceptions.Set(ctx, in, day)
}

func (s *classDayService) ClearArrivalException(ctx context.Context, schoolClass string, date classday.Date) error {
	if s.arrivalExceptions == nil {
		return classday.ErrArrivalExceptionsNotConfigured
	}
	day, err := parseDate(date)
	if err != nil {
		return err
	}
	return s.arrivalExceptions.Clear(ctx, schoolClass, day)
}

func (s *classDayService) EarliestBlockStart(ctx context.Context, schoolClass string, date classday.Date) (string, error) {
	if s.arrivalExceptions == nil {
		return "", classday.ErrArrivalExceptionsNotConfigured
	}
	day, err := parseDate(date)
	if err != nil {
		return "", err
	}
	return s.arrivalExceptions.EarliestBlockStart(ctx, schoolClass, day)
}
