package carerequests

import (
	"encoding/json"
	"strings"

	calendar "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// PickupChangeTerms contains the stored ask, never today's plan. PickupTime
// and DepartureMode are empty for a leg the request leaves unchanged (#3945).
type PickupChangeTerms struct {
	Date                   calendar.Date
	PickupTime             string
	PreviousPickupTime     string
	DepartureMode          string
	PreviousDepartureModes []string
}

func StoredPickupTerms(raw json.RawMessage) *PickupChangeTerms {
	var payload map[string]any
	if json.Unmarshal(raw, &payload) != nil {
		return nil
	}
	change, ok := pickupTerms(payload)
	if !ok {
		return nil
	}
	terms := &PickupChangeTerms{Date: change.Date, DepartureMode: change.DepartureMode}
	if change.PickupTime != nil {
		terms.PickupTime = change.PickupTime.Format("15:04")
		if raw, ok := payload["previous_pickup_time"].(string); ok {
			if previous, err := parseWallClock(raw); err == nil {
				terms.PreviousPickupTime = previous.Format("15:04")
			}
		}
	}
	if change.DepartureMode != "" {
		terms.PreviousDepartureModes = storedModes(payload["previous_departure_modes"])
	}
	return terms
}

func storedModes(value any) []string {
	list, _ := value.([]any)
	modes := make([]string, 0, len(list))
	for _, item := range list {
		if mode, ok := item.(string); ok && mode != "" {
			modes = append(modes, mode)
		}
	}
	if len(modes) == 0 {
		return nil
	}
	return modes
}

// PickupChangeEntries renders the requested legs of one day in a fixed
// order: Abholzeit, then Abholart. oldPickup and oldModes are the values the
// day had before; empty values leave the "before" side blank, as the
// requested summary of a decided request does.
func PickupChangeEntries(change PickupChange, oldPickup string, oldModes []string) []DiffEntry {
	day := change.Date.Format("02.01.2006")
	entries := make([]DiffEntry, 0, 2)
	if change.PickupTime != nil {
		entries = append(entries, DiffEntry{Label: day + " · Abholzeit", Old: oldPickup, New: change.PickupTime.Format("15:04"), CareKind: KindPickup})
	}
	if change.DepartureMode != "" {
		entry := DiffEntry{Label: day + " · Abholart", New: departureLabel(change.DepartureMode), CareKind: KindDepartureMode, NewMode: change.DepartureMode}
		if len(oldModes) > 0 {
			entry.OldModes = append([]string(nil), oldModes...)
			labels := make([]string, 0, len(oldModes))
			for _, mode := range oldModes {
				labels = append(labels, departureLabel(mode))
			}
			entry.Old = strings.Join(labels, " / ")
		}
		entries = append(entries, entry)
	}
	return entries
}

// DepartureModeLabel is the German label the review and the pills use.
func DepartureModeLabel(mode string) string {
	return departureLabel(mode)
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
	if change, ok := pickupTerms(payload); ok {
		return PickupChangeEntries(change, "", nil)
	}
	weekly, err := decodePayload[WeeklyChange](raw)
	if err != nil {
		return nil
	}
	return WeeklySummary(weekly.Weekdays)
}
