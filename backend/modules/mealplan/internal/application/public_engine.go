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

func (e engine) ReplaceParticipationSchedule(ctx context.Context, studentID, accountID int64, weekdays []mealplan.Weekday, effectiveFrom string) (mealplan.Date, error) {
	values := make([]domain.Weekday, 0, len(weekdays))
	for _, weekday := range weekdays {
		values = append(values, domain.Weekday(weekday))
	}
	from, err := e.service.ReplaceParticipationSchedule(ctx, studentID, accountID, values, domain.Date(effectiveFrom))
	return mealplan.Date(from.String()), mapPublicError(err)
}

func (e engine) SetParticipationDay(ctx context.Context, studentID, accountID int64, date string, participating, bySchool bool) error {
	parsed, err := domain.ParseDate(date)
	if err != nil {
		return err
	}
	return mapPublicError(e.service.SetParticipationDay(ctx, studentID, accountID, parsed, participating, bySchool))
}

func (e engine) SetParticipationWeek(ctx context.Context, studentID, accountID int64, monday string, participating bool) (mealplan.WeekChange, error) {
	parsed, err := domain.ParseDate(monday)
	if err != nil {
		return mealplan.WeekChange{}, err
	}
	changed, locked, err := e.service.SetParticipationWeek(ctx, studentID, accountID, parsed, participating)
	if err != nil {
		return mealplan.WeekChange{}, mapPublicError(err)
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

func (e engine) ClearParticipationDay(ctx context.Context, studentID, accountID int64, date string, bySchool bool) error {
	parsed, err := domain.ParseDate(date)
	if err != nil {
		return err
	}
	return mapPublicError(e.service.ClearParticipationDay(ctx, studentID, accountID, parsed, bySchool))
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
	result := mealplan.DailyList{Date: mealplan.Date(list.Date.String()), CutoffTime: list.CutoffTime, Deadline: list.Deadline, Closed: list.Closed, Participants: make([]mealplan.DailyParticipant, 0, len(list.Participants))}
	for _, participant := range list.Participants {
		result.Participants = append(result.Participants, mealplan.DailyParticipant(participant))
	}
	for _, other := range list.Others {
		result.Others = append(result.Others, mealplan.DailyParticipant(other))
	}
	return result, nil
}

func (e engine) SettleOrders(ctx context.Context) error {
	return mapPublicError(e.service.SettleOrders(ctx))
}

func (e engine) Orders(ctx context.Context, from, to string) (mealplan.OrderWeek, error) {
	view, err := e.service.Orders(ctx, domain.Date(from), domain.Date(to))
	if err != nil {
		return mealplan.OrderWeek{}, mapPublicError(err)
	}
	week := mealplan.OrderWeek{Rhythm: string(view.Rules.Rhythm), Detail: string(view.Rules.Detail), Days: make([]mealplan.OrderDay, 0, len(view.Days))}
	for _, day := range view.Days {
		week.Days = append(week.Days, publicOrderDay(day, view.Names))
	}
	return week, nil
}

func publicOrderDay(day domain.OrderDay, names map[int64]domain.DailyCandidate) mealplan.OrderDay {
	child := func(studentID int64, schoolClass string) mealplan.OrderChild {
		name := names[studentID]
		return mealplan.OrderChild{StudentID: studentID, FirstName: name.FirstName, LastName: name.LastName, SchoolClass: schoolClass}
	}
	result := mealplan.OrderDay{Date: mealplan.Date(day.Date.String()), Closed: day.Closed, Deadline: day.Deadline, Status: mealplan.OrderOpen, Total: len(day.Children)}
	if day.Unordered {
		result.Status = mealplan.OrderUnordered
	}
	if day.Order != nil {
		result.Status, result.OrderID, result.ConfirmedAt = mealplan.OrderOrdered, day.Order.ID, day.Order.ConfirmedAt
		if day.Order.ConfirmedAt != nil {
			result.Status = mealplan.OrderConfirmed
		}
	}
	for _, count := range domain.CountByClass(day.Children) {
		result.Classes = append(result.Classes, mealplan.ClassCount(count))
	}
	for _, eating := range day.Eating {
		result.Children = append(result.Children, mealplan.OrderChild(eating))
	}
	for _, change := range day.Changes {
		result.Changes = append(result.Changes, mealplan.OrderChange{Sequence: change.Sequence, Date: result.Date, Child: child(change.StudentID, change.SchoolClass), Delta: change.Delta, DetectedAt: change.DetectedAt})
	}
	return result
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

// publicErrors maps the application's errors to the module's, first match
// wins.
var publicErrors = []struct{ from, to error }{
	{ErrDisabled, mealplan.ErrDisabled},
	{ErrRegistrationDisabled, mealplan.ErrRegistrationDisabled},
	{ErrCutoffPassed, mealplan.ErrParticipationCutoff},
	{ErrPastDay, mealplan.ErrPastDay},
	{ErrDayClosed, mealplan.ErrDayClosed},
	{ErrStudentNotFound, mealplan.ErrStudentNotFound},
	{ErrNotEnrolled, mealplan.ErrChildNotEnrolled},
	{ErrInvalidCutoff, mealplan.ErrInvalidParticipation},
	{ErrTooFarAhead, mealplan.ErrInvalidParticipation},
}

func mapPublicError(err error) error {
	for _, mapping := range publicErrors {
		if errors.Is(err, mapping.from) {
			return mapping.to
		}
	}
	return err
}
