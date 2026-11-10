// Package ports declares what the holiday closure notification consumes of
// its owners (ADR 0196). The root binds every port.
package ports

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// Proposals is School Calendar's proposal side.
type Proposals = schoolcalendar.HolidayClosureProposals

// Queue is School Calendar's command side.
type Queue = schoolcalendar.HolidayClosureEventQueue

// Closures is what the workflow asks of Enrollment, always in the Träger
// transaction of the Träger it names.
type Closures interface {
	PlanHolidayClosureOperation(ctx context.Context, organizationID int64, operation enrollment.HolidayClosureOperation) (enrollment.HolidayClosureImpact, error)
	NotifyHolidayClosureOperation(ctx context.Context, organizationID int64, operation enrollment.HolidayClosureOperation) (enrollment.HolidayClosureOperationResult, error)
	HolidayClosureOperationDelivery(ctx context.Context, organizationID int64, operationID string) (enrollment.HolidayClosureDelivery, error)
}

// NoCareDays names the days without care of a school in [from, to]: the
// statutory holidays of its federal state, the registry default when it
// stored none, and the closing days the calendar read names.
type NoCareDays func(ctx context.Context, school schoolcalendar.HolidayClosureCalendar, from, to calendar.Date) ([]enrollment.HolidayNoCareDay, error)

// Units opens the units of work. WithinSchool and WithinTraeger each open
// a unit of their own and refuse to nest in another one.
type Units interface {
	WithinSchool(ctx context.Context, schoolID int64, fn func(context.Context) error) error
	WithinTraeger(ctx context.Context, organizationID int64, fn func(context.Context) error) error
	WithinAdmin(ctx context.Context, fn func(context.Context) error) error
}
