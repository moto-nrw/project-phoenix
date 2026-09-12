package offeringrequests

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

var ErrInvalidPayload = errors.New("enrollment: invalid offering change request")

type Selection struct {
	OfferingID   int64
	SelectedDays []string
}

// ParseSelections preserves the stored JSON payload contract, including
// canonical weekday normalization. Malformed payloads are never withdrawals.
func ParseSelections(raw json.RawMessage) ([]Selection, error) {
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidPayload, err)
	}
	return selectionsFromPayload(payload)
}

func selectionsFromPayload(payload map[string]any) ([]Selection, error) {
	raw, ok := payload["offerings"]
	if !ok {
		return nil, fmt.Errorf("%w: payload has no offerings", ErrInvalidPayload)
	}
	list, ok := raw.([]any)
	if !ok {
		return nil, fmt.Errorf("%w: payload offerings must be a list", ErrInvalidPayload)
	}
	selections := make([]Selection, 0, len(list))
	for _, entry := range list {
		selection, err := selectionFromPayloadEntry(entry)
		if err != nil {
			return nil, err
		}
		selections = append(selections, selection)
	}
	return selections, nil
}

func selectionFromPayloadEntry(entry any) (Selection, error) {
	row, ok := entry.(map[string]any)
	if !ok {
		return Selection{}, fmt.Errorf("%w: payload offering entry is not an object", ErrInvalidPayload)
	}
	id, err := payloadInt64(row["offering_id"])
	if err != nil {
		return Selection{}, err
	}
	selection := Selection{OfferingID: id}
	daysRaw, hasDays := row["selected_days"]
	if !hasDays || daysRaw == nil {
		return selection, nil
	}
	days, ok := daysRaw.([]any)
	if !ok {
		return Selection{}, fmt.Errorf("%w: payload selected_days must be a list", ErrInvalidPayload)
	}
	values, err := payloadStrings(days)
	if err != nil {
		return Selection{}, err
	}
	selection.SelectedDays = CanonicalDays(values)
	return selection, nil
}

func payloadStrings(values []any) ([]string, error) {
	strings := make([]string, 0, len(values))
	for _, value := range values {
		text, ok := value.(string)
		if !ok {
			return nil, fmt.Errorf("%w: payload selected_days must contain strings", ErrInvalidPayload)
		}
		strings = append(strings, text)
	}
	return strings, nil
}

func payloadInt64(value any) (int64, error) {
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
			return 0, fmt.Errorf("%w: offering id %q is not a number", ErrInvalidPayload, typed)
		}
		return parsed, nil
	default:
		return 0, fmt.Errorf("%w: offering id is missing", ErrInvalidPayload)
	}
}

type decisionEntry struct {
	OfferingID int64    `json:"offering_id"`
	Label      string   `json:"label"`
	OldState   string   `json:"old_state"`
	OldDays    []string `json:"old_days,omitempty"`
	NewState   string   `json:"new_state"`
	NewDays    []string `json:"new_days,omitempty"`
	// NewAutomaticDays is the share of NewDays a Mitbuchungs-Regel (or the
	// required-lunch derivation) added rather than the parents.
	NewAutomaticDays []string `json:"new_automatic_days,omitempty"`
	// NewRuleDays is the subset added specifically by a Mitbuchungs-Regel.
	NewRuleDays []string `json:"new_rule_days,omitempty"`
	// AutoTriggerNames names the selected offerings that triggered the
	// automatic share; empty for the required-lunch derivation.
	AutoTriggerNames []string `json:"auto_trigger_names,omitempty"`
	// IsCourse preserves the staff-visible course marker on a decided request.
	IsCourse bool `json:"is_course,omitempty"`
}

// DecisionDiff reads the frozen decision facts without consulting live rules.
func DecisionDiff(raw json.RawMessage) ([]DiffEntry, error) {
	var snapshot struct {
		Diff []decisionEntry `json:"diff"`
	}
	if err := json.Unmarshal(raw, &snapshot); err != nil {
		return nil, err
	}
	result := make([]DiffEntry, 0, len(snapshot.Diff))
	for _, entry := range snapshot.Diff {
		result = append(result, DiffEntry{
			OfferingID: entry.OfferingID, Label: entry.Label,
			OldState: entry.OldState, OldDays: entry.OldDays,
			NewState: entry.NewState, NewDays: entry.NewDays,
			NewAutomaticDays: entry.NewAutomaticDays, NewRuleDays: entry.NewRuleDays,
			AutoTriggerNames: entry.AutoTriggerNames, IsCourse: entry.IsCourse,
		})
	}
	return result, nil
}
