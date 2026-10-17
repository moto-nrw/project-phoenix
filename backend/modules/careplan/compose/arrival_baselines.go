package compose

import (
	"context"
	"errors"

	carePlanPorts "github.com/moto-nrw/project-phoenix/modules/careplan/ports"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/application"
	calendar "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// ArrivalBaselineDependencies binds tenant-safe projections from People and
// Timetable. BookingMode is required even when a caller selects stored plans.
type ArrivalBaselineDependencies struct {
	Records         carePlanPorts.ArrivalBaselineRecords
	Bookings        careplan.ApprovedBookingReader
	BookingMode     func(context.Context) (bool, error)
	StudentClasses  func(context.Context, []int64) (map[int64]string, error)
	ClassPlans      func(context.Context, []string) ([]*ArrivalClassPlan, error)
	ClassExceptions func(context.Context, []string, calendar.Date, calendar.Date) (map[string]careplan.ClassArrivalExceptionsByDate, error)
}

func NewArrivalBaselines(deps ArrivalBaselineDependencies) (careplan.ArrivalBaselineReader, error) {
	if deps.Records == nil || deps.Bookings == nil || deps.BookingMode == nil || deps.StudentClasses == nil || deps.ClassPlans == nil || deps.ClassExceptions == nil {
		return nil, errors.New("arrival baselines: records, bookings, booking mode, students, class plans, and exceptions are required")
	}
	return application.NewArrivalBaselines(application.ArrivalBaselineDependencies(deps)), nil
}
