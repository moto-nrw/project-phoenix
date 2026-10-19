package application

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar/internal/ports"
)

// Service joins the caller's transaction or opens a school unit of work for
// record operations. Pure holiday/render operations need no database scope.
// It records one observation per call and preserves the domain errors.
type Service struct {
	store          ports.Store
	observe        ports.Observer
	administration Administration
}

// New composes the service. The administrative write path needs a
// transaction runner and a clock; the gate, guard and federal-state ports
// are optional collaborators of the composition.
func New(store ports.Store, observe ports.Observer, administration Administration) *Service {
	if store == nil || observe == nil || administration.Transaction == nil || administration.Today == nil {
		panic("school calendar application: all dependencies are required")
	}
	return &Service{store: store, observe: observe, administration: administration}
}

// --- calendar periods ---

func (s *Service) FindCalendarPeriod(ctx context.Context, id int64) (result domain.CalendarPeriod, err error) {
	err = s.run(ctx, "find_calendar_period", func(ctx context.Context, stats *domain.OperationStats) error {
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
	err = s.run(ctx, "list_calendar_periods", func(ctx context.Context, stats *domain.OperationStats) error {
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
	err = s.run(ctx, operation, func(ctx context.Context, stats *domain.OperationStats) error {
		var createStats domain.OperationStats
		result, created, createStats, err = s.store.CreateCalendarPeriod(ctx, fields, ifAbsent)
		stats.Add(createStats)
		return err
	})
	return result, created, err
}

func (s *Service) UpdateCalendarPeriod(ctx context.Context, id int64, fields domain.CalendarPeriodFields) (result domain.CalendarPeriod, err error) {
	err = s.run(ctx, "update_calendar_period", func(ctx context.Context, stats *domain.OperationStats) error {
		var updateStats domain.OperationStats
		result, updateStats, err = s.store.UpdateCalendarPeriod(ctx, id, fields)
		stats.Add(updateStats)
		return err
	})
	return result, err
}

func (s *Service) DeleteCalendarPeriod(ctx context.Context, id int64) error {
	return s.run(ctx, "delete_calendar_period", func(ctx context.Context, stats *domain.OperationStats) error {
		deleteStats, err := s.store.DeleteCalendarPeriod(ctx, id)
		stats.Add(deleteStats)
		return err
	})
}

// --- closing days ---

func (s *Service) FindClosingDay(ctx context.Context, id int64) (result domain.ClosingDay, err error) {
	err = s.run(ctx, "find_closing_day", func(ctx context.Context, stats *domain.OperationStats) error {
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
	err = s.run(ctx, "list_closing_days", func(ctx context.Context, stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		result, queryStats, err = s.store.ListClosingDays(ctx, filter)
		stats.Add(queryStats)
		return err
	})
	return result, err
}

func (s *Service) CreateClosingDay(ctx context.Context, fields domain.ClosingDayFields) (result domain.ClosingDay, err error) {
	err = s.run(ctx, "create_closing_day", func(ctx context.Context, stats *domain.OperationStats) error {
		var createStats domain.OperationStats
		result, createStats, err = s.store.CreateClosingDay(ctx, fields)
		stats.Add(createStats)
		return err
	})
	return result, err
}

func (s *Service) UpdateClosingDay(ctx context.Context, id int64, fields domain.ClosingDayFields) (result domain.ClosingDay, err error) {
	err = s.run(ctx, "update_closing_day", func(ctx context.Context, stats *domain.OperationStats) error {
		var updateStats domain.OperationStats
		result, updateStats, err = s.store.UpdateClosingDay(ctx, id, fields)
		stats.Add(updateStats)
		return err
	})
	return result, err
}

func (s *Service) DeleteClosingDay(ctx context.Context, id int64) error {
	return s.run(ctx, "delete_closing_day", func(ctx context.Context, stats *domain.OperationStats) error {
		deleteStats, err := s.store.DeleteClosingDay(ctx, id)
		stats.Add(deleteStats)
		return err
	})
}

// --- dateframes ---

func (s *Service) FindDateframe(ctx context.Context, id int64) (result domain.Dateframe, err error) {
	err = s.run(ctx, "find_dateframe", func(ctx context.Context, stats *domain.OperationStats) error {
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
	err = s.run(ctx, "list_dateframes", func(ctx context.Context, stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		result, queryStats, err = s.store.ListDateframes(ctx, filter)
		stats.Add(queryStats)
		return err
	})
	return result, err
}

func (s *Service) CreateDateframe(ctx context.Context, fields domain.DateframeFields) (result domain.Dateframe, err error) {
	err = s.run(ctx, "create_dateframe", func(ctx context.Context, stats *domain.OperationStats) error {
		var createStats domain.OperationStats
		result, createStats, err = s.store.CreateDateframe(ctx, fields)
		stats.Add(createStats)
		return err
	})
	return result, err
}

func (s *Service) UpdateDateframe(ctx context.Context, id int64, fields domain.DateframeFields) (result domain.Dateframe, err error) {
	err = s.run(ctx, "update_dateframe", func(ctx context.Context, stats *domain.OperationStats) error {
		var updateStats domain.OperationStats
		result, updateStats, err = s.store.UpdateDateframe(ctx, id, fields)
		stats.Add(updateStats)
		return err
	})
	return result, err
}

func (s *Service) DeleteDateframe(ctx context.Context, id int64) error {
	return s.run(ctx, "delete_dateframe", func(ctx context.Context, stats *domain.OperationStats) error {
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
	err = s.observeOperation("list_holidays", func(ctx context.Context, stats *domain.OperationStats) error {
		result, err = domain.HolidaysInRange(region, from, to)
		stats.Rows = int64(len(result))
		return err
	})
	return result, err
}

func (s *Service) HolidayDates(_ context.Context, region, from, to string) (result map[string]bool, err error) {
	err = s.observeOperation("holiday_dates", func(ctx context.Context, stats *domain.OperationStats) error {
		result, err = domain.HolidayDates(region, from, to)
		stats.Rows = int64(len(result))
		return err
	})
	return result, err
}

func (s *Service) RenderCalendar(_ context.Context, name string, events []domain.CalendarEvent) (result string, err error) {
	err = s.observeOperation("render_calendar", func(ctx context.Context, stats *domain.OperationStats) error {
		result = domain.RenderCalendar(name, events)
		stats.Rows = int64(len(events))
		return nil
	})
	return result, err
}

func (s *Service) RenderCalendarObject(_ context.Context, event domain.CalendarEvent) (result string, err error) {
	err = s.observeOperation("render_calendar_object", func(ctx context.Context, stats *domain.OperationStats) error {
		result = domain.RenderCalendarObject(event)
		stats.Rows = 1
		return nil
	})
	return result, err
}

// --- plumbing ---

func (s *Service) run(ctx context.Context, operation string, fn func(context.Context, *domain.OperationStats) error) (err error) {
	return s.administration.Transaction(ctx, func(txCtx context.Context) error {
		return s.observeOperation(operation, func(_ context.Context, stats *domain.OperationStats) error { return fn(txCtx, stats) })
	})
}

func (s *Service) observeOperation(operation string, fn func(context.Context, *domain.OperationStats) error) (err error) {
	started := time.Now()
	stats := domain.OperationStats{}
	defer func() {
		s.observe(ports.Observation{Operation: operation, Duration: time.Since(started), Stats: stats, Err: err})
	}()
	err = fn(context.Background(), &stats)
	if err != nil {
		stats.Rows = 0
	}
	return err
}
