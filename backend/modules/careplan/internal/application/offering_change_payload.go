package application

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The payload of an offering change request is the complete desired
// selection, {"offerings":[{"offering_id":<int>,"selected_days":["mon"]}]},
// never a delta. It is parsed strictly: a payload that cannot be read must
// not be applied as "no offerings", which would unbook the child entirely.

// offeringChangeDecisionSnapshot is the frozen review diff stored on a
// decided request (ADR 0002). The JSON shape is persisted.
type offeringChangeDecisionSnapshot struct {
	Diff                []offeringChangeSnapshotEntry    `json:"diff"`
	OverriddenOfferings []offeringChangeSnapshotOffering `json:"overridden_offerings,omitempty"`
}

type offeringChangeSnapshotEntry struct {
	OfferingID       int64    `json:"offering_id"`
	Label            string   `json:"label"`
	OldState         string   `json:"old_state"`
	OldDays          []string `json:"old_days,omitempty"`
	NewState         string   `json:"new_state"`
	NewDays          []string `json:"new_days,omitempty"`
	NewAutomaticDays []string `json:"new_automatic_days,omitempty"`
	NewRuleDays      []string `json:"new_rule_days,omitempty"`
	AutoTriggerNames []string `json:"auto_trigger_names,omitempty"`
	IsCourse         bool     `json:"is_course,omitempty"`
}

type offeringChangeSnapshotOffering struct {
	OfferingID int64  `json:"offering_id"`
	Name       string `json:"name"`
}

// offeringChangeTerminal reports whether a row can no longer be decided or
// withdrawn.
func offeringChangeTerminal(row careplan.OfferingChangeRequest) bool {
	switch row.Status {
	case careplan.OfferingChangeApproved, careplan.OfferingChangeRejected, careplan.OfferingChangeWithdrawn,
		careplan.OfferingChangeCareEnded, careplan.OfferingChangeDone:
		return true
	default:
		return false
	}
}

func offeringChangeEffectiveFrom(row careplan.OfferingChangeRequest) calendar.Date {
	return calendar.Date(row.EffectiveFrom)
}

func canonicalDays(days []string) []string {
	return careplan.CanonicalOfferingReviewDays(days)
}

func payloadFromSelections(selections []careplan.OfferingChangeSelection) (json.RawMessage, error) {
	rows := make([]any, 0, len(selections))
	for _, selection := range selections {
		row := map[string]any{"offering_id": selection.OfferingID}
		if len(selection.SelectedDays) > 0 {
			days := make([]any, 0, len(selection.SelectedDays))
			for _, day := range selection.SelectedDays {
				days = append(days, day)
			}
			row["selected_days"] = days
		}
		rows = append(rows, row)
	}
	return json.Marshal(map[string]any{"offerings": rows})
}

// requestSelections parses the stored payload of a request.
func requestSelections(row careplan.OfferingChangeRequest) ([]careplan.OfferingChangeSelection, error) {
	var payload map[string]any
	if len(row.Payload) > 0 && string(row.Payload) != "null" {
		if err := json.Unmarshal(row.Payload, &payload); err != nil {
			return nil, fmt.Errorf("decode offering change payload: %w", err)
		}
	}
	return selectionsFromPayload(payload)
}

func selectionsFromPayload(payload map[string]any) ([]careplan.OfferingChangeSelection, error) {
	raw, ok := payload["offerings"]
	if !ok {
		return nil, fmt.Errorf("%w: payload has no offerings", careplan.ErrOfferingChangeInvalid)
	}
	list, ok := raw.([]any)
	if !ok {
		return nil, fmt.Errorf("%w: payload offerings must be a list", careplan.ErrOfferingChangeInvalid)
	}
	selections := make([]careplan.OfferingChangeSelection, 0, len(list))
	for _, entry := range list {
		selection, err := selectionFromPayloadEntry(entry)
		if err != nil {
			return nil, err
		}
		selections = append(selections, selection)
	}
	return selections, nil
}

func selectionFromPayloadEntry(entry any) (careplan.OfferingChangeSelection, error) {
	row, ok := entry.(map[string]any)
	if !ok {
		return careplan.OfferingChangeSelection{}, fmt.Errorf("%w: payload offering entry is not an object", careplan.ErrOfferingChangeInvalid)
	}
	id, err := payloadOfferingID(row["offering_id"])
	if err != nil {
		return careplan.OfferingChangeSelection{}, err
	}
	selection := careplan.OfferingChangeSelection{OfferingID: id}
	daysRaw, hasDays := row["selected_days"]
	if !hasDays || daysRaw == nil {
		return selection, nil
	}
	days, ok := daysRaw.([]any)
	if !ok {
		return careplan.OfferingChangeSelection{}, fmt.Errorf("%w: payload selected_days must be a list", careplan.ErrOfferingChangeInvalid)
	}
	values := make([]string, 0, len(days))
	for _, value := range days {
		text, ok := value.(string)
		if !ok {
			return careplan.OfferingChangeSelection{}, fmt.Errorf("%w: payload selected_days must contain strings", careplan.ErrOfferingChangeInvalid)
		}
		values = append(values, text)
	}
	selection.SelectedDays = canonicalDays(values)
	return selection, nil
}

