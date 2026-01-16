package api

import (
	"context"
	"encoding/json"
	"fmt"
)

func (s *FixedSeeder) seedGroups(_ context.Context, result *FixedResult) error {
	// Create groups with themed names (typical for German OGS)
	// Each teacher gets at least one group so they see "Meine Gruppe" in frontend
	// Teacher distribution:
	//   Sternengruppe: Anna Müller, Thomas Weber, Sarah Schmidt (3 teachers)
	//   Bärengruppe: Michael Hoffmann, Lisa Wagner (2 teachers)
	//   Sonnengruppe: Jan Becker, Maria Fischer (2 teachers)
	classes := []struct {
		key      string   // lowercase for internal lookup
		name     string   // display name
		teachers []string // teacher names (must match DemoStaff)
	}{
		{key: "sternengruppe", name: "Sternengruppe", teachers: []string{"Anna Müller", "Thomas Weber", "Sarah Schmidt"}},
		{key: "bärengruppe", name: "Bärengruppe", teachers: []string{"Michael Hoffmann", "Lisa Wagner"}},
		{key: "sonnengruppe", name: "Sonnengruppe", teachers: []string{"Jan Becker", "Maria Fischer"}},
	}

	for _, class := range classes {
		// Collect teacher IDs for this group
		teacherIDsForGroup := []int64{}
		for _, teacherName := range class.teachers {
			if teacherID, ok := s.teacherIDs[teacherName]; ok {
				teacherIDsForGroup = append(teacherIDsForGroup, teacherID)
			}
		}

		body := map[string]any{
			"name":        class.name,
			"teacher_ids": teacherIDsForGroup,
		}

		respBody, err := s.client.Post("/api/groups", body)
		if err != nil {
			return fmt.Errorf("failed to create group %s: %w", class.name, err)
		}

		var resp struct {
			Status string `json:"status"`
			Data   struct {
				ID int64 `json:"id"`
			} `json:"data"`
		}
		if err := json.Unmarshal(respBody, &resp); err != nil {
			return fmt.Errorf("failed to parse group response: %w", err)
		}

		s.groupIDs[class.key] = resp.Data.ID
		result.GroupCount++
	}

	if s.verbose {
		fmt.Printf("  ✓ %d education groups created (with teacher assignments)\n", result.GroupCount)
	}
	return nil
}
