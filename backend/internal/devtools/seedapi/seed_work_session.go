package api

import (
	"fmt"
	"math/rand/v2"
	"time"
)

// seedSessionViaAPI walks the live clocking flow for one historical day:
// open + close a fresh session today, then PUT to backdate it. Returns
// whether a session was created (false when the staff already had an
// open session that we couldn't safely close).
func seedSessionViaAPI(client *Client, rng *rand.Rand, day time.Time, loc *time.Location, withBreak bool) (bool, error) {
	status := "present"
	if rng.Float64() < 0.1 {
		status = "home_office"
	}

	checkInResp, err := client.Post("/api/time-tracking/check-in", map[string]any{
		"status": status,
		"reason": "Seed-Backdatierung",
	})
	if err != nil {
		return false, fmt.Errorf("post check-in: %w", err)
	}
	sessionID, err := extractSessionID(checkInResp)
	if err != nil {
		return false, fmt.Errorf("parse check-in response: %w", err)
	}
	if withBreak {
		if err := seedOneWorkSessionBreak(client); err != nil {
			return false, err
		}
	}

	if _, err := client.Post("/api/time-tracking/check-out", map[string]any{"reason": "Seed-Backdatierung"}); err != nil {
		return false, fmt.Errorf("post check-out: %w", err)
	}

	checkInWall := time.Date(day.Year(), day.Month(), day.Day(), 8, rng.IntN(20)-10, 0, 0, loc)
	checkOutWall := time.Date(day.Year(), day.Month(), day.Day(), 16, 30+rng.IntN(30)-15, 0, 0, loc)
	breakMinutes := 30 + rng.IntN(11) - 5 // 25..35

	// The backend decodes Date as *time.Time (RFC3339), so we send a
	// midnight timestamp in UTC. The service only looks at the date
	// portion when comparing or persisting.
	updateBody := map[string]any{
		"date":           time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, time.UTC).Format(time.RFC3339),
		"check_in_time":  checkInWall.Format(time.RFC3339),
		"check_out_time": checkOutWall.Format(time.RFC3339),
		"break_minutes":  breakMinutes,
		// A note is mandatory whenever recorded times change while the
		// deviation-reason gate is on (F8). That gate now defaults on (#1844),
		// so the backdate PUT must carry one or the seeder 400s.
		"notes": "Seed-Backdatierung",
	}
	if _, err := client.Put(fmt.Sprintf("/api/time-tracking/%d", sessionID), updateBody); err != nil {
		return false, fmt.Errorf("put backdate: %w", err)
	}
	return true, nil
}
