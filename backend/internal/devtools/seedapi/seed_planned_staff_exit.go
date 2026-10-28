package api

import (
	"context"
	"fmt"
	"time"
)

// seedPlannedStaffExitStep plans the exit of one demo staff member (#4809),
// so the profile and the staff list show „Austritt am …" on every dev
// machine. The last working day is the end of the month after next: far
// enough that neither the seed nor a demo running for weeks offboards the
// person, and it never falls into the reminder week on the seed day.
type seedPlannedStaffExitStep struct{}

func (seedPlannedStaffExitStep) Name() string { return "Seeding a planned staff exit" }

func (seedPlannedStaffExitStep) Run(_ context.Context, rt *Runtime) error {
	staffIDs := orderedSeedStaffIDs(rt.FixedSeeder)
	if len(staffIDs) < 2 {
		return fmt.Errorf("a planned staff exit needs the demo staff")
	}
	staffID := staffIDs[len(staffIDs)-1]
	day := plannedSeedExitDay(todaySeedDate())
	path := fmt.Sprintf("/api/staff/%d/planned-exit", staffID)
	if _, err := rt.Client.PutWithAuth(rt.TenantAuth, path, map[string]any{"last_working_day": day.String()}); err != nil {
		return fmt.Errorf("plan the exit of staff %d: %w", staffID, err)
	}
	fmt.Printf("  exit of staff %d planned for %s\n", staffID, day)
	return nil
}

// plannedSeedExitDay is the last day of the month after next.
func plannedSeedExitDay(today seedDate) seedDate {
	firstOfMonth := time.Date(today.Year(), today.Month(), 1, 0, 0, 0, 0, time.UTC)
	return seedDate{Time: firstOfMonth.AddDate(0, 3, -1)}
}
