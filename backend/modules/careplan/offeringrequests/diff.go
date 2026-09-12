// Package offeringrequests defines native booking-review facts.
package offeringrequests

import (
	"encoding/json"
	"fmt"
	"slices"
	"strconv"
	"strings"
)

type DiffEntry struct {
	OfferingID          int64
	Label               string
	OldState            string
	OldDays             []string
	NewState            string
	NewDays             []string
	NewAutomaticDays    []string
	NewRuleDays         []string
	NewDaysWithoutRules []string
	AutoTriggerIDs      []int64
	AutoTriggerNames    []string
	IsCourse            bool
}

type adjustmentSnapshot struct {
	OfferingID   string   `json:"offering_id"`
	OfferingName string   `json:"offering_name"`
	SelectedDays []string `json:"selected_days,omitempty"`
}

// CorrectionDiff reads the frozen before/after state, never the live catalog.
func CorrectionDiff(beforeRaw, afterRaw json.RawMessage) ([]DiffEntry, error) {
	before, err := decodeAdjustment(beforeRaw)
	if err != nil {
		return nil, err
	}
	after, err := decodeAdjustment(afterRaw)
	if err != nil {
		return nil, err
	}
	ids := make([]int64, 0, len(before)+len(after))
	labels := make(map[int64]string, len(before)+len(after))
	for _, side := range []map[int64]adjustmentSnapshot{before, after} {
		for id, snapshot := range side {
			if !slices.Contains(ids, id) {
				ids = append(ids, id)
			}
			if name := strings.TrimSpace(snapshot.OfferingName); name != "" {
				labels[id] = name
			}
		}
	}
	slices.Sort(ids)
	diff := make([]DiffEntry, 0, len(ids))
	for _, id := range ids {
		entry := DiffEntry{OfferingID: id, Label: labels[id], OldState: "not_booked", NewState: "removed"}
		if entry.Label == "" {
			entry.Label = fmt.Sprintf("Angebot %d", id)
		}
		if snapshot, ok := before[id]; ok {
			entry.OldState, entry.OldDays = "booked", CanonicalDays(snapshot.SelectedDays)
		}
		if snapshot, ok := after[id]; ok {
			entry.NewState, entry.NewDays = "booked", CanonicalDays(snapshot.SelectedDays)
		}
		if entry.OldState != entry.NewState || !slices.Equal(entry.OldDays, entry.NewDays) {
			diff = append(diff, entry)
		}
	}
	return diff, nil
}

func decodeAdjustment(raw json.RawMessage) (map[int64]adjustmentSnapshot, error) {
	result := map[int64]adjustmentSnapshot{}
	if len(raw) == 0 {
		return result, nil
	}
	var entries []adjustmentSnapshot
	if err := json.Unmarshal(raw, &entries); err != nil {
		return nil, err
	}
	for _, entry := range entries {
		id, err := strconv.ParseInt(entry.OfferingID, 10, 64)
		if err != nil {
			return nil, err
		}
		result[id] = entry
	}
	return result, nil
}

func CanonicalDays(days []string) []string {
	week := [...]string{"mon", "tue", "wed", "thu", "fri", "sat", "sun"}
	selected := make(map[string]bool, len(days))
	for _, day := range days {
		selected[strings.ToLower(strings.TrimSpace(day))] = true
	}
	result := make([]string, 0, len(days))
	for _, day := range week {
		if selected[day] {
			result = append(result, day)
		}
	}
	return result
}
