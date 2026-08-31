package compose

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/moto-nrw/project-phoenix/modules/mealplan"
	"github.com/moto-nrw/project-phoenix/modules/mealplan/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/modules/mealplan/internal/application"
	"github.com/moto-nrw/project-phoenix/modules/mealplan/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/mealplan/internal/ports"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

type SettingsFunc func(context.Context) (bool, error)

func (fn SettingsFunc) MealPlanEnabled(ctx context.Context) (bool, error) { return fn(ctx) }

type TransactionFunc func(context.Context, func(context.Context) error) error

func (fn TransactionFunc) Run(ctx context.Context, callback func(context.Context) error) error {
	return fn(ctx, callback)
}

type Observation = ports.Observation

type Dependencies struct {
	DB       *bun.DB
	Settings interface {
		MealPlanEnabled(context.Context) (bool, error)
	}
	Observe func(Observation)
}

func New(dependencies Dependencies) (*mealplan.Module, error) {
	if dependencies.DB == nil || dependencies.Settings == nil || dependencies.Observe == nil {
		return nil, errors.New("meal plan compose: all dependencies are required")
	}
	store := postgres.New(func(ctx context.Context) (bun.IDB, int64, error) {
		transaction, ok := tenant.TransactionFromContext(ctx)
		if !ok {
			return nil, 0, errors.New("meal plan postgres: transaction is required")
		}
		tx, ok := transaction.(bun.Tx)
		if !ok {
			return nil, 0, fmt.Errorf("meal plan postgres: unsupported transaction %T", transaction)
		}
		tenantID, err := tenant.TenantFromContext(ctx)
		if err != nil {
			return nil, 0, err
		}
		return tx, tenantID.Int64(), nil
	})
	service := application.New(store, dependencies.Settings, TransactionFunc(tenant.NewTransactionRunner().RunInTx), dependencies.Observe)
	return mealplan.NewModule(engine{service: service}), nil
}

type Settings struct {
	mu      sync.RWMutex
	resolve SettingsFunc
}

func NewSettings() *Settings { return &Settings{} }

func (s *Settings) Bind(resolve func(context.Context) (bool, error)) {
	if resolve == nil {
		panic("meal plan settings: resolver is required")
	}
	s.mu.Lock()
	if s.resolve != nil {
		s.mu.Unlock()
		panic("meal plan settings: resolver is already bound")
	}
	s.resolve = SettingsFunc(resolve)
	s.mu.Unlock()
}

func (s *Settings) MealPlanEnabled(ctx context.Context) (bool, error) {
	s.mu.RLock()
	resolve := s.resolve
	s.mu.RUnlock()
	if resolve == nil {
		return false, errors.New("meal plan settings: resolver is not bound")
	}
	return resolve(ctx)
}

type engine struct{ service *application.Service }

func (e engine) Available(ctx context.Context) (bool, error) { return e.service.Available(ctx) }

func (e engine) Week(ctx context.Context, monday string) ([]mealplan.Entry, error) {
	parsed, err := domain.ParseDate(monday)
	if err != nil {
		return nil, err
	}
	entries, err := e.service.Week(ctx, parsed)
	if err != nil {
		return nil, mapError(err)
	}
	result := make([]mealplan.Entry, 0, len(entries))
	for _, entry := range entries {
		result = append(result, mealplan.Entry{Date: mealplan.Date(entry.Date.String()), Position: entry.Position, Dish: entry.Dish, Note: entry.Note})
	}
	return result, nil
}

func (e engine) Replace(ctx context.Context, date string, dishes []mealplan.Dish) error {
	parsed, err := domain.ParseDate(date)
	if err != nil {
		return err
	}
	values := make([]domain.Dish, 0, len(dishes))
	for _, dish := range dishes {
		values = append(values, domain.Dish{Dish: dish.Dish, Note: dish.Note})
	}
	return mapError(e.service.Replace(ctx, parsed, values))
}

func (e engine) Clear(ctx context.Context, date string) error {
	parsed, err := domain.ParseDate(date)
	if err != nil {
		return err
	}
	return mapError(e.service.Clear(ctx, parsed))
}

func mapError(err error) error {
	if errors.Is(err, application.ErrDisabled) {
		return mealplan.ErrDisabled
	}
	return err
}
