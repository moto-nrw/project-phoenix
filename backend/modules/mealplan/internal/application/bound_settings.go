package application

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/mealplan/internal/ports"
)

// Settings are the meal plan's settings resolvers, bound at construction.
type Settings struct {
	mealPlanEnabled         func(context.Context) (bool, error)
	mealRegistrationEnabled func(context.Context) (bool, error)
	orderingSettings        func(context.Context) (ports.OrderingSettings, error)
}

// NewBoundSettings binds the resolvers; a missing one is a construction
// error.
func NewBoundSettings(mealPlanEnabled, mealRegistrationEnabled func(context.Context) (bool, error), orderingSettings func(context.Context) (ports.OrderingSettings, error)) (*Settings, error) {
	if mealPlanEnabled == nil || mealRegistrationEnabled == nil || orderingSettings == nil {
		return nil, errors.New("meal plan settings: resolvers are required")
	}
	return &Settings{mealPlanEnabled: mealPlanEnabled, mealRegistrationEnabled: mealRegistrationEnabled, orderingSettings: orderingSettings}, nil
}

func (s *Settings) MealPlanEnabled(ctx context.Context) (bool, error) { return s.mealPlanEnabled(ctx) }

func (s *Settings) MealRegistrationEnabled(ctx context.Context) (bool, error) {
	return s.mealRegistrationEnabled(ctx)
}

func (s *Settings) MealOrderingSettings(ctx context.Context) (ports.OrderingSettings, error) {
	return s.orderingSettings(ctx)
}
