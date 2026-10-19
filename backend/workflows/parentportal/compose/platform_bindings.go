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
	ClearParticipationForDay(context.Context, mealplanModule.SetParticipationDay) error
}

func newMealPlan(provider MealPlanProvider) care.MealPlan {
	return care.NewMealPlan(provider, care.MealPlanRows[mealplanModule.Date, mealplanModule.Entry, mealplanModule.ParticipationPlan, mealplanModule.Weekday, mealplanModule.ParticipationDay, mealplanModule.ReplaceParticipationSchedule, mealplanModule.SetParticipationDay]{
		Entry: func(row mealplanModule.Entry) care.MealPlanEntry {
			return care.MealPlanEntry{Date: string(row.Date), Position: row.Position, Dish: row.Dish, Note: row.Note}
		},
		Plan: func(row mealplanModule.ParticipationPlan) (string, string, []mealplanModule.Weekday, []mealplanModule.ParticipationDay) {
			return string(row.EffectiveFrom), row.CutoffTime, row.Weekdays, row.Days
		},
		ParticipationDay: func(row mealplanModule.ParticipationDay) care.MealParticipationDay {
			return care.MealParticipationDay{Date: string(row.Date), Participating: row.Participating, Source: string(row.Source), Changeable: row.Changeable}
		},
		Schedule: func(studentID, guardianID int64, weekdays []mealplanModule.Weekday) mealplanModule.ReplaceParticipationSchedule {
			return mealplanModule.ReplaceParticipationSchedule{StudentID: studentID, GuardianAccountID: guardianID, Weekdays: weekdays}
		},
		Change: func(studentID, guardianID int64, day mealplanModule.Date, participating bool) mealplanModule.SetParticipationDay {
			return mealplanModule.SetParticipationDay{StudentID: studentID, GuardianAccountID: guardianID, Date: day, Participating: participating}
		},
	}, care.MealPlanErrors{Disabled: mealplanModule.ErrDisabled, RegistrationDisabled: mealplanModule.ErrRegistrationDisabled, Cutoff: mealplanModule.ErrParticipationCutoff, Invalid: mealplanModule.ErrInvalidParticipation})
}
