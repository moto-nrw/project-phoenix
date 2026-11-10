package api

import (
	"fmt"
	"math/rand/v2"
)

// seedSessionWithBreakViaAPI records real pause intervals through live clocking.
// Keep this block on its actual day: a session-only backdate would strand its
// pause outside the session. Historical examples use the admin Nachtrag.
func seedSessionWithBreakViaAPI(client *Client, rng *rand.Rand) error {
	status := "present"
	if rng.Float64() < 0.1 {
		status = "home_office"
	}
	checkInResp, err := client.Post("/api/time-tracking/check-in", map[string]any{
		"status": status,
		"reason": "Seed-Backdatierung",
	})
	if err != nil {
		return fmt.Errorf("post check-in: %w", err)
	}
	if _, err := extractSessionID(checkInResp); err != nil {
		return fmt.Errorf("parse check-in response: %w", err)
	}
	if err := seedOneWorkSessionBreak(client); err != nil {
		return err
	}
	if _, err := client.Post("/api/time-tracking/check-out", map[string]any{"reason": "Seed-Backdatierung"}); err != nil {
		return fmt.Errorf("post check-out: %w", err)
	}
	return nil
}
