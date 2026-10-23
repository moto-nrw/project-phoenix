package care

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

type MealPlanSource[D ~string, E, P, R, C any] interface {
	Available(context.Context) (bool, error)
	Week(context.Context, D) ([]E, error)
	RegistrationAvailable(context.Context) (bool, error)
	Participation(context.Context, int64, D, D) (P, error)
	ReplaceParticipationSchedule(context.Context, R) (D, error)
	SetParticipationForDay(context.Context, C) error
	ClearParticipationForDay(context.Context, C) error
}
type MealPlanRows[D ~string, E, P any, W ~int, Day, R, C any] struct {
	Entry            func(E) MealPlanEntry
	Plan             func(P) (string, string, int, []W, []Day)
	ParticipationDay func(Day) MealParticipationDay
	Schedule         func(int64, int64, []W) R
	Change           func(int64, int64, D, bool) C
}
type MealPlanErrors struct{ Disabled, RegistrationDisabled, Cutoff, Invalid error }
type mealPlan[D ~string, E, P any, W ~int, Day, R, C any] struct {
	provider MealPlanSource[D, E, P, R, C]
	rows     MealPlanRows[D, E, P, W, Day, R, C]
	errors   MealPlanErrors
}

func NewMealPlan[D ~string, E, P any, W ~int, Day, R, C any](provider MealPlanSource[D, E, P, R, C], rows MealPlanRows[D, E, P, W, Day, R, C], kinds MealPlanErrors) MealPlan {
	return mealPlan[D, E, P, W, Day, R, C]{provider: provider, rows: rows, errors: kinds}
}
func mealParticipationDay[D ~string, C any](change MealParticipationChange, convert func(int64, int64, D, bool) C) C {
	return convert(change.StudentID, change.GuardianAccountID, D(change.Date.String()), change.Participating)
}
func portalSchedule[W ~int, R any](schedule MealParticipationSchedule, convert func(int64, int64, []W) R) R {
	weekdays := make([]W, 0, len(schedule.Weekdays))
	for _, weekday := range schedule.Weekdays {
		weekdays = append(weekdays, W(weekday))
	}
	return convert(schedule.StudentID, schedule.GuardianAccountID, weekdays)
}
func portalParticipation[D ~string, E, P any, W ~int, Day, R, C any](plan P, rows MealPlanRows[D, E, P, W, Day, R, C]) MealParticipationPlan {
	effective, cutoff, leadDays, weekdays, days := rows.Plan(plan)
	out := MealParticipationPlan{EffectiveFrom: effective, CutoffTime: cutoff, CutoffLeadDays: leadDays, Weekdays: make([]MealWeekday, 0, len(weekdays)), Days: make([]MealParticipationDay, 0, len(days))}
	for _, weekday := range weekdays {
		out.Weekdays = append(out.Weekdays, MealWeekday(weekday))
	}
	for _, day := range days {
		out.Days = append(out.Days, portalRecord(day, rows.ParticipationDay))
	}
	return out
}

func (m mealPlan[D, E, P, W, Day, R, C]) Available(ctx context.Context) (bool, error) {
	available, err := m.provider.Available(ctx)
	return available, mealPlanError(err, m.errors)
}

func (m mealPlan[D, E, P, W, Day, R, C]) Week(ctx context.Context, monday calendar.Date) ([]MealPlanEntry, error) {
	rows, err := m.provider.Week(ctx, D(monday.String()))
	if err != nil {
		return nil, mealPlanError(err, m.errors)
	}
	out := make([]MealPlanEntry, 0, len(rows))
	for _, row := range rows {
		out = append(out, portalRecord(row, m.rows.Entry))
	}
	return out, nil
}

func (m mealPlan[D, E, P, W, Day, R, C]) RegistrationAvailable(ctx context.Context) (bool, error) {
	available, err := m.provider.RegistrationAvailable(ctx)
	return available, mealPlanError(err, m.errors)
}

func (m mealPlan[D, E, P, W, Day, R, C]) Participation(ctx context.Context, studentID int64, from, to calendar.Date) (MealParticipationPlan, error) {
	plan, err := m.provider.Participation(ctx, studentID, D(from.String()), D(to.String()))
	if err != nil {
		return MealParticipationPlan{}, mealPlanError(err, m.errors)
	}
	out := portalParticipation(plan, m.rows)
	return out, nil
}

func (m mealPlan[D, E, P, W, Day, R, C]) ReplaceParticipationSchedule(ctx context.Context, schedule MealParticipationSchedule) (string, error) {
	effectiveFrom, err := m.provider.ReplaceParticipationSchedule(ctx, portalSchedule(schedule, m.rows.Schedule))
	return string(effectiveFrom), mealPlanError(err, m.errors)
}

func (m mealPlan[D, E, P, W, Day, R, C]) SetParticipationForDay(ctx context.Context, change MealParticipationChange) error {
	return mealPlanError(m.provider.SetParticipationForDay(ctx, mealParticipationDay(change, m.rows.Change)), m.errors)
}

func (m mealPlan[D, E, P, W, Day, R, C]) ClearParticipationForDay(ctx context.Context, change MealParticipationChange) error {
	change.Participating = false
	return mealPlanError(m.provider.ClearParticipationForDay(ctx, mealParticipationDay(change, m.rows.Change)), m.errors)
}

func mealPlanError(err error, kinds MealPlanErrors) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, kinds.Disabled):
		return errors.Join(ErrMealPlanDisabled, err)
	case errors.Is(err, kinds.RegistrationDisabled):
		return errors.Join(ErrMealRegistrationDisabled, err)
	case errors.Is(err, kinds.Cutoff):
		return errors.Join(ErrMealParticipationCutoff, err)
	case errors.Is(err, kinds.Invalid):
		return errors.Join(ErrInvalidMealParticipation, err)
	default:
		return err
	}
}
