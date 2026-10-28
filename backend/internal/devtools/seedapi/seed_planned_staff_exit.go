package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"
)

// seedPlannedStaffExitStep plans the exit of one demo staff member (#4809),
// so the profile and the staff list show „Austritt am …" on every dev
// machine. The last working day is the end of the month after next: far
// enough that neither the seed nor a demo running for weeks offboards the
// person, and it never falls into the reminder week on the seed day.
type seedPlannedStaffExitStep struct{}

func (seedPlannedStaffExitStep) Name() string { return "Seeding a planned staff exit" }

// Run plans the exit of the last fixed demo staff member who still works at
// the school: earlier steps retire some of them, which the API answers with
// 404. The first demo staff member stays out of it.
func (seedPlannedStaffExitStep) Run(_ context.Context, rt *Runtime) error {
	staffIDs := orderedSeedStaffIDs(rt.FixedSeeder)
	day := plannedSeedExitDay(todaySeedDate())
	for i := len(staffIDs) - 1; i > 0; i-- {
		path := fmt.Sprintf("/api/staff/%d/planned-exit", staffIDs[i])
		_, err := rt.Client.PutWithAuth(rt.TenantAuth, path, map[string]any{"last_working_day": day.String()})
		var apiErr *APIError
		if errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusNotFound {
			continue
		}
		if err != nil {
			return fmt.Errorf("plan the exit of staff %d: %w", staffIDs[i], err)
		}
		fmt.Printf("  exit of staff %d planned for %s\n", staffIDs[i], day)
		return nil
	}
	return fmt.Errorf("a planned staff exit needs a live demo staff member")
}

// plannedSeedExitDay is the last day of the month after next.
func plannedSeedExitDay(today seedDate) seedDate {
	firstOfMonth := time.Date(today.Year(), today.Month(), 1, 0, 0, 0, 0, time.UTC)
	return seedDate{Time: firstOfMonth.AddDate(0, 3, -1)}
}
