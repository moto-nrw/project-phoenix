package application

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/mealplan"
	"github.com/moto-nrw/project-phoenix/modules/mealplan/internal/domain"
)

func NewPublicModule(service *Service) *mealplan.Module {
	return mealplan.NewModule(engine{service: service})
}

type engine struct{ service *Service }

func (e engine) Available(ctx context.Context) (bool, error) { return e.service.Available(ctx) }

func (e engine) Week(ctx context.Context, monday string) ([]mealplan.Entry, error) {
	parsed, err := domain.ParseDate(monday)
	if err != nil {
		return nil, err
	}
	entries, err := e.service.Week(ctx, parsed)
	if err != nil {
		return nil, mapPublicError(err)
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
	return mapPublicError(e.service.Replace(ctx, parsed, values))
}

func (e engine) Clear(ctx context.Context, date string) error {
	parsed, err := domain.ParseDate(date)
	if err != nil {
		return err
	}
	return mapPublicError(e.service.Clear(ctx, parsed))
}

func (e engine) RegistrationAvailable(ctx context.Context) (bool, error) {
	available, err := e.service.RegistrationAvailable(ctx)
	return available, mapPublicError(err)
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
		return mealplan.ParticipationPlan{}, mapPublicError(err)
	}
	return publicParticipationPlan(plan), nil
}

func (e engine) ReplaceParticipationSchedule(ctx context.Context, studentID, accountID int64, weekdays []mealplan.Weekday) (mealplan.Date, error) {
	values := make([]domain.Weekday, 0, len(weekdays))
	for _, weekday := range weekdays {
		values = append(values, domain.Weekday(weekday))
	}
	effectiveFrom, err := e.service.ReplaceParticipationSchedule(ctx, studentID, accountID, values)
	return mealplan.Date(effectiveFrom.String()), mapPublicError(err)
}

func (e engine) SetParticipationDay(ctx context.Context, studentID, accountID int64, date string, participating bool) error {
	parsed, err := domain.ParseDate(date)
	if err != nil {
		return err
	}
	return mapPublicError(e.service.SetParticipationDay(ctx, studentID, accountID, parsed, participating))
}

func (e engine) ClearParticipationDay(ctx context.Context, studentID, accountID int64, date string) error {
	parsed, err := domain.ParseDate(date)
	if err != nil {
		return err
	}
	return mapPublicError(e.service.ClearParticipationDay(ctx, studentID, accountID, parsed))
}

func (e engine) DailyParticipants(ctx context.Context, date string) (mealplan.DailyList, error) {
	parsed, err := domain.ParseDate(date)
	if err != nil {
		return mealplan.DailyList{}, err
	}
	list, err := e.service.DailyParticipants(ctx, parsed)
	if err != nil {
		return mealplan.DailyList{}, mapPublicError(err)
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

func mapPublicError(err error) error {
	if errors.Is(err, ErrDisabled) {
		return mealplan.ErrDisabled
	}
	if errors.Is(err, ErrRegistrationDisabled) {
		return mealplan.ErrRegistrationDisabled
	}
	if errors.Is(err, ErrCutoffPassed) {
		return mealplan.ErrParticipationCutoff
	}
	if errors.Is(err, ErrInvalidCutoff) {
		return mealplan.ErrInvalidParticipation
	}
	return err
}
