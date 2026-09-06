package schedule

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/schedule"
)

// Class-wide arrival day exceptions (#2962): one date on which a whole class
// arrives at a different time than its weekly Unterrichtsschluss says. The
// write side lives here; the read side is the arrival projection
// (arrival_baseline_service.go), which folds the exception into the class
// time of that date so every reader picks it up without its own lookup.

// ClassArrivalExceptionInput is what a person enters for one class and date.
type ClassArrivalExceptionInput struct {
	SchoolClass string
	Date        timezone.Date
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

const (
	opListClassArrivalExceptions  = "list class arrival exceptions"
	opUpsertClassArrivalException = "upsert class arrival exception"
	opDeleteClassArrivalException = "delete class arrival exception"
)

// ListClassArrivalExceptions returns the exceptions of one class with
// from <= date <= to, ordered by date.
func (s *arrivalScheduleService) ListClassArrivalExceptions(
	ctx context.Context,
	schoolClass string,
	from, to timezone.Date,
) ([]*schedule.ClassArrivalException, error) {
	if s.classExceptions == nil {
		return nil, &ScheduleError{Op: opListClassArrivalExceptions, Err: ErrClassArrivalExceptionNotConfigured}
	}
	class := strings.TrimSpace(schoolClass)
	if class == "" {
		return []*schedule.ClassArrivalException{}, nil
	}
	rows, err := s.classExceptions.FindByClassesAndDateRange(ctx, []string{class}, schedule.Date(from), schedule.Date(to))
	if err != nil {
		return nil, &ScheduleError{Op: opListClassArrivalExceptions, Err: err}
	}
	return rows, nil
}

// UpsertClassArrivalException stores the exception of one class and date,
// replacing an existing one. The date must be today or later and the class
// must have at least one active child.
func (s *arrivalScheduleService) UpsertClassArrivalException(
	ctx context.Context,
	input ClassArrivalExceptionInput,
	createdBy int64,
) (*schedule.ClassArrivalException, error) {
	if s.classExceptions == nil {
		return nil, &ScheduleError{Op: opUpsertClassArrivalException, Err: ErrClassArrivalExceptionNotConfigured}
	}
	class := strings.TrimSpace(input.SchoolClass)
	if input.Date.Before(timezone.TodayDate()) {
		return nil, &ScheduleError{Op: opUpsertClassArrivalException, Err: ErrClassArrivalExceptionPastDate}
	}
	if isWeekend(input.Date) {
		return nil, &ScheduleError{Op: opUpsertClassArrivalException, Err: ErrClassArrivalExceptionWeekend}
	}
	if err := s.requireActiveClass(ctx, class, opUpsertClassArrivalException); err != nil {
		return nil, err
	}

	row := &schedule.ClassArrivalException{
		SchoolClass: class,
		Date:        schedule.Date(input.Date),
		ArrivalTime: timezone.NormalizeWallClock(input.ArrivalTime),
		Reason:      trimmedOptionalReason(input.Reason),
		Origin:      cmp.Or(input.Origin, schedule.ClassArrivalExceptionOriginOGS),
	}
	if createdBy > 0 {
		row.CreatedBy = &createdBy
	}
	if err := row.Validate(); err != nil {
		return nil, &ScheduleError{Op: opUpsertClassArrivalException, Err: err}
	}
	if err := s.classExceptions.Upsert(ctx, row); err != nil {
		return nil, &ScheduleError{Op: opUpsertClassArrivalException, Err: err}
	}
	return row, nil
}

// DeleteClassArrivalException removes the exception of one class and date.
// Past dates stay as they are: the day already happened.
func (s *arrivalScheduleService) DeleteClassArrivalException(
	ctx context.Context,
	schoolClass string,
	date timezone.Date,
) error {
	if s.classExceptions == nil {
		return &ScheduleError{Op: opDeleteClassArrivalException, Err: ErrClassArrivalExceptionNotConfigured}
	}
	if date.Before(timezone.TodayDate()) {
		return &ScheduleError{Op: opDeleteClassArrivalException, Err: ErrClassArrivalExceptionPastDate}
	}
	deleted, err := s.classExceptions.DeleteByClassAndDate(ctx, strings.TrimSpace(schoolClass), schedule.Date(date))
	if err != nil {
		return &ScheduleError{Op: opDeleteClassArrivalException, Err: err}
	}
	if !deleted {
		return &ScheduleError{Op: opDeleteClassArrivalException, Err: ErrClassArrivalExceptionNotFound}
	}
	return nil
}

// requireActiveClass refuses a class nobody is in: a typo would otherwise
// store an exception that never shows up anywhere.
func (s *arrivalScheduleService) requireActiveClass(ctx context.Context, class, op string) error {
	if class == "" {
		return &ScheduleError{Op: op, Err: ErrClassArrivalExceptionClassNotFound}
	}
	students, err := s.studentRepo.FindBySchoolClass(ctx, class)
	if err != nil {
		return &ScheduleError{Op: op, Err: fmt.Errorf("failed to find students for school class %s: %w", class, err)}
	}
	if len(dropDepartedStudents(students)) == 0 {
		return &ScheduleError{Op: op, Err: ErrClassArrivalExceptionClassNotFound}
	}
	return nil
}

func trimmedOptionalReason(reason *string) *string {
	if reason == nil {
		return nil
	}
	value := strings.TrimSpace(*reason)
	if value == "" {
		return nil
	}
	return &value
}

func isWeekend(date timezone.Date) bool {
	weekday := date.Weekday()
	return weekday == time.Saturday || weekday == time.Sunday
}

// attachClassException copies the class-wide day exception behind a projected
// row onto the effective time (#2962). A per-child day exception overrides
// the class one, so nothing is attached then.
func attachClassException(effective *EffectiveArrivalTime, row *schedule.StudentArrivalSchedule) {
	if effective == nil || row == nil || effective.IsException || effective.ArrivalTime == nil {
		return
	}
	if row.Source != schedule.ArrivalScheduleSourceClassException {
		return
	}
	effective.ClassException = &ClassArrivalExceptionInfo{
		SchoolClass: row.SourceClass,
		ArrivalTime: effective.ArrivalTime.Format("15:04"),
		Label:       row.SourceLabel,
	}
}
