package parentportal

import (
	"encoding/json"

	"github.com/moto-nrw/project-phoenix/workflows/parentportal/care"
)

type DepartureModesInspection care.DepartureModesInspection
type DeparturePolicy interface{ care.DeparturePolicy }

func NewDeparturePolicy(inspect func(json.RawMessage, bool) (DepartureModesInspection, error)) DeparturePolicy {
	return departurePolicy{inspect}
}

type departurePolicy struct {
	inspect func(json.RawMessage, bool) (DepartureModesInspection, error)
}

func (p departurePolicy) Inspect(raw json.RawMessage, validate bool) (care.DepartureModesInspection, error) {
	value, err := p.inspect(raw, validate)
	return care.DepartureModesInspection(value), err
}
