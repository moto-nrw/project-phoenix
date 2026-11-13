package care

import (
	"context"
	"errors"
	"fmt"

	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
)

// MealPlanWeek returns the child's school meal plan for the Monday-Friday week
// containing weekStart. Unlike ListSickDays this is gated by the
// operations.meal_plan_enabled toggle: if the school does not run a meal plan
// the parent must not see one, so a disabled tenant yields ErrMealPlanDisabled.
func (s *Service) MealPlanWeek(ctx context.Context, accountID, studentID int64, weekStart calendar.Date) ([]MealPlanEntry, error) {
	child, err := s.ResolvePermittedChild(ctx, accountID, studentID, GuardianPermissionPortalAccess)
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
	txErr := s.Runtime.WithinTenant(ctx, child.TenantID, func(txCtx context.Context) error {
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
	err := s.Runtime.WithinTenant(ctx, tenantID, func(txCtx context.Context) error {
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
	err := s.Runtime.WithinTenant(ctx, tenantID, func(txCtx context.Context) error {
		var resolveErr error
		available, resolveErr = s.MealPlan.RegistrationAvailable(txCtx)
		return resolveErr
	})
	if errors.Is(err, ErrMealPlanDisabled) {
		return false, nil
	}
	return available, err
}

func (s *Service) MealParticipation(ctx context.Context, accountID, studentID int64, from, to calendar.Date) (MealParticipationPlan, error) {
	var plan MealParticipationPlan
	err := s.WithPermittedChild(ctx, accountID, studentID, GuardianPermissionPortalAccess, func(*Child) error {
		currentMonday := s.todayDate().StartOfISOWeek()
		if from.Before(currentMonday) || to.After(lastMealParticipationDay(currentMonday)) || to.Before(from) {
			return ErrMealParticipationOutOfRange
		}
		return nil
	}, func(txCtx context.Context, _ *Child) error {
		var readErr error
		plan, readErr = s.MealPlan.Participation(txCtx, studentID, from, to)
		if readErr != nil {
			return mapMealParticipationError(readErr)
		}
		return nil
	})
	if err != nil {
		return MealParticipationPlan{}, err
	}
	return plan, nil
}

// ErrMealParticipationManageDenied leaves the child's meal participation readable.
var ErrMealParticipationManageDenied = errors.New("parent: meal participation management not permitted")

// mealParticipationWriteAccess holds the manage permission of a meal
// participation change until it commits (#6529); its refusal stays
// ErrMealParticipationManageDenied (mapMealParticipationError).
func mealParticipationWriteAccess(access *ChildWriteAccess) {
	access.Permissions = append(access.Permissions, GuardianPermissionMealParticipationManage)
}

// mealParticipationWriter is the between of a meal participation write: the
// relationship must grant the change and the child's care must run, both
// decided from the child check before any statement of the school.
func mealParticipationWriter(child *Child) error {
	if !child.HasPermission(GuardianPermissionMealParticipationManage) {
		return ErrMealParticipationManageDenied
	}
	return child.RequireCareRunning()
}

func (s *Service) ReplaceMealParticipationSchedule(ctx context.Context, accountID, studentID int64, weekdays []MealWeekday) (string, error) {
	var effectiveFrom string
	err := s.WithPermittedChildForWrite(ctx, accountID, studentID, GuardianPermissionPortalAccess, mealParticipationWriter, func(txCtx context.Context, _ *Child) error {
		if err := s.RequireCareRunningForUpdate(txCtx, studentID); err != nil {
			return err
		}
		var writeErr error
		effectiveFrom, writeErr = s.MealPlan.ReplaceParticipationSchedule(txCtx, MealParticipationSchedule{StudentID: studentID, GuardianAccountID: accountID, Weekdays: weekdays})
		return writeErr
	}, mealParticipationWriteAccess)
	if err != nil {
		return "", mapMealParticipationError(err)
	}
	return effectiveFrom, nil
}

func (s *Service) SetMealParticipationDay(ctx context.Context, accountID, studentID int64, date calendar.Date, participating bool) error {
	return s.changeMealParticipationDay(ctx, accountID, studentID, date, &participating)
}

// SetMealParticipationWeek registers or cancels the remaining weekdays of the
// week containing date. Days past their deadline stay and come back as
// locked.
func (s *Service) SetMealParticipationWeek(ctx context.Context, accountID, studentID int64, date calendar.Date, participating bool) (MealWeekChange, error) {
	monday := date.StartOfISOWeek()
	var result MealWeekChange
	err := s.WithPermittedChildForWrite(ctx, accountID, studentID, GuardianPermissionPortalAccess, func(child *Child) error {
		if err := mealParticipationWriter(child); err != nil {
			return err
		}
		if monday.AddDays(4).Before(s.todayDate()) || monday.After(lastMealParticipationDay(s.todayDate().StartOfISOWeek())) {
			return ErrMealParticipationOutOfRange
		}
		return nil
	}, func(txCtx context.Context, _ *Child) error {
		if err := s.RequireCareRunningForUpdate(txCtx, studentID); err != nil {
			return err
		}
		var writeErr error
		result, writeErr = s.MealPlan.SetParticipationForWeek(txCtx, MealParticipationChange{StudentID: studentID, GuardianAccountID: accountID, Date: monday, Participating: participating})
		return writeErr
	}, mealParticipationWriteAccess)
	if err != nil {
		return MealWeekChange{}, mapMealParticipationError(err)
	}
	return result, nil
}

// lastMealParticipationDay is the last day a parent reads or changes: the
// Friday three weeks after the current week, so deadlines with a lead time
// stay reachable.
func lastMealParticipationDay(currentMonday calendar.Date) calendar.Date {
	return currentMonday.AddDays(25)
}

func (s *Service) ClearMealParticipationDay(ctx context.Context, accountID, studentID int64, date calendar.Date) error {
	return s.changeMealParticipationDay(ctx, accountID, studentID, date, nil)
}

// changeMealParticipationDay sets the day's participation, or clears the
// family's choice for the day when participating is nil.
func (s *Service) changeMealParticipationDay(ctx context.Context, accountID, studentID int64, date calendar.Date, participating *bool) error {
	err := s.WithPermittedChildForWrite(ctx, accountID, studentID, GuardianPermissionPortalAccess, func(child *Child) error {
		if err := mealParticipationWriter(child); err != nil {
			return err
		}
		today := s.todayDate()
		if date.Before(today) || date.After(lastMealParticipationDay(today.StartOfISOWeek())) {
			return ErrMealParticipationOutOfRange
		}
		return nil
	}, func(txCtx context.Context, _ *Child) error {
		if err := s.RequireCareRunningForUpdate(txCtx, studentID); err != nil {
			return err
		}
		change := MealParticipationChange{StudentID: studentID, GuardianAccountID: accountID, Date: date}
		if participating == nil {
			return s.MealPlan.ClearParticipationForDay(txCtx, change)
		}
		change.Participating = *participating
		return s.MealPlan.SetParticipationForDay(txCtx, change)
	}, mealParticipationWriteAccess)
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
	case errors.Is(err, ErrGuardianPermissionDenied):
		return ErrMealParticipationManageDenied
	default:
		return err
	}
}
