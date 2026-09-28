package services

import (
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// parseStaffOfferingValue reads the offerings a staff member chose from the
// conflict coordinator payload {"value": {"effective_from": "YYYY-MM-DD",
// "selections": [{"offering_id": 12, "selected_days": ["mon"]}]}}.
func parseStaffOfferingValue(value map[string]any) (calendar.Date, []careplan.OfferingChangeSelection, error) {
	inner, ok := value["value"].(map[string]any)
	if !ok {
		return calendar.Date(""), nil, fmt.Errorf("%w: staff value is missing", careplan.ErrOfferingChangeInvalid)
	}
	rawDate, ok := inner["effective_from"].(string)
	if !ok {
		return calendar.Date(""), nil, fmt.Errorf("%w: effective_from is required", careplan.ErrOfferingChangeInvalid)
	}
	effectiveFrom, err := calendar.ParseDate(rawDate)
	if err != nil {
		return calendar.Date(""), nil, fmt.Errorf("%w: effective_from is not a date", careplan.ErrOfferingChangeInvalid)
	}
	rawSelections, ok := inner["selections"].([]any)
	if !ok {
		return calendar.Date(""), nil, fmt.Errorf("%w: selections are required", careplan.ErrOfferingChangeInvalid)
	}
	selections := make([]careplan.OfferingChangeSelection, 0, len(rawSelections))
	for _, entry := range rawSelections {
		selection, err := parseStaffOfferingSelection(entry)
		if err != nil {
			return calendar.Date(""), nil, err
		}
		selections = append(selections, selection)
	}
	return effectiveFrom, selections, nil
}

func parseStaffOfferingSelection(entry any) (careplan.OfferingChangeSelection, error) {
	raw, ok := entry.(map[string]any)
	if !ok {
		return careplan.OfferingChangeSelection{}, fmt.Errorf("%w: selection is malformed", careplan.ErrOfferingChangeInvalid)
	}
	// JSON numbers decode as float64; an offering id that is not a whole
	// number is a malformed request, not a rounding job.
	id, ok := raw["offering_id"].(float64)
	if !ok || id <= 0 || id != float64(int64(id)) {
		return careplan.OfferingChangeSelection{}, fmt.Errorf("%w: offering_id is invalid", careplan.ErrOfferingChangeInvalid)
	}
	rawDays, _ := raw["selected_days"].([]any)
	days := make([]string, 0, len(rawDays))
	for _, day := range rawDays {
		text, ok := day.(string)
		if !ok {
			return careplan.OfferingChangeSelection{}, fmt.Errorf("%w: selected_days are invalid", careplan.ErrOfferingChangeInvalid)
		}
		days = append(days, text)
	}
	return careplan.OfferingChangeSelection{OfferingID: int64(id), SelectedDays: days}, nil
}
