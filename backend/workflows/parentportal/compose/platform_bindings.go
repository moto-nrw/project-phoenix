package compose

import (
	"context"
	"errors"

	mealplanModule "github.com/moto-nrw/project-phoenix/modules/mealplan"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/workflows/parentportal/internal/care"
)

// MealPlanProvider is the Meal Plan capability of the child's school, in the
// owner's vocabulary.
type MealPlanProvider interface {
	Available(context.Context) (bool, error)
	Week(context.Context, mealplanModule.Date) ([]mealplanModule.Entry, error)
	RegistrationAvailable(context.Context) (bool, error)
	Participation(context.Context, int64, mealplanModule.Date, mealplanModule.Date) (mealplanModule.ParticipationPlan, error)
	ReplaceParticipationSchedule(context.Context, mealplanModule.ReplaceParticipationSchedule) (mealplanModule.Date, error)
	SetParticipationForDay(context.Context, mealplanModule.SetParticipationDay) error
	SetParticipationForWeek(context.Context, mealplanModule.SetParticipationWeek) (mealplanModule.WeekChange, error)
	ClearParticipationForDay(context.Context, mealplanModule.SetParticipationDay) error
}

// mealPlan binds the Meal Plan capability to the flows' port and translates
// the owner's refusals into the portal's sentinels.
type mealPlan struct{ provider MealPlanProvider }

func (m mealPlan) Available(ctx context.Context) (bool, error) {
	available, err := m.provider.Available(ctx)
	return available, mealPlanError(err)
}

func (m mealPlan) Week(ctx context.Context, monday calendar.Date) ([]care.MealPlanEntry, error) {
	rows, err := m.provider.Week(ctx, mealplanModule.Date(monday.String()))
	if err != nil {
		return nil, mealPlanError(err)
	}
	out := make([]care.MealPlanEntry, 0, len(rows))
	for _, row := range rows {
		out = append(out, care.MealPlanEntry{Date: string(row.Date), Position: row.Position, Dish: row.Dish, Note: row.Note})
	}
	return out, nil
}

func (m mealPlan) RegistrationAvailable(ctx context.Context) (bool, error) {
	available, err := m.provider.RegistrationAvailable(ctx)
	return available, mealPlanError(err)
}

func (m mealPlan) Participation(ctx context.Context, studentID int64, from, to calendar.Date) (care.MealParticipationPlan, error) {
	plan, err := m.provider.Participation(ctx, studentID, mealplanModule.Date(from.String()), mealplanModule.Date(to.String()))
	if err != nil {
		return care.MealParticipationPlan{}, mealPlanError(err)
	}
	out := care.MealParticipationPlan{
		EffectiveFrom:      string(plan.EffectiveFrom),
		CutoffTime:         plan.CutoffTime,
		Model:              string(plan.Model),
		ScheduleChangeable: plan.ScheduleChangeable,
		SickCancels:        plan.SickCancels,
		Weekdays:           make([]care.MealWeekday, 0, len(plan.Weekdays)),
		Days:               make([]care.MealParticipationDay, 0, len(plan.Days)),
	}
	for _, weekday := range plan.Weekdays {
		out.Weekdays = append(out.Weekdays, care.MealWeekday(weekday))
	}
	for _, day := range plan.Days {
		out.Days = append(out.Days, care.MealParticipationDay{
			Date: string(day.Date), Participating: day.Participating, Regular: day.Regular, Source: string(day.Source), Changeable: day.Changeable,
			CanRegister: day.CanRegister, CanCancel: day.CanCancel, RegisterUntil: day.RegisterUntil, CancelUntil: day.CancelUntil,
		})
	}
	return out, nil
}

func (m mealPlan) ReplaceParticipationSchedule(ctx context.Context, schedule care.MealParticipationSchedule) (string, error) {
	weekdays := make([]mealplanModule.Weekday, 0, len(schedule.Weekdays))
	for _, weekday := range schedule.Weekdays {
		weekdays = append(weekdays, mealplanModule.Weekday(weekday))
	}
	effectiveFrom, err := m.provider.ReplaceParticipationSchedule(ctx, mealplanModule.ReplaceParticipationSchedule{
		StudentID: schedule.StudentID, GuardianAccountID: schedule.GuardianAccountID, Weekdays: weekdays,
	})
	return string(effectiveFrom), mealPlanError(err)
}

func (m mealPlan) SetParticipationForDay(ctx context.Context, change care.MealParticipationChange) error {
	return mealPlanError(m.provider.SetParticipationForDay(ctx, mealParticipationDay(change)))
}

func (m mealPlan) SetParticipationForWeek(ctx context.Context, change care.MealParticipationChange) (care.MealWeekChange, error) {
	result, err := m.provider.SetParticipationForWeek(ctx, mealplanModule.SetParticipationWeek{
		StudentID: change.StudentID, GuardianAccountID: change.GuardianAccountID,
		WeekOf: mealplanModule.Date(change.Date.String()), Participating: change.Participating,
	})
	if err != nil {
		return care.MealWeekChange{}, mealPlanError(err)
	}
	out := care.MealWeekChange{Changed: make([]string, 0, len(result.Changed)), Locked: make([]string, 0, len(result.Locked))}
	for _, date := range result.Changed {
		out.Changed = append(out.Changed, string(date))
	}
	for _, date := range result.Locked {
		out.Locked = append(out.Locked, string(date))
	}
	return out, nil
}

func (m mealPlan) ClearParticipationForDay(ctx context.Context, change care.MealParticipationChange) error {
	day := mealParticipationDay(change)
	day.Participating = false
	return mealPlanError(m.provider.ClearParticipationForDay(ctx, day))
}

func mealParticipationDay(change care.MealParticipationChange) mealplanModule.SetParticipationDay {
	return mealplanModule.SetParticipationDay{
		StudentID: change.StudentID, GuardianAccountID: change.GuardianAccountID,
		Date: mealplanModule.Date(change.Date.String()), Participating: change.Participating,
	}
}

func mealPlanError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, mealplanModule.ErrDisabled):
		return errors.Join(care.ErrMealPlanDisabled, err)
	case errors.Is(err, mealplanModule.ErrRegistrationDisabled):
		return errors.Join(care.ErrMealRegistrationDisabled, err)
	case errors.Is(err, mealplanModule.ErrParticipationCutoff), errors.Is(err, mealplanModule.ErrDayClosed), errors.Is(err, mealplanModule.ErrPastDay):
		return errors.Join(care.ErrMealParticipationCutoff, err)
	case errors.Is(err, mealplanModule.ErrInvalidParticipation):
		return errors.Join(care.ErrInvalidMealParticipation, err)
	default:
		return err
	}
}

// children resolves the guardian's child for messaging through the child
// flows, which the composition builds after messaging.
type children struct{ flows **care.Service }

func (c children) ResolvePermittedChild(ctx context.Context, accountID, studentID int64, permission string) (*care.Child, error) {
	return (*c.flows).ResolvePermittedChild(ctx, accountID, studentID, permission)
}
