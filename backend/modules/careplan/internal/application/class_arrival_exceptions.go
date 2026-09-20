package application

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/ports"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

type ClassArrivalExceptions struct {
	store    ports.ClassArrivalExceptionStore
	students ports.ClassArrivalStudents
}

// A missing store preserves the optional endpoint's explicit not-configured error.
func NewClassArrivalExceptions(store ports.ClassArrivalExceptionStore, students ports.ClassArrivalStudents) *ClassArrivalExceptions {
	if store != nil && students == nil {
		panic("care plan: class arrival student directory is required")
	}
	return &ClassArrivalExceptions{store: store, students: students}
}

var _ careplan.ClassArrivalExceptions = (*ClassArrivalExceptions)(nil)

const (
	opListClassArrivalExceptions  = "list class arrival exceptions"
	opUpsertClassArrivalException = "upsert class arrival exception"
	opDeleteClassArrivalException = "delete class arrival exception"
)

// ListClassArrivalExceptions returns the exceptions of one class with
// from <= date <= to, ordered by date.
func (s *ClassArrivalExceptions) ListClassArrivalExceptions(
	ctx context.Context,
	schoolClass string,
	from, to calendar.Date,
) ([]*careplan.ClassArrivalException, error) {
	if s.store == nil {
		return nil, &careplan.ScheduleError{Op: opListClassArrivalExceptions, Err: careplan.ErrClassArrivalExceptionNotConfigured}
	}
	class := strings.TrimSpace(schoolClass)
	if class == "" {
		return []*careplan.ClassArrivalException{}, nil
	}
	rows, err := s.store.List(ctx, []string{class}, calendar.Date(from), calendar.Date(to))
	if err != nil {
		return nil, &careplan.ScheduleError{Op: opListClassArrivalExceptions, Err: err}
	}
	return rows, nil
}

// UpsertClassArrivalException stores the exception of one class and date,
// replacing an existing one. The date must be today or later and the class
// must have at least one active child.
func (s *ClassArrivalExceptions) UpsertClassArrivalException(
	ctx context.Context,
	input careplan.ClassArrivalExceptionInput,
	createdBy int64,
) (*careplan.ClassArrivalException, error) {
	if s.store == nil {
		return nil, &careplan.ScheduleError{Op: opUpsertClassArrivalException, Err: careplan.ErrClassArrivalExceptionNotConfigured}
	}
	class := strings.TrimSpace(input.SchoolClass)
	if input.Date.Before(calendar.TodayDate()) {
		return nil, &careplan.ScheduleError{Op: opUpsertClassArrivalException, Err: careplan.ErrClassArrivalExceptionPastDate}
	}
	if isWeekend(input.Date) {
		return nil, &careplan.ScheduleError{Op: opUpsertClassArrivalException, Err: careplan.ErrClassArrivalExceptionWeekend}
	}
	if err := s.requireActiveClass(ctx, class, opUpsertClassArrivalException); err != nil {
		return nil, err
	}

	origin := input.Origin
	if origin == "" {
		origin = careplan.ClassArrivalExceptionOriginOGS
	}
	row := &careplan.ClassArrivalException{
		SchoolClass: class,
		Date:        calendar.Date(input.Date),
		ArrivalTime: calendar.NormalizeWallClock(input.ArrivalTime),
		Reason:      trimmedOptionalReason(input.Reason),
		Origin:      origin,
	}
	if createdBy > 0 {
		row.CreatedBy = &createdBy
	}
	if err := row.Validate(); err != nil {
		return nil, &careplan.ScheduleError{Op: opUpsertClassArrivalException, Err: err}
	}
	if err := s.store.Upsert(ctx, row); err != nil {
		return nil, &careplan.ScheduleError{Op: opUpsertClassArrivalException, Err: err}
	}
	return row, nil
}

// DeleteClassArrivalException removes the exception of one class and date.
// Past dates stay as they are: the day already happened.
func (s *ClassArrivalExceptions) DeleteClassArrivalException(
	ctx context.Context,
	schoolClass string,
	date calendar.Date,
) error {
	if s.store == nil {
		return &careplan.ScheduleError{Op: opDeleteClassArrivalException, Err: careplan.ErrClassArrivalExceptionNotConfigured}
	}
	if date.Before(calendar.TodayDate()) {
		return &careplan.ScheduleError{Op: opDeleteClassArrivalException, Err: careplan.ErrClassArrivalExceptionPastDate}
	}
	deleted, err := s.store.Delete(ctx, strings.TrimSpace(schoolClass), calendar.Date(date))
	if err != nil {
		return &careplan.ScheduleError{Op: opDeleteClassArrivalException, Err: err}
	}
	if !deleted {
		return &careplan.ScheduleError{Op: opDeleteClassArrivalException, Err: careplan.ErrClassArrivalExceptionNotFound}
	}
	return nil
}

// requireActiveClass refuses a class nobody is in: a typo would otherwise
// store an exception that never shows up anywhere.
func (s *ClassArrivalExceptions) requireActiveClass(ctx context.Context, class, op string) error {
	if class == "" {
		return &careplan.ScheduleError{Op: op, Err: careplan.ErrClassArrivalExceptionClassNotFound}
	}
	active, err := s.students.HasActiveClass(ctx, class, calendar.TodayDate())
	if err != nil {
		return &careplan.ScheduleError{Op: op, Err: fmt.Errorf("failed to find students for school class %s: %w", class, err)}
	}
	if !active {
		return &careplan.ScheduleError{Op: op, Err: careplan.ErrClassArrivalExceptionClassNotFound}
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

func isWeekend(date calendar.Date) bool {
	weekday := date.Weekday()
	return weekday == time.Saturday || weekday == time.Sunday
}
