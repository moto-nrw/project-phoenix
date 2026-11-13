package api

import (
	"fmt"
	"time"
)

// seedStaffWithoutTarget keeps one member of staff without a Soll (#6887),
// the case a school without maintained contracts runs into: the Zeitkonten
// count her, her Krankheit and Urlaub credit nothing, and the Arbeitszeitmodell
// tab shows its empty state.
const seedStaffWithoutTarget = "Mai Pham"

func seedSchedulesViaAPI(rt *Runtime, staff []StaffCredentials, staffIDByEmail map[string]int64, validFrom time.Time) (int, error) {
	rt.Client.BindAuth(rt.TenantAuth)
	entries := make([]map[string]any, 0, 4-0+1)
	for d := 0; d <= 4; d++ {
		entries = append(entries, map[string]any{
			"week_index":     0,
			"day_of_week":    d,
			"target_minutes": timeTrackingDailyTargetMin,
		})
	}

	count := 0
	for _, cred := range staff {
		if cred.Position == "Extern" || cred.Name == seedStaffWithoutTarget {
			continue
		}
		staffID := staffIDByEmail[cred.Email]
		path := fmt.Sprintf("/api/staff/%d/schedule", staffID)
		body := map[string]any{
			"mode":            "custom",
			"rotation_length": 1,
			"valid_from":      toDateKey(validFrom),
			"entries":         entries,
		}
		if _, err := rt.Client.Put(path, body); err != nil {
			return count, fmt.Errorf("put schedule for staff %d: %w", staffID, err)
		}
		count += len(entries)
	}
	return count, nil
}
