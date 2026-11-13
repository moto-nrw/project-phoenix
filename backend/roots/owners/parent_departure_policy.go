package owners

import (
	"encoding/json"

	"github.com/moto-ogs/project-phoenix/modules/peopledirectory"
	"github.com/moto-ogs/project-phoenix/workflows/parentportal"
)

func ParentDeparturePolicy() parentportal.DeparturePolicy {
	return parentportal.NewDeparturePolicy(func(raw json.RawMessage, validate bool) (parentportal.DepartureModesInspection, error) {
		value, err := peopledirectory.InspectDepartureModesJSON(raw, validate)
		return parentportal.DepartureModesInspection(value), err
	}, peopledirectory.DepartureDayOrder)
}
