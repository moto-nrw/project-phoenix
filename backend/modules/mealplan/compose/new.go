package compose

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/mealplan"
	"github.com/moto-nrw/project-phoenix/modules/mealplan/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/modules/mealplan/internal/application"
	"github.com/moto-nrw/project-phoenix/modules/mealplan/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/mealplan/internal/ports"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

type TransactionFunc func(context.Context, func(context.Context) error) error

func (fn TransactionFunc) Run(ctx context.Context, callback func(context.Context) error) error {
	return fn(ctx, callback)
}

type Observation = ports.Observation

// OrderingSettings are the school's meal ordering settings as stored.
type OrderingSettings = ports.OrderingSettings

type ParticipantCandidate struct {
	StudentID   int64
	FirstName   string
	LastName    string
	SchoolClass string
}

type ParticipantFinder func(context.Context, string) ([]ParticipantCandidate, error)

// CalendarFinder reads the school calendar for the inclusive window
// [from, to]: the closed days (closing days, public holidays) and the days of
// school holiday periods, both as YYYY-MM-DD sets.
type CalendarFinder func(ctx context.Context, from, to string) (closed, holiday map[string]bool, err error)

type Dependencies struct {
	DB       *bun.DB
	Settings interface {
		MealPlanEnabled(context.Context) (bool, error)
		MealRegistrationEnabled(context.Context) (bool, error)
		MealOrderingSettings(context.Context) (OrderingSettings, error)
	}
	Observe      func(Observation)
	Now          func() time.Time
	Participants ParticipantFinder
	Calendar     CalendarFinder
}

func New(dependencies Dependencies) (*mealplan.Module, error) {
	if dependencies.DB == nil || dependencies.Settings == nil || dependencies.Observe == nil || dependencies.Now == nil || dependencies.Participants == nil || dependencies.Calendar == nil {
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
	service := application.New(store, participantDirectory{find: dependencies.Participants}, schoolCalendar{find: dependencies.Calendar}, dependencies.Settings, TransactionFunc(tenant.NewTransactionRunner().RunInTx), dependencies.Observe, dependencies.Now)
	return mealplan.NewModule(engine{service: service}), nil
}

type participantDirectory struct{ find ParticipantFinder }

func (d participantDirectory) FindDailyCandidates(ctx context.Context, date domain.Date) ([]domain.DailyCandidate, domain.OperationStats, error) {
	started := time.Now()
	rows, err := d.find(ctx, date.String())
	stats := domain.OperationStats{Queries: 1, StatementDuration: time.Since(started)}
	if err != nil {
		return nil, stats, err
	}
	stats.Rows = int64(len(rows))
	candidates := make([]domain.DailyCandidate, 0, len(rows))
	for _, row := range rows {
		candidates = append(candidates, domain.DailyCandidate{
			StudentID: row.StudentID, FirstName: row.FirstName, LastName: row.LastName, SchoolClass: row.SchoolClass,
		})
	}
	return candidates, stats, nil
}

type schoolCalendar struct{ find CalendarFinder }

func (c schoolCalendar) Days(ctx context.Context, from, to domain.Date) (domain.CalendarDays, domain.OperationStats, error) {
	started := time.Now()
	closed, holiday, err := c.find(ctx, from.String(), to.String())
	return domain.NewCalendarDays(closed, holiday), domain.OperationStats{Queries: 3, StatementDuration: time.Since(started)}, err
}

// Settings are the meal plan's settings resolvers, bound at construction.
type Settings struct {
	mealPlanEnabled         func(context.Context) (bool, error)
	mealRegistrationEnabled func(context.Context) (bool, error)
	orderingSettings        func(context.Context) (OrderingSettings, error)
}

// NewBoundSettings binds the resolvers; a missing one is a construction
// error.
func NewBoundSettings(mealPlanEnabled, mealRegistrationEnabled func(context.Context) (bool, error), orderingSettings func(context.Context) (OrderingSettings, error)) (*Settings, error) {
	if mealPlanEnabled == nil || mealRegistrationEnabled == nil || orderingSettings == nil {
		return nil, errors.New("meal plan settings: resolvers are required")
	}
	return &Settings{mealPlanEnabled: mealPlanEnabled, mealRegistrationEnabled: mealRegistrationEnabled, orderingSettings: orderingSettings}, nil
}

func (s *Settings) MealPlanEnabled(ctx context.Context) (bool, error) { return s.mealPlanEnabled(ctx) }

func (s *Settings) MealRegistrationEnabled(ctx context.Context) (bool, error) {
	return s.mealRegistrationEnabled(ctx)
}

func (s *Settings) MealOrderingSettings(ctx context.Context) (OrderingSettings, error) {
	return s.orderingSettings(ctx)
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

func (e engine) SetParticipationWeek(ctx context.Context, studentID, accountID int64, monday string, participating bool) (mealplan.WeekChange, error) {
	parsed, err := domain.ParseDate(monday)
	if err != nil {
		return mealplan.WeekChange{}, err
	}
	changed, locked, err := e.service.SetParticipationWeek(ctx, studentID, accountID, parsed, participating)
	if err != nil {
		return mealplan.WeekChange{}, mapError(err)
	}
	return mealplan.WeekChange{Changed: publicDates(changed), Locked: publicDates(locked)}, nil
}

func publicDates(dates []domain.Date) []mealplan.Date {
	result := make([]mealplan.Date, 0, len(dates))
	for _, date := range dates {
		result = append(result, mealplan.Date(date.String()))
	}
	return result
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
	result := mealplan.DailyList{Date: mealplan.Date(list.Date.String()), CutoffTime: list.CutoffTime, Deadline: list.Deadline, Closed: list.Closed, Participants: make([]mealplan.DailyParticipant, 0, len(list.Participants))}
	for _, participant := range list.Participants {
		result.Participants = append(result.Participants, mealplan.DailyParticipant{StudentID: participant.StudentID, FirstName: participant.FirstName, LastName: participant.LastName, SchoolClass: participant.SchoolClass})
	}
	return result, nil
}

func publicParticipationPlan(plan domain.ParticipationPlan) mealplan.ParticipationPlan {
	result := mealplan.ParticipationPlan{EffectiveFrom: mealplan.Date(plan.EffectiveFrom.String()), CutoffTime: plan.CutoffTime, Model: mealplan.RegistrationModel(plan.Model), ScheduleChangeable: plan.ScheduleChangeable, SickCancels: plan.Sick == domain.SickCancelled, Weekdays: make([]mealplan.Weekday, 0, len(plan.Weekdays)), Days: make([]mealplan.ParticipationDay, 0, len(plan.Days))}
	for _, weekday := range plan.Weekdays {
		result.Weekdays = append(result.Weekdays, mealplan.Weekday(weekday))
	}
	for _, day := range plan.Days {
		result.Days = append(result.Days, mealplan.ParticipationDay{
			Date: mealplan.Date(day.Date.String()), Participating: day.Participating, Regular: day.Regular, Source: mealplan.ParticipationSource(day.Source),
			Changeable: day.Changeable(), CanRegister: day.CanRegister, CanCancel: day.CanCancel,
			RegisterUntil: day.RegisterUntil, CancelUntil: day.CancelUntil,
		})
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
