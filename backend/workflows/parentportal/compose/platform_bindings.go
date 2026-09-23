package compose

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/localization"
	mealplanModule "github.com/moto-nrw/project-phoenix/modules/mealplan"
	"github.com/moto-nrw/project-phoenix/realtime"
	"github.com/moto-nrw/project-phoenix/workflows/parentportal/care"
	"github.com/moto-nrw/project-phoenix/workflows/parentportal/messaging"
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

// mealPlan binds the Meal Plan capability to the flows' port and translates
// the owner's refusals into the portal's sentinels.
type mealPlan struct{ provider MealPlanProvider }

func (m mealPlan) Available(ctx context.Context) (bool, error) {
	available, err := m.provider.Available(ctx)
	return available, mealPlanError(err)
}

func (m mealPlan) Week(ctx context.Context, monday timezone.Date) ([]care.MealPlanEntry, error) {
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

func (m mealPlan) Participation(ctx context.Context, studentID int64, from, to timezone.Date) (care.MealParticipationPlan, error) {
	plan, err := m.provider.Participation(ctx, studentID, mealplanModule.Date(from.String()), mealplanModule.Date(to.String()))
	if err != nil {
		return care.MealParticipationPlan{}, mealPlanError(err)
	}
	out := care.MealParticipationPlan{
		EffectiveFrom: string(plan.EffectiveFrom),
		CutoffTime:    plan.CutoffTime,
		Weekdays:      make([]care.MealWeekday, 0, len(plan.Weekdays)),
		Days:          make([]care.MealParticipationDay, 0, len(plan.Days)),
	}
	for _, weekday := range plan.Weekdays {
		out.Weekdays = append(out.Weekdays, care.MealWeekday(weekday))
	}
	for _, day := range plan.Days {
		out.Days = append(out.Days, care.MealParticipationDay{Date: string(day.Date), Participating: day.Participating, Source: string(day.Source), Changeable: day.Changeable})
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
	case errors.Is(err, mealplanModule.ErrParticipationCutoff):
		return errors.Join(care.ErrMealParticipationCutoff, err)
	case errors.Is(err, mealplanModule.ErrInvalidParticipation):
		return errors.Join(care.ErrInvalidMealParticipation, err)
	default:
		return err
	}
}

// studentUpdates publishes the student_updated live event for the school.
type studentUpdates struct{ broadcaster realtime.Broadcaster }

func (s studentUpdates) StudentUpdated(tenantID int64, source string) error {
	event := realtime.NewEvent(realtime.EventStudentUpdated, "", realtime.EventData{Source: &source})
	return s.broadcaster.BroadcastToTenant(tenantID, event)
}

// locales is the portal's language catalog.
type locales struct{}

func (locales) IsSupported(locale string) bool { return localization.IsSupported(locale) }
func (locales) Normalize(locale string) string { return localization.NormalizeLocale(locale) }
func (locales) Default() string                { return localization.DefaultLocale() }

// requestSharing binds the request-sharing ledger of messaging to the child
// flows' port.
type requestSharing struct{ messaging *messaging.Service }

func (r requestSharing) ShareRequestInTx(
	ctx context.Context, accountID, studentID int64, requestType string, requestID int64, recipientProfileIDs []int64,
) error {
	return r.messaging.ShareRequestInTx(ctx, accountID, studentID, requestType, requestID, recipientProfileIDs)
}

func (r requestSharing) LoadRequestShareVisibility(ctx context.Context, studentID int64) (care.RequestShareVisibility, error) {
	visibility, err := r.messaging.LoadRequestShareVisibility(ctx, studentID)
	if err != nil || visibility == nil {
		return nil, err
	}
	return visibility, nil
}

// children resolves the guardian's child for messaging through the child
// flows, which the composition builds after messaging.
type children struct{ flows **care.Service }

func (c children) ResolvePermittedChild(ctx context.Context, accountID, studentID int64, permission string) (*care.Child, error) {
	return (*c.flows).ResolvePermittedChild(ctx, accountID, studentID, permission)
}
