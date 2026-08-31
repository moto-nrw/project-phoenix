package application

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/mealplan/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/mealplan/internal/ports"
)

var ErrDisabled = errors.New("meal plan is disabled")

type Service struct {
	store    ports.Store
	settings ports.Settings
	tx       ports.Transaction
	observe  ports.Observer
}

func New(store ports.Store, settings ports.Settings, tx ports.Transaction, observe ports.Observer) *Service {
	if store == nil || settings == nil || tx == nil || observe == nil {
		panic("meal plan application: all dependencies are required")
	}
	return &Service{store: store, settings: settings, tx: tx, observe: observe}
}

func (s *Service) Available(ctx context.Context) (available bool, err error) {
	err = s.run(ctx, "availability", func(txCtx context.Context) (int64, int64, time.Duration, error) {
		available, err = s.settings.MealPlanEnabled(txCtx)
		return 0, 0, 0, err
	})
	return available, err
}

func (s *Service) Week(ctx context.Context, monday domain.Date) (entries []domain.Entry, err error) {
	err = s.run(ctx, "read_week", func(txCtx context.Context) (int64, int64, time.Duration, error) {
		if err := s.requireEnabled(txCtx); err != nil {
			return 0, 0, 0, err
		}
		var queries int64
		entries, queries, err = s.store.FindWeek(txCtx, monday, monday.AddDays(4))
		return 0, queries, 0, err
	})
	return entries, err
}

func (s *Service) Replace(ctx context.Context, date domain.Date, dishes []domain.Dish) error {
	return s.run(ctx, "replace_day", func(txCtx context.Context) (int64, int64, time.Duration, error) {
		if err := s.requireEnabled(txCtx); err != nil {
			return 0, 0, 0, err
		}
		return s.store.ReplaceDay(txCtx, date, dishes)
	})
}

func (s *Service) Clear(ctx context.Context, date domain.Date) error {
	return s.run(ctx, "clear_day", func(txCtx context.Context) (int64, int64, time.Duration, error) {
		if err := s.requireEnabled(txCtx); err != nil {
			return 0, 0, 0, err
		}
		return s.store.ClearDay(txCtx, date)
	})
}

func (s *Service) requireEnabled(ctx context.Context) error {
	enabled, err := s.settings.MealPlanEnabled(ctx)
	if err != nil {
		return fmt.Errorf("meal plan: resolve availability: %w", err)
	}
	if !enabled {
		return ErrDisabled
	}
	return nil
}

func (s *Service) run(ctx context.Context, operation string, fn func(context.Context) (int64, int64, time.Duration, error)) (err error) {
	started := time.Now()
	var rows, queries int64
	var lockWait time.Duration
	defer func() {
		s.observe(ports.Observation{Operation: operation, Duration: time.Since(started), Queries: queries, Rows: rows, LockWait: lockWait, Err: err})
	}()
	err = s.tx.Run(ctx, func(txCtx context.Context) error {
		rows, queries, lockWait, err = fn(txCtx)
		return err
	})
	if err != nil {
		rows = 0
	}
	return err
}
