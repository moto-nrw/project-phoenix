package careplan

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// ClassArrivalExceptionInput is what a person enters for one class and date.
type ClassArrivalExceptionInput struct {
	SchoolClass string
	Date        calendar.Date
	// ArrivalTime is a wall-clock value; only hour and minute are used.
	ArrivalTime time.Time
	Reason      *string
	// Origin is the portal the entry comes from (#2970); empty means the
	// OGS portal.
	Origin string
}

var (
	// ErrClassArrivalExceptionNotConfigured means the service was built
	// without the exception repository.
	ErrClassArrivalExceptionNotConfigured = errors.New("class arrival exceptions are not configured")
	// ErrClassArrivalExceptionPastDate refuses writes and deletes for dates
	// before today: the day already happened, nothing downstream re-reads it.
	ErrClassArrivalExceptionPastDate = errors.New("class arrival exception date lies in the past")
	// ErrClassArrivalExceptionWeekend refuses exceptions that the arrival
	// projection would never apply because care days only run Monday–Friday.
	ErrClassArrivalExceptionWeekend = errors.New("class arrival exceptions can only be set from Monday to Friday")
	// ErrClassArrivalExceptionClassNotFound means no active child carries the
	// class, so the exception would apply to nobody.
	ErrClassArrivalExceptionClassNotFound = errors.New("school class has no active students")
	// ErrClassArrivalExceptionNotFound means there is nothing to delete.
	ErrClassArrivalExceptionNotFound = errors.New("class arrival exception not found")
)

const ClassArrivalExceptionOriginOGS = "ogs"
const ClassArrivalExceptionOriginSchool = "school"

// ClassArrivalException is the native class-day value. Timetable owns its persistence.
type ClassArrivalException struct {
	ID          int64
	TenantID    int64
	CreatedAt   time.Time
	UpdatedAt   time.Time
	SchoolClass string
	Date        calendar.Date
	ArrivalTime time.Time
	Reason      *string
	CreatedBy   *int64
	Origin      string
}

// ClassArrivalExceptions applies Care Plan's class-day rules through the owning
// module's write boundary. It does not expose an ORM repository to callers.
type ClassArrivalExceptions interface {
	ListClassArrivalExceptions(context.Context, string, calendar.Date, calendar.Date) ([]*ClassArrivalException, error)
	UpsertClassArrivalException(context.Context, ClassArrivalExceptionInput, int64) (*ClassArrivalException, error)
	DeleteClassArrivalException(context.Context, string, calendar.Date) error
}

func (e *ClassArrivalException) Validate() error {
	if strings.TrimSpace(e.SchoolClass) == "" {
		return errors.New("school class is required")
	}
	if e.Date.IsZero() {
		return errors.New("date is required")
	}
	if e.ArrivalTime.IsZero() {
		return errors.New("arrival time is required")
	}
	if e.Reason != nil && utf8.RuneCountInString(*e.Reason) > 255 {
		return errors.New("reason cannot exceed 255 characters")
	}
	if e.Origin != "" && e.Origin != ClassArrivalExceptionOriginOGS && e.Origin != ClassArrivalExceptionOriginSchool {
		return errors.New("origin must be ogs or school")
	}
	return nil
}

// Label distinguishes class-wide changes from a child's individual plan.
func (e *ClassArrivalException) Label() string {
	class := strings.TrimSpace(e.SchoolClass)
	if len(class) > 7 && strings.EqualFold(class[:7], "klasse ") {
		class = strings.TrimSpace(class[7:])
	}
	if e.Reason != nil && strings.TrimSpace(*e.Reason) != "" {
		return "Klasse " + class + ": " + strings.TrimSpace(*e.Reason)
	}
	return "Klasse " + class + ": andere Ankunftszeit"
}
