package api

import (
	"context"
	"encoding/json"
	"fmt"
)

func (s *FixedSeeder) seedRooms(_ context.Context, result *FixedResult) error {
	for _, room := range DemoRooms {
		body := map[string]any{
			"name":     room.Name,
			"capacity": room.Capacity,
			"category": room.Category, // German category name for display
		}

		// Add building if specified
		if room.Building != "" {
			body["building"] = room.Building
		}

		// Add floor if specified (can be 0, so check for nil)
		if room.Floor != nil {
			body["floor"] = *room.Floor
		}

		respBody, err := s.client.Post("/api/rooms", body)
		if err != nil {
			return fmt.Errorf("failed to create room %s: %w", room.Name, err)
		}

		// Parse response to extract ID
		var resp struct {
			Status string `json:"status"`
			Data   struct {
				ID int64 `json:"id"`
			} `json:"data"`
		}
		if err := json.Unmarshal(respBody, &resp); err != nil {
			return fmt.Errorf("failed to parse room response: %w", err)
		}

		s.roomIDs[room.Name] = resp.Data.ID
		result.RoomCount++
	}

	if s.verbose {
		fmt.Printf("  ✓ %d rooms created\n", result.RoomCount)
	}
	return nil
}
