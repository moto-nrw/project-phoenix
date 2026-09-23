package timetable

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar"
)

// ClosingDaysMock is a func-field test double for the ClosingDays port.
// Nil functions return zero values.
type ClosingDaysMock struct {
	ListFn   func(ctx context.Context, filter schoolcalendar.ClosingDayFilter) ([]schoolcalendar.ClosingDay, error)
	FindFn   func(ctx context.Context, id int64) (schoolcalendar.ClosingDay, error)
	CreateFn func(ctx context.Context, input schoolcalendar.CreateClosingDay) (schoolcalendar.ClosingDay, error)
	UpdateFn func(ctx context.Context, input schoolcalendar.UpdateClosingDay) (schoolcalendar.ClosingDay, error)
	DeleteFn func(ctx context.Context, id int64) error
}

var _ ClosingDays = (*ClosingDaysMock)(nil)

func (m *ClosingDaysMock) ListClosingDays(ctx context.Context, filter schoolcalendar.ClosingDayFilter) ([]schoolcalendar.ClosingDay, error) {
	if m.ListFn != nil {
		return m.ListFn(ctx, filter)
	}
	return nil, nil
}

func (m *ClosingDaysMock) FindClosingDay(ctx context.Context, id int64) (schoolcalendar.ClosingDay, error) {
	if m.FindFn != nil {
		return m.FindFn(ctx, id)
	}
	return schoolcalendar.ClosingDay{}, nil
}

func (m *ClosingDaysMock) CreateClosingDay(ctx context.Context, input schoolcalendar.CreateClosingDay) (schoolcalendar.ClosingDay, error) {
	if m.CreateFn != nil {
		return m.CreateFn(ctx, input)
	}
	return schoolcalendar.ClosingDay{ID: 1, StartDate: input.StartDate, EndDate: input.EndDate, Reason: input.Reason}, nil
}

func (m *ClosingDaysMock) UpdateClosingDay(ctx context.Context, input schoolcalendar.UpdateClosingDay) (schoolcalendar.ClosingDay, error) {
	if m.UpdateFn != nil {
		return m.UpdateFn(ctx, input)
	}
	return schoolcalendar.ClosingDay{ID: input.ID, StartDate: input.StartDate, EndDate: input.EndDate, Reason: input.Reason}, nil
}

func (m *ClosingDaysMock) DeleteClosingDay(ctx context.Context, id int64) error {
	if m.DeleteFn != nil {
		return m.DeleteFn(ctx, id)
	}
	return nil
}
