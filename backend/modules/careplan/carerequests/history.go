package carerequests

import (
	"encoding/json"

	calendar "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// PickupChangeTerms contains the stored ask, never today's plan.
type PickupChangeTerms struct {
	Date               calendar.Date
	PickupTime         string
	PreviousPickupTime string
}

func StoredPickupTerms(raw json.RawMessage) *PickupChangeTerms {
	var payload map[string]any
	if json.Unmarshal(raw, &payload) != nil {
		return nil
	}
	date, pickup, ok := pickupTerms(payload)
	if !ok {
		return nil
	}
	terms := &PickupChangeTerms{Date: date, PickupTime: pickup.Format("15:04")}
	if raw, ok := payload["previous_pickup_time"].(string); ok {
		if previous, err := parseWallClock(raw); err == nil {
			terms.PreviousPickupTime = previous.Format("15:04")
		}
	}
	return terms
}

// DecisionSnapshot stores the review comparison, not a projection of today's
// plan. Its JSON field names are the existing persisted contract (ADR 0002).
type DecisionSnapshot struct {
	Diff []SnapshotEntry `json:"diff"`
}

type SnapshotEntry struct {
	Label    string   `json:"label"`
	Old      string   `json:"old,omitempty"`
	New      string   `json:"new,omitempty"`
	Weekday  int      `json:"weekday,omitempty"`
	CareKind string   `json:"care_kind,omitempty"`
	OldModes []string `json:"old_modes,omitempty"`
	NewMode  string   `json:"new_mode,omitempty"`
}

// FreezeDecision copies the comparison so later edits cannot alter history.
func FreezeDecision(diff []DiffEntry) *DecisionSnapshot {
	snapshot := &DecisionSnapshot{Diff: make([]SnapshotEntry, 0, len(diff))}
	for _, entry := range diff {
		snapshot.Diff = append(snapshot.Diff, SnapshotEntry{
			Label: entry.Label, Old: entry.Old, New: entry.New,
			Weekday: entry.Weekday, CareKind: entry.CareKind,
			OldModes: append([]string(nil), entry.OldModes...), NewMode: entry.NewMode,
		})
	}
	return snapshot
}

// Entries replays only the frozen comparison. A nil snapshot identifies old
// or withdrawn rows whose readers fall back to the stored requested summary.
func (s *DecisionSnapshot) Entries() []DiffEntry {
	if s == nil {
		return nil
	}
	entries := make([]DiffEntry, 0, len(s.Diff))
	for _, entry := range s.Diff {
		entries = append(entries, DiffEntry{
			Label: entry.Label, Old: entry.Old, New: entry.New,
			Weekday: entry.Weekday, CareKind: entry.CareKind,
			OldModes: append([]string(nil), entry.OldModes...), NewMode: entry.NewMode,
		})
	}
	return entries
}

// RequestedSummary reads the stored ask without consulting the live plan.
// Malformed historical payloads leave the row readable with an empty summary.
func RequestedSummary(raw json.RawMessage) []DiffEntry {
	var payload map[string]any
	if json.Unmarshal(raw, &payload) != nil {
		return nil
	}
	if date, pickup, ok := pickupTerms(payload); ok {
		return []DiffEntry{{Label: date.Format("02.01.2006") + " · Abholzeit", New: pickup.Format("15:04"), CareKind: KindPickup}}
	}
	weekly, err := decodePayload[WeeklyChange](raw)
	if err != nil {
		return nil
	}
	return WeeklySummary(weekly.Weekdays)
}