// payloadOfferingID accepts the shapes jsonb round-trips through Go: float64
// from encoding/json, plus int64 and string for hand-written payloads.
func payloadOfferingID(value any) (int64, error) {
	switch typed := value.(type) {
	case float64:
		return int64(typed), nil
	case int64:
		return typed, nil
	case int:
		return int64(typed), nil
	case string:
		parsed, err := strconv.ParseInt(strings.TrimSpace(typed), 10, 64)
		if err != nil {
			return 0, fmt.Errorf("%w: offering id %q is not a number", careplan.ErrOfferingChangeInvalid, typed)
		}
		return parsed, nil
	default:
		return 0, fmt.Errorf("%w: offering id is missing", careplan.ErrOfferingChangeInvalid)
	}
}

// decisionSnapshot reads the frozen decision of a row; nil for rows decided
// before the snapshot column existed.
func decisionSnapshot(row careplan.OfferingChangeRequest) (*offeringChangeDecisionSnapshot, error) {
	if len(row.DecisionSnapshot) == 0 || string(row.DecisionSnapshot) == "null" {
		return nil, nil
	}
	var snapshot *offeringChangeDecisionSnapshot
	if err := json.Unmarshal(row.DecisionSnapshot, &snapshot); err != nil {
		return nil, fmt.Errorf("decode offering change decision snapshot: %w", err)
	}
	return snapshot, nil
}

func encodeDecisionSnapshot(diff *offeringDecisionDiff) (json.RawMessage, error) {
	snapshot := offeringChangeDecisionSnapshot{
		Diff:                make([]offeringChangeSnapshotEntry, 0, len(diff.entries)),
		OverriddenOfferings: snapshotOverrides(diff.overridden),
	}
	for _, entry := range diff.entries {
		snapshot.Diff = append(snapshot.Diff, offeringChangeSnapshotEntry{
			OfferingID: entry.OfferingID, Label: entry.Label, OldState: entry.OldState, OldDays: entry.OldDays,
			NewState: entry.NewState, NewDays: entry.NewDays, NewAutomaticDays: entry.NewAutomaticDays,
			NewRuleDays: entry.NewRuleDays, AutoTriggerNames: entry.AutoTriggerNames, IsCourse: entry.IsCourse,
		})
	}
	return json.Marshal(snapshot)
}

func snapshotOverrides(values []careplan.OfferingOverride) []offeringChangeSnapshotOffering {
	if values == nil {
		return nil
	}
	result := make([]offeringChangeSnapshotOffering, 0, len(values))
	for _, value := range values {
		result = append(result, offeringChangeSnapshotOffering{OfferingID: value.OfferingID, Name: value.Name})
	}
	return result
}

// diffEntriesFromSnapshot maps the frozen entries back into the diff shape
// shared by the pending and recap views.
func diffEntriesFromSnapshot(entries []offeringChangeSnapshotEntry) []careplan.OfferingChangeDiffEntry {
	out := make([]careplan.OfferingChangeDiffEntry, 0, len(entries))
	for _, entry := range entries {
		out = append(out, careplan.OfferingChangeDiffEntry{
			OfferingID: entry.OfferingID, Label: entry.Label, OldState: entry.OldState, OldDays: entry.OldDays,
			NewState: entry.NewState, NewDays: entry.NewDays, NewAutomaticDays: entry.NewAutomaticDays,
			NewRuleDays: entry.NewRuleDays, AutoTriggerNames: entry.AutoTriggerNames, IsCourse: entry.IsCourse,
		})
	}
	return out
}

func overridesFromSnapshot(values []offeringChangeSnapshotOffering) []careplan.OfferingOverride {
	if values == nil {
		return nil
	}
	result := make([]careplan.OfferingOverride, 0, len(values))
	for _, value := range values {
		result = append(result, careplan.OfferingOverride{OfferingID: value.OfferingID, Name: value.Name})
	}
	return result
}

func optionalReason(value string) *string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return &value
}

func offeringIDSet(ids []int64) map[int64]bool {
	result := make(map[int64]bool, len(ids))
	for _, id := range ids {
		result[id] = true
	}
	return result
}

// isPendingOfferingChangeConflict recognises the partial unique index
// violation of a concurrent second submit.
func isPendingOfferingChangeConflict(err error) bool {
	return err != nil && strings.Contains(err.Error(), "uq_offering_change_requests_pending")
}
