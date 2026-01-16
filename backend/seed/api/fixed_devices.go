package api

import (
	"context"
	"encoding/json"
	"fmt"
)

func (s *FixedSeeder) seedDevices(_ context.Context, result *FixedResult) error {
	for _, device := range DemoDevices {
		body := map[string]any{
			"device_id":   device.DeviceID,
			"name":        device.Name,
			"device_type": "rfid_scanner",
			"status":      "active",
		}

		// Device CRUD routes are at /api/iot/ (not /api/iot/devices)
		// The devices router is mounted at "/" within the IoT router
		respBody, err := s.client.Post("/api/iot/", body)
		if err != nil {
			return fmt.Errorf("failed to create device %s: %w", device.DeviceID, err)
		}

		// Parse response to extract API key
		var resp struct {
			Status string `json:"status"`
			Data   struct {
				ID     int64  `json:"id"`
				APIKey string `json:"api_key"`
			} `json:"data"`
		}
		if err := json.Unmarshal(respBody, &resp); err != nil {
			return fmt.Errorf("failed to parse device response: %w", err)
		}

		// Store device API key for later use in RuntimeSeeder
		s.deviceKeys[device.DeviceID] = resp.Data.APIKey

		result.DeviceCount++
	}

	if s.verbose {
		fmt.Printf("  ✓ %d IoT devices created\n", result.DeviceCount)
	}
	return nil
}
