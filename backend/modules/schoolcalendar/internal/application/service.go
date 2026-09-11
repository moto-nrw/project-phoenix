package application

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar/internal/ports"
)

// Service runs every calendar operation on the caller's ambient transaction
// (or the shared connection when there is none, exactly like the legacy
// repositories), records one observation per call and turns "no row"
// outcomes into the stable domain errors.
type Service struct {
	store   ports.Store
	observe ports.Observer
}

func New(store ports.Store, observe ports.Observer) *Service {
	if store == nil || observe == nil {
		panic("school calendar application: all dependencies are required")
	}
	return &Service{store: store, observe: observe}
}

// --- calendar periods ---

func (s *Service) FindCalendarPeriod(ctx context.Context, id int64) (result domain.CalendarPeriod, err error) {
	err = s.run("find_calendar_period", func(stats *domain.OperationStats) error {
		var found bool
		var queryStats domain.OperationStats
		result, found, queryStats, err = s.store.FindCalendarPeriod(ctx, id)
		stats.Add(queryStats)
		if err == nil && !found {
			return domain.ErrCalendarPeriodNotFound
		}
		return err
	})
	return result, err
}

func (s *Service) ListCalendarPeriods(ctx context.Context, filter domain.CalendarPeriodFilter) (result []domain.CalendarPeriod, err error) {
	err = s.run("list_calendar_periods", func(stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		result, queryStats, err = s.store.ListCalendarPeriods(ctx, filter)
		stats.Add(queryStats)
		return err
	})
	return result, err
}

func (s *Service) CreateCalendarPeriod(ctx context.Context, fields domain.CalendarPeriodFields, ifAbsent bool) (result domain.CalendarPeriod, created bool, err error) {
	operation := "create_calendar_period"
	if ifAbsent {
		operation = "create_calendar_period_if_absent"
	}
	err = s.run(operation, func(stats *domain.OperationStats) error {
		var createStats domain.OperationStats
		result, created, createStats, err = s.store.CreateCalendarPeriod(ctx, fields, ifAbsent)
		stats.Add(createStats)
		return err
	})
	return result, created, err
}

func (s *Service) UpdateCalendarPeriod(ctx context.Context, id int64, fields domain.CalendarPeriodFields) (result domain.CalendarPeriod, err error) {
	err = s.run("update_calendar_period", func(stats *domain.OperationStats) error {
		var updateStats domain.OperationStats
		result, updateStats, err = s.store.UpdateCalendarPeriod(ctx, id, fields)
		stats.Add(updateStats)
		return err
	})
	return result, err
}

func (s *Service) DeleteCalendarPeriod(ctx context.Context, id int64) error {
	return s.run("delete_calendar_period", func(stats *domain.OperationStats) error {
		deleteStats, err := s.store.DeleteCalendarPeriod(ctx, id)
		stats.Add(deleteStats)
		return err
	})
}

// --- closing days ---

func (s *Service) FindClosingDay(ctx context.Context, id int64) (result domain.ClosingDay, err error) {
	err = s.run("find_closing_day", func(stats *domain.OperationStats) error {
		var found bool
		var queryStats domain.OperationStats
		result, found, queryStats, err = s.store.FindClosingDay(ctx, id)
		stats.Add(queryStats)
		if err == nil && !found {
			return domain.ErrClosingDayNotFound
		}
		return err
	})
	return result, err
}

func (s *Service) ListClosingDays(ctx context.Context, filter domain.ClosingDayFilter) (result []domain.ClosingDay, err error) {
	err = s.run("list_closing_days", func(stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		result, queryStats, err = s.store.ListClosingDays(ctx, filter)
		stats.Add(queryStats)
		return err
	})
	return result, err
}

func (s *Service) CreateClosingDay(ctx context.Context, fields domain.ClosingDayFields) (result domain.ClosingDay, err error) {
	err = s.run("create_closing_day", func(stats *domain.OperationStats) error {
		var createStats domain.OperationStats
		result, createStats, err = s.store.CreateClosingDay(ctx, fields)
		stats.Add(createStats)
		return err
	})
	return result, err
}

