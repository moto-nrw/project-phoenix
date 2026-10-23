package api

import (
	"context"
	"fmt"
	"time"
)

// seedMealOrdersStep fixes one real order through the school's API, so the
// order week, its lines and a change after the deadline show on every dev
// machine (#3873). The school registers a child for the next weekday while
// its deadline is ahead, moves the deadline back so the day is due, reads
// the orders (which fixes the day), cancels the child after the deadline
// (a change) and restores the deadline. The caterer tables stay empty: a
// caterer exists only through the link the school mails it, which nobody
// else ever receives (ADR 0077).
type seedMealOrdersStep struct{}

func (seedMealOrdersStep) Name() string { return "Seeding meal orders" }

func (seedMealOrdersStep) Run(_ context.Context, rt *Runtime) error {
	if rt == nil || rt.Client == nil || len(rt.Parents) == 0 || len(rt.Parents[0].StudentIDs) == 0 {
		return fmt.Errorf("meal order prerequisites not available")
	}
	rt.Client.BindAuth(rt.TenantAuth)
	studentID := rt.Parents[0].StudentIDs[0]
	day, leadDays := mealOrderSeedDay(todaySeedDate())
	dayPath := fmt.Sprintf("/api/meal-plan/participation/%d/days/%s", studentID, day)
	if _, err := rt.Client.Put(dayPath, map[string]any{"participating": true}); err != nil {
		return fmt.Errorf("register child for the seeded order: %w", err)
	}
	if err := setMealLeadDays(rt, leadDays); err != nil {
		return err
	}
	if _, err := rt.Client.Get(fmt.Sprintf("/api/meal-plan/orders?from=%s&to=%s", day, day)); err != nil {
		return fmt.Errorf("fix the seeded order: %w", err)
	}
	if _, err := rt.Client.Put(dayPath, map[string]any{"participating": false}); err != nil {
		return fmt.Errorf("change the seeded order after its deadline: %w", err)
	}
	if err := setMealLeadDays(rt, 0); err != nil {
		return err
	}
	fmt.Printf("  1 meal order fixed for %s with 1 change after the deadline\n", day)
	return nil
}

// mealOrderSeedDay is the next weekday after today and the number of lead
// days that put its deadline before today, whatever the time of day.
func mealOrderSeedDay(today seedDate) (seedDate, int) {
	day := today.AddDays(1)
	for day.Weekday() == time.Saturday || day.Weekday() == time.Sunday {
		day = day.AddDays(1)
	}
	return day, int(day.Sub(today.Time).Hours()/24) + 1
}

func setMealLeadDays(rt *Runtime, days int) error {
	if _, err := rt.Client.Put("/api/settings/values/operations.meal_registration_lead_days", map[string]any{"value": days}); err != nil {
		return fmt.Errorf("set meal registration lead days to %d: %w", days, err)
	}
	return nil
}
