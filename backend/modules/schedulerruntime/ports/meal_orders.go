package ports

import "context"

// MealOrderSettlement fixes the due meal orders of the school in ctx and
// records every change of an ordered day since (#3873). The root binds the
// Meal Plan.
type MealOrderSettlement func(ctx context.Context) error
