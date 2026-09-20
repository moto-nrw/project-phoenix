package ports

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// ClassArrivalExceptionStore is implemented by a boundary to Timetable,
// which retains ownership of education.class_arrival_exceptions.
type ClassArrivalExceptionStore interface {
	List(context.Context, []string, calendar.Date, calendar.Date) ([]*careplan.ClassArrivalException, error)
	Upsert(context.Context, *careplan.ClassArrivalException) error
	Delete(context.Context, string, calendar.Date) (bool, error)
}
type ClassArrivalStudents interface {
	HasActiveClass(context.Context, string, calendar.Date) (bool, error)
}
