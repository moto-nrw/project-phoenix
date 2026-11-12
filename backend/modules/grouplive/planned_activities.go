package grouplive

import (
	"context"
	"fmt"
)

// PlannedActivity is one timetable block a child is planned for today
// (#6487): an offering such as "Töpfer-AG" with its wall-clock times (HH:MM).
type PlannedActivity struct {
	Title     string `json:"title"`
	StartTime string `json:"start_time"`
	EndTime   string `json:"end_time"`
}

// applyPlannedActivities attaches today's offerings to the child cards when
// the school shows them there. Like arrival and pickup they are part of the
// child's day plan and need planning access (users:read).
func (s *service) applyPlannedActivities(ctx context.Context, state *buildState, ids []int64) error {
	if len(ids) == 0 {
		return nil
	}
	enabled, err := s.deps.Settings.StudentCardActivitiesEnabled(ctx)
	if err != nil {
		return fmt.Errorf("resolve student card activities setting: %w", err)
	}
	if !enabled {
		return nil
	}
	activities, err := s.deps.Planning.PlannedActivities(ctx, ids, state.today)
	if err != nil {
		return fmt.Errorf("load planned activities: %w", err)
	}
	for i := range state.projected {
		if state.projected[i].fullAccess {
			state.projected[i].PlannedActivities = activities[state.projected[i].ID]
		}
	}
	return nil
}
