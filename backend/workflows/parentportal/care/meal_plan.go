package care

import (
	"context"
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
)

// MealPlanWeek returns the child's school meal plan for the Monday-Friday week
// containing weekStart. Unlike ListSickDays this is gated by the
// operations.meal_plan_enabled toggle: if the school does not run a meal plan
// the parent must not see one, so a disabled tenant yields ErrMealPlanDisabled.
func (s *Service) MealPlanWeek(ctx context.Context, accountID, studentID int64, weekStart timezone.Date) ([]MealPlanEntry, error) {
	child, err := s.ResolvePermittedChild(ctx, accountID, studentID, authorize.GuardianPermissionPortalAccess)
	if err != nil {
		return nil, err
	}

	monday := weekStart.StartOfISOWeek()
	// Parents may only read the current and next work week. Staff can plan
	// arbitrary future (and past) weeks on the staff page; those are drafts and
	// must not be reachable through the parent proxy by supplying a crafted
	// week_start. Compare on the normalized Monday so any day within an allowed
	// week resolves the same.
	currentMonday := s.todayDate().StartOfISOWeek()
	if monday != currentMonday && monday != currentMonday.AddDays(7) {
		return nil, ErrMealPlanWeekOutOfRange
	}

	if s.MealPlan == nil {
		return nil, errors.New("parent: meal plan capability is required")
	}
	var out []MealPlanEntry
	txErr := InTenant(ctx, child.TenantID, func(txCtx context.Context) error {
		rows, findErr := s.MealPlan.Week(txCtx, monday)
		if findErr != nil {
			return findErr
		}
		// Never nil, so an empty week stays an empty list on the wire.
		out = append(make([]MealPlanEntry, 0, len(rows)), rows...)
		return nil
	})
	if txErr != nil {
		if errors.Is(txErr, ErrMealPlanDisabled) {
			return nil, ErrMealPlanDisabled
		}
		return nil, fmt.Errorf("parent: meal plan week: %w", txErr)
	}
	return out, nil
}

func (s *Service) mealPlanAvailableForTenant(ctx context.Context, tenantID int64) (bool, error) {
	if s.MealPlan == nil {
		return false, errors.New("parent: meal plan capability is required")
	}
	var available bool
	err := InTenant(ctx, tenantID, func(txCtx context.Context) error {
		var resolveErr error
		available, resolveErr = s.MealPlan.Available(txCtx)
		return resolveErr
	})
	return available, err
}

func (s *Service) mealRegistrationAvailableForTenant(ctx context.Context, tenantID int64) (bool, error) {
	if s.MealPlan == nil {
		return false, errors.New("parent: meal plan capability is required")
	}
	var available bool
	err := InTenant(ctx, tenantID, func(txCtx context.Context) error {
		var resolveErr error
		available, resolveErr = s.MealPlan.RegistrationAvailable(txCtx)
		return resolveErr
	})
	if errors.Is(err, ErrMealPlanDisabled) {
		return false, nil
	}
	return available, err
}

func (s *Service) MealParticipation(ctx context.Context, accountID, studentID int64, from, to timezone.Date) (MealParticipationPlan, error) {
	child, err := s.ResolvePermittedChild(ctx, accountID, studentID, authorize.GuardianPermissionPortalAccess)
	if err != nil {
		return MealParticipationPlan{}, err
	}
	currentMonday := s.todayDate().StartOfISOWeek()
	if from.Before(currentMonday) || to.After(currentMonday.AddDays(11)) || to.Before(from) {
		return MealParticipationPlan{}, ErrMealParticipationOutOfRange
	}
	var plan MealParticipationPlan
	err = InTenant(ctx, child.TenantID, func(txCtx context.Context) error {
		var readErr error
		plan, readErr = s.MealPlan.Participation(txCtx, studentID, from, to)
		return readErr
	})
	if err != nil {
		return MealParticipationPlan{}, mapMealParticipationError(err)
	}
	return plan, nil
}

func (s *Service) ReplaceMealParticipationSchedule(ctx context.Context, accountID, studentID int64, weekdays []MealWeekday) (string, error) {
	child, err := s.ResolvePermittedChild(ctx, accountID, studentID, authorize.GuardianPermissionMealParticipationManage)
	if err != nil {
		return "", err
	}
	if err := child.RequireCareRunning(); err != nil {
		return "", err
	}
	var effectiveFrom string
	err = InTenant(ctx, child.TenantID, func(txCtx context.Context) error {
		if err := s.RequireCareRunningForUpdate(txCtx, studentID); err != nil {
			return err
		}
		var writeErr error
		effectiveFrom, writeErr = s.MealPlan.ReplaceParticipationSchedule(txCtx, MealParticipationSchedule{StudentID: studentID, GuardianAccountID: accountID, Weekdays: weekdays})
		return writeErr
	})
	if err != nil {
		return "", mapMealParticipationError(err)
	}
	return effectiveFrom, nil
}

func (s *Service) SetMealParticipationDay(ctx context.Context, accountID, studentID int64, date timezone.Date, participating bool) error {
	return s.changeMealParticipationDay(ctx, accountID, studentID, date, func(txCtx context.Context) error {
		return s.MealPlan.SetParticipationForDay(txCtx, MealParticipationChange{StudentID: studentID, GuardianAccountID: accountID, Date: date, Participating: participating})
	})
}

func (s *Service) ClearMealParticipationDay(ctx context.Context, accountID, studentID int64, date timezone.Date) error {
	return s.changeMealParticipationDay(ctx, accountID, studentID, date, func(txCtx context.Context) error {
		return s.MealPlan.ClearParticipationForDay(txCtx, MealParticipationChange{StudentID: studentID, GuardianAccountID: accountID, Date: date})
	})
}

func (s *Service) changeMealParticipationDay(ctx context.Context, accountID, studentID int64, date timezone.Date, change func(context.Context) error) error {
	child, err := s.ResolvePermittedChild(ctx, accountID, studentID, authorize.GuardianPermissionMealParticipationManage)
	if err != nil {
		return err
	}
	if err := child.RequireCareRunning(); err != nil {
		return err
	}
	today := s.todayDate()
	if date.Before(today) || date.After(today.StartOfISOWeek().AddDays(11)) {
		return ErrMealParticipationOutOfRange
	}
	err = InTenant(ctx, child.TenantID, func(txCtx context.Context) error {
		if err := s.RequireCareRunningForUpdate(txCtx, studentID); err != nil {
			return err
		}
		return change(txCtx)
	})
	return mapMealParticipationError(err)
}

func mapMealParticipationError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, ErrMealPlanDisabled), errors.Is(err, ErrMealRegistrationDisabled):
		return ErrMealRegistrationDisabled
	case errors.Is(err, ErrMealParticipationCutoff):
		return ErrMealParticipationCutoff
	case errors.Is(err, ErrInvalidMealParticipation):
		return ErrInvalidMealParticipation
	default:
		return err
	}
}
