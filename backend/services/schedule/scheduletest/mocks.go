// Package scheduletest provides shared test doubles for schedule services.
package scheduletest

import (
	"context"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	scheduleService "github.com/moto-nrw/project-phoenix/services/schedule"
)

// ClosingDayServiceMock is a func-field test double for
// schedule.ClosingDayService. Nil functions return zero values.
type ClosingDayServiceMock struct {
	GetAllFn             func(ctx context.Context) ([]*scheduleModel.ClosingDay, error)
	GetByIDFn            func(ctx context.Context, id int64) (*scheduleModel.ClosingDay, error)
	CreateFn             func(ctx context.Context, day *scheduleModel.ClosingDay) error
	UpdateFn             func(ctx context.Context, day *scheduleModel.ClosingDay) error
	DeleteFn             func(ctx context.Context, id int64) error
	ClosingDaysInRangeFn func(ctx context.Context, from, to timezone.Date) ([]*scheduleModel.ClosingDay, error)
	ClosingDayDatesFn    func(ctx context.Context, from, to timezone.Date) (map[timezone.Date]bool, error)
}

var _ scheduleService.ClosingDayService = (*ClosingDayServiceMock)(nil)

func (m *ClosingDayServiceMock) GetAll(ctx context.Context) ([]*scheduleModel.ClosingDay, error) {
	if m.GetAllFn != nil {
		return m.GetAllFn(ctx)
	}
	return nil, nil
}

func (m *ClosingDayServiceMock) GetByID(ctx context.Context, id int64) (*scheduleModel.ClosingDay, error) {
	if m.GetByIDFn != nil {
		return m.GetByIDFn(ctx, id)
	}
	return nil, nil
}

func (m *ClosingDayServiceMock) Create(ctx context.Context, day *scheduleModel.ClosingDay) error {
	if m.CreateFn != nil {
		return m.CreateFn(ctx, day)
	}
	return nil
}

func (m *ClosingDayServiceMock) Update(ctx context.Context, day *scheduleModel.ClosingDay) error {
	if m.UpdateFn != nil {
		return m.UpdateFn(ctx, day)
	}
	return nil
}

func (m *ClosingDayServiceMock) Delete(ctx context.Context, id int64) error {
	if m.DeleteFn != nil {
		return m.DeleteFn(ctx, id)
	}
	return nil
}

func (m *ClosingDayServiceMock) ClosingDaysInRange(ctx context.Context, from, to timezone.Date) ([]*scheduleModel.ClosingDay, error) {
	if m.ClosingDaysInRangeFn != nil {
		return m.ClosingDaysInRangeFn(ctx, from, to)
	}
	return nil, nil
}

func (m *ClosingDayServiceMock) ClosingDayDates(ctx context.Context, from, to timezone.Date) (map[timezone.Date]bool, error) {
	if m.ClosingDayDatesFn != nil {
		return m.ClosingDayDatesFn(ctx, from, to)
	}
	return nil, nil
}