func (s *Service) UpdateClosingDay(ctx context.Context, id int64, fields domain.ClosingDayFields) (result domain.ClosingDay, err error) {
	err = s.run("update_closing_day", func(stats *domain.OperationStats) error {
		var updateStats domain.OperationStats
		result, updateStats, err = s.store.UpdateClosingDay(ctx, id, fields)
		stats.Add(updateStats)
		return err
	})
	return result, err
}

func (s *Service) DeleteClosingDay(ctx context.Context, id int64) error {
	return s.run("delete_closing_day", func(stats *domain.OperationStats) error {
		deleteStats, err := s.store.DeleteClosingDay(ctx, id)
		stats.Add(deleteStats)
		return err
	})
}

// --- dateframes ---

func (s *Service) FindDateframe(ctx context.Context, id int64) (result domain.Dateframe, err error) {
	err = s.run("find_dateframe", func(stats *domain.OperationStats) error {
		var found bool
		var queryStats domain.OperationStats
		result, found, queryStats, err = s.store.FindDateframe(ctx, id)
		stats.Add(queryStats)
		if err == nil && !found {
			return domain.ErrDateframeNotFound
		}
		return err
	})
	return result, err
}

func (s *Service) ListDateframes(ctx context.Context, filter domain.DateframeFilter) (result []domain.Dateframe, err error) {
	err = s.run("list_dateframes", func(stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		result, queryStats, err = s.store.ListDateframes(ctx, filter)
		stats.Add(queryStats)
		return err
	})
	return result, err
}

func (s *Service) CreateDateframe(ctx context.Context, fields domain.DateframeFields) (result domain.Dateframe, err error) {
	err = s.run("create_dateframe", func(stats *domain.OperationStats) error {
		var createStats domain.OperationStats
		result, createStats, err = s.store.CreateDateframe(ctx, fields)
		stats.Add(createStats)
		return err
	})
	return result, err
}

func (s *Service) UpdateDateframe(ctx context.Context, id int64, fields domain.DateframeFields) (result domain.Dateframe, err error) {
	err = s.run("update_dateframe", func(stats *domain.OperationStats) error {
		var updateStats domain.OperationStats
		result, updateStats, err = s.store.UpdateDateframe(ctx, id, fields)
		stats.Add(updateStats)
		return err
	})
	return result, err
}

func (s *Service) DeleteDateframe(ctx context.Context, id int64) error {
	return s.run("delete_dateframe", func(stats *domain.OperationStats) error {
		deleteStats, err := s.store.DeleteDateframe(ctx, id)
		stats.Add(deleteStats)
		return err
	})
}

// --- holidays and calendar documents ---

func (s *Service) ValidHolidayRegion(region string) bool {
	return domain.ValidHolidayRegion(region)
}

func (s *Service) ListHolidays(_ context.Context, region, from, to string) (result []domain.Holiday, err error) {
	err = s.run("list_holidays", func(stats *domain.OperationStats) error {
		result, err = domain.HolidaysInRange(region, from, to)
		stats.Rows = int64(len(result))
		return err
	})
	return result, err
}

func (s *Service) HolidayDates(_ context.Context, region, from, to string) (result map[string]bool, err error) {
	err = s.run("holiday_dates", func(stats *domain.OperationStats) error {
		result, err = domain.HolidayDates(region, from, to)
		stats.Rows = int64(len(result))
		return err
	})
	return result, err
}

func (s *Service) RenderCalendar(_ context.Context, name string, events []domain.CalendarEvent) (result string, err error) {
	err = s.run("render_calendar", func(stats *domain.OperationStats) error {
		result = domain.RenderCalendar(name, events)
		stats.Rows = int64(len(events))
		return nil
	})
	return result, err
}

func (s *Service) RenderCalendarObject(_ context.Context, event domain.CalendarEvent) (result string, err error) {
	err = s.run("render_calendar_object", func(stats *domain.OperationStats) error {
		result = domain.RenderCalendarObject(event)
		stats.Rows = 1
		return nil
	})
	return result, err
}

// --- plumbing ---

func (s *Service) run(operation string, fn func(*domain.OperationStats) error) (err error) {
	started := time.Now()
	stats := domain.OperationStats{}
	defer func() {
		s.observe(ports.Observation{Operation: operation, Duration: time.Since(started), Stats: stats, Err: err})
	}()
	err = fn(&stats)
	if err != nil {
		stats.Rows = 0
	}
	return err
}
