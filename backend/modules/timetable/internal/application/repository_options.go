package application

import (
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

func ParseActivityExceptionDate(value string) (calendar.Date, error) {
	parsed, err := calendar.ParseDate(value)
	if err != nil {
		return "", fmt.Errorf("invalid calendar date %q", value)
	}
	return parsed, nil
}

func ParseActivityInstanceDate(value string) (calendar.Date, error) {
	parsed, err := calendar.ParseDate(value)
	if err != nil {
		return "", fmt.Errorf("invalid calendar date %q", value)
	}
	return parsed, nil
}

func InstanceStudentWallClock(at time.Time) string {
	return at.In(calendar.Berlin).Format("15:04:05")
}

func WrapDatabaseError(operation string, err error) error {
	return &domain.StoreError{Op: operation, Err: err}
}

func WrapNotFoundDatabaseError(operation string) error {
	return WrapDatabaseError(operation, domain.ErrRecordNotFound)
}

func InstanceStudentDay(at time.Time) string {
	return calendar.DateFromTime(at).String()
}
