package api

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
)

func (s *FixedSeeder) seedGroupHandover(_ context.Context) error {
	groupID, ok := s.groupIDs["sternengruppe"]
	if !ok {
		return fmt.Errorf("group not found: sternengruppe")
	}
	targetStaffID, ok := s.staffIDs["Birgit Braun"]
	if !ok {
		return fmt.Errorf("staff not found: Birgit Braun")
	}

	today := timezone.TodayDate()
	_, err := s.client.Post("/api/substitutions", map[string]any{
		"type": "group_handover",
		"group_handover": map[string]any{
			"group_id": groupID, "target_staff_id": targetStaffID,
			"start_date": today.String(), "end_date": today.AddDays(2).String(),
		},
	})
	return err
}
