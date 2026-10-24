package careplan

import (
	"fmt"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// ParseOfferingStaffValue reads the offerings a staff member chose from the
// "value" of the conflict coordinator payload {"value": {"effective_from":
// "YYYY-MM-DD", "selections": [{"offering_id": 12, "selected_days":
// ["mon"]}]}}.
func ParseOfferingStaffValue(value any) (calendar.Date, []OfferingChangeSelection, error) {
	inner, ok := value.(map[string]any)
	if !ok {
		return calendar.Date(""), nil, fmt.Errorf("%w: staff value is missing", ErrOfferingChangeInvalid)
	}
	rawDate, ok := inner["effective_from"].(string)
	if !ok {
		return calendar.Date(""), nil, fmt.Errorf("%w: effective_from is required", ErrOfferingChangeInvalid)
	}
	effectiveFrom, err := calendar.ParseDate(rawDate)
	if err != nil {
		return calendar.Date(""), nil, fmt.Errorf("%w: effective_from is not a date", ErrOfferingChangeInvalid)
	}
	rawSelections, ok := inner["selections"].([]any)
	if !ok {
		return calendar.Date(""), nil, fmt.Errorf("%w: selections are required", ErrOfferingChangeInvalid)
	}
	selections := make([]OfferingChangeSelection, 0, len(rawSelections))
	for _, entry := range rawSelections {
		selection, err := parseOfferingStaffSelection(entry)
		if err != nil {
			return calendar.Date(""), nil, err
		}
		selections = append(selections, selection)
	}
	return effectiveFrom, selections, nil
}

func parseOfferingStaffSelection(entry any) (OfferingChangeSelection, error) {
	raw, ok := entry.(map[string]any)
	if !ok {
		return OfferingChangeSelection{}, fmt.Errorf("%w: selection is malformed", ErrOfferingChangeInvalid)
	}
	// JSON numbers decode as float64; an offering id that is not a whole
	// number is a malformed request, not a rounding job.
	id, ok := raw["offering_id"].(float64)
	if !ok || id <= 0 || id != float64(int64(id)) {
		return OfferingChangeSelection{}, fmt.Errorf("%w: offering_id is invalid", ErrOfferingChangeInvalid)
	}
	rawDays, _ := raw["selected_days"].([]any)
	days := make([]string, 0, len(rawDays))
	for _, day := range rawDays {
		text, ok := day.(string)
		if !ok {
			return OfferingChangeSelection{}, fmt.Errorf("%w: selected_days are invalid", ErrOfferingChangeInvalid)
		}
		days = append(days, text)
	}
	return OfferingChangeSelection{OfferingID: int64(id), SelectedDays: days}, nil
}
