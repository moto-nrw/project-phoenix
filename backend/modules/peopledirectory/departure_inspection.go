package peopledirectory

import (
	"encoding/json"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/departure"
)

type DepartureModesInspection struct {
	Normalized  json.RawMessage
	Accompanied bool
}

// InspectDepartureModesJSON uses the existing canonical value policy. Current
// stored values normalize without revalidation, exactly like retained reads.
func InspectDepartureModesJSON(raw json.RawMessage, validate bool) (DepartureModesInspection, error) {
	var modes departure.AllowedDepartureModes
	if len(raw) > 0 || validate {
		if err := json.Unmarshal(raw, &modes); err != nil {
			return DepartureModesInspection{}, err
		}
	}
	if validate {
		if err := modes.Validate(); err != nil {
			return DepartureModesInspection{}, err
		}
	}
	normalized, err := json.Marshal(modes.Normalize())
	if err != nil {
		return DepartureModesInspection{}, err
	}
	return DepartureModesInspection{Normalized: normalized, Accompanied: modes.HasMode(departure.DepartureAccompanied)}, nil
}

// DepartureDayOrder is the canonical sequence used by the retained schedule view.
func DepartureDayOrder() []string { return departure.PickupDayOrder }
