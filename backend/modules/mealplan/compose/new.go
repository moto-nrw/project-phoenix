package compose

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/mealplan"
	"github.com/moto-nrw/project-phoenix/modules/mealplan/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/modules/mealplan/internal/application"
	"github.com/moto-nrw/project-phoenix/modules/mealplan/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/mealplan/internal/ports"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

type boolSettingsResolver func(context.Context) (bool, error)
type stringSettingsResolver func(context.Context) (string, error)

type TransactionFunc func(context.Context, func(context.Context) error) error

func (fn TransactionFunc) Run(ctx context.Context, callback func(context.Context) error) error {
	return fn(ctx, callback)
}

type Observation = ports.Observation

type Dependencies struct {
	DB       *bun.DB
	Settings interface {
		MealPlanEnabled(context.Context) (bool, error)
		MealRegistrationEnabled(context.Context) (bool, error)
		MealRegistrationCutoff(context.Context) (string, error)
	}
	Observe func(Observation)
	Now     func() time.Time
}

func New(dependencies Dependencies) (*mealplan.Module, error) {
	if dependencies.DB == nil || dependencies.Settings == nil || dependencies.Observe == nil || dependencies.Now == nil {
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
	service := application.New(store, dependencies.Settings, TransactionFunc(tenant.NewTransactionRunner().RunInTx), dependencies.Observe, dependencies.Now)
	return mealplan.NewModule(engine{service: service}), nil
}

type Settings struct {
	mu                      sync.RWMutex
	mealPlanEnabled         boolSettingsResolver
	mealRegistrationEnabled boolSettingsResolver
	mealRegistrationCutoff  stringSettingsResolver
}

func NewSettings() *Settings { return &Settings{} }

func (s *Settings) Bind(mealPlanEnabled, mealRegistrationEnabled func(context.Context) (bool, error), mealRegistrationCutoff func(context.Context) (string, error)) {
	if mealPlanEnabled == nil || mealRegistrationEnabled == nil || mealRegistrationCutoff == nil {
		panic("meal plan settings: resolvers are required")
	}
	s.mu.Lock()
	if s.mealPlanEnabled != nil {
		s.mu.Unlock()
		panic("meal plan settings: resolvers are already bound")
	}
	s.mealPlanEnabled = boolSettingsResolver(mealPlanEnabled)
	s.mealRegistrationEnabled = boolSettingsResolver(mealRegistrationEnabled)
	s.mealRegistrationCutoff = stringSettingsResolver(mealRegistrationCutoff)
	s.mu.Unlock()
}

func (s *Settings) MealPlanEnabled(ctx context.Context) (bool, error) {
	s.mu.RLock()
	resolve := s.mealPlanEnabled
	s.mu.RUnlock()
	if resolve == nil {
		return false, errors.New("meal plan settings: resolver is not bound")
	}
	return resolve(ctx)
}

func (s *Settings) MealRegistrationEnabled(ctx context.Context) (bool, error) {
	s.mu.RLock()
	resolve := s.mealRegistrationEnabled
	s.mu.RUnlock()
	if resolve == nil {
		return false, errors.New("meal plan settings: resolver is not bound")
	}
	return resolve(ctx)
}

func (s *Settings) MealRegistrationCutoff(ctx context.Context) (string, error) {
	s.mu.RLock()
	resolve := s.mealRegistrationCutoff
	s.mu.RUnlock()
	if resolve == nil {
		return "", errors.New("meal plan settings: resolver is not bound")
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

func (e engine) RegistrationAvailable(ctx context.Context) (bool, error) {
	available, err := e.service.RegistrationAvailable(ctx)
	return available, mapError(err)
}

func (e engine) Participation(ctx context.Context, studentID int64, from, to string) (mealplan.ParticipationPlan, error) {
	start, err := domain.ParseDate(from)
	if err != nil {
		return mealplan.ParticipationPlan{}, err
	}
	end, err := domain.ParseDate(to)
	if err != nil {
		return mealplan.ParticipationPlan{}, err
	}
	plan, err := e.service.Participation(ctx, studentID, start, end)
	if err != nil {
		return mealplan.ParticipationPlan{}, mapError(err)
	}
	return publicParticipationPlan(plan), nil
}

func (e engine) ReplaceParticipationSchedule(ctx context.Context, studentID, accountID int64, weekdays []mealplan.Weekday) (mealplan.Date, error) {
	values := make([]domain.Weekday, 0, len(weekdays))
	for _, weekday := range weekdays {
		values = append(values, domain.Weekday(weekday))
	}
	effectiveFrom, err := e.service.ReplaceParticipationSchedule(ctx, studentID, accountID, values)
	return mealplan.Date(effectiveFrom.String()), mapError(err)
}

func (e engine) SetParticipationDay(ctx context.Context, studentID, accountID int64, date string, participating bool) error {
	parsed, err := domain.ParseDate(date)
	if err != nil {
		return err
	}
	return mapError(e.service.SetParticipationDay(ctx, studentID, accountID, parsed, participating))
}

func (e engine) ClearParticipationDay(ctx context.Context, studentID, accountID int64, date string) error {
	parsed, err := domain.ParseDate(date)
	if err != nil {
		return err
	}
	return mapError(e.service.ClearParticipationDay(ctx, studentID, accountID, parsed))
}

func (e engine) DailyParticipants(ctx context.Context, date string) (mealplan.DailyList, error) {
	parsed, err := domain.ParseDate(date)
	if err != nil {
		return mealplan.DailyList{}, err
	}
	list, err := e.service.DailyParticipants(ctx, parsed)
	if err != nil {
		return mealplan.DailyList{}, mapError(err)
	}
	result := mealplan.DailyList{Date: mealplan.Date(list.Date.String()), CutoffTime: list.CutoffTime, Participants: make([]mealplan.DailyParticipant, 0, len(list.Participants))}
	for _, participant := range list.Participants {
		result.Participants = append(result.Participants, mealplan.DailyParticipant{StudentID: participant.StudentID, FirstName: participant.FirstName, LastName: participant.LastName, SchoolClass: participant.SchoolClass})
	}
	return result, nil
}

func publicParticipationPlan(plan domain.ParticipationPlan) mealplan.ParticipationPlan {
	result := mealplan.ParticipationPlan{EffectiveFrom: mealplan.Date(plan.EffectiveFrom.String()), CutoffTime: plan.CutoffTime, Weekdays: make([]mealplan.Weekday, 0, len(plan.Weekdays)), Days: make([]mealplan.ParticipationDay, 0, len(plan.Days))}
	for _, weekday := range plan.Weekdays {
		result.Weekdays = append(result.Weekdays, mealplan.Weekday(weekday))
	}
	for _, day := range plan.Days {
		result.Days = append(result.Days, mealplan.ParticipationDay{Date: mealplan.Date(day.Date.String()), Participating: day.Participating, Source: mealplan.ParticipationSource(day.Source), Changeable: day.Changeable})
	}
	return result
}

func mapError(err error) error {
	if errors.Is(err, application.ErrDisabled) {
		return mealplan.ErrDisabled
	}
	if errors.Is(err, application.ErrRegistrationDisabled) {
		return mealplan.ErrRegistrationDisabled
	}
	if errors.Is(err, application.ErrCutoffPassed) {
		return mealplan.ErrParticipationCutoff
	}
	if errors.Is(err, application.ErrInvalidCutoff) {
		return mealplan.ErrInvalidParticipation
	}
	return err
}
