package compose

import (
	"context"

	mealplanModule "github.com/moto-nrw/project-phoenix/modules/mealplan"
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

func newMealPlan(provider MealPlanProvider) care.MealPlan {
	return care.NewMealPlan(provider, care.MealPlanRows[mealplanModule.Date, mealplanModule.Entry, mealplanModule.ParticipationPlan, mealplanModule.Weekday, mealplanModule.ParticipationDay, mealplanModule.ReplaceParticipationSchedule, mealplanModule.SetParticipationDay]{
		Entry: func(row mealplanModule.Entry) care.MealPlanEntry {
			return care.MealPlanEntry{Date: string(row.Date), Position: row.Position, Dish: row.Dish, Note: row.Note, Allergens: row.AllergenCodes()}
		},
		Plan: func(row mealplanModule.ParticipationPlan) (care.MealParticipationPlan, []mealplanModule.Weekday, []mealplanModule.ParticipationDay) {
			return care.MealParticipationPlan{
				EffectiveFrom: string(row.EffectiveFrom), CutoffTime: row.CutoffTime, Model: string(row.Model),
				ScheduleChangeable: row.ScheduleChangeable, SickCancels: row.SickCancels,
			}, row.Weekdays, row.Days
		},
		ParticipationDay: func(row mealplanModule.ParticipationDay) care.MealParticipationDay {
			return care.MealParticipationDay{
				Date: string(row.Date), Participating: row.Participating, Regular: row.Regular, Source: string(row.Source), Changeable: row.Changeable,
				CanRegister: row.CanRegister, CanCancel: row.CanCancel, RegisterUntil: row.RegisterUntil, CancelUntil: row.CancelUntil,
			}
		},
		Week: func(ctx context.Context, change care.MealParticipationChange) (care.MealWeekChange, error) {
			return mealWeek(ctx, provider, change)
		},
		Schedule: func(studentID, guardianID int64, weekdays []mealplanModule.Weekday) mealplanModule.ReplaceParticipationSchedule {
			return mealplanModule.ReplaceParticipationSchedule{StudentID: studentID, GuardianAccountID: guardianID, Weekdays: weekdays}
		},
		Change: func(studentID, guardianID int64, day mealplanModule.Date, participating bool) mealplanModule.SetParticipationDay {
			return mealplanModule.SetParticipationDay{StudentID: studentID, GuardianAccountID: guardianID, Date: day, Participating: participating}
		},
	}, care.MealPlanErrors{
		Disabled: mealplanModule.ErrDisabled, RegistrationDisabled: mealplanModule.ErrRegistrationDisabled, Invalid: mealplanModule.ErrInvalidParticipation,
		Cutoff: []error{mealplanModule.ErrParticipationCutoff, mealplanModule.ErrDayClosed, mealplanModule.ErrPastDay},
	})
}

// mealWeek changes the open days of a week (#3873).
func mealWeek(ctx context.Context, provider MealPlanProvider, change care.MealParticipationChange) (care.MealWeekChange, error) {
	result, err := provider.SetParticipationForWeek(ctx, mealplanModule.SetParticipationWeek{
		StudentID: change.StudentID, GuardianAccountID: change.GuardianAccountID,
		WeekOf: mealplanModule.Date(change.Date.String()), Participating: change.Participating,
	})
	out := care.MealWeekChange{Changed: make([]string, 0, len(result.Changed)), Locked: make([]string, 0, len(result.Locked))}
	for _, date := range result.Changed {
		out.Changed = append(out.Changed, string(date))
	}
	for _, date := range result.Locked {
		out.Locked = append(out.Locked, string(date))
	}
	return out, err
}
