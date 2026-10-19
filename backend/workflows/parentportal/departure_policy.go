package parentportal

import (
	"encoding/json"

	"github.com/moto-nrw/project-phoenix/workflows/parentportal/internal/care"
)

type DepartureModesInspection care.DepartureModesInspection
type DeparturePolicy interface{ care.DeparturePolicy }

func NewDeparturePolicy(inspect func(json.RawMessage, bool) (DepartureModesInspection, error), days func() []string) DeparturePolicy {
	return departurePolicy{inspect: inspect, days: days}
}

type departurePolicy struct {
	days    func() []string
	inspect func(json.RawMessage, bool) (DepartureModesInspection, error)
}

func (p departurePolicy) Inspect(raw json.RawMessage, validate bool) (care.DepartureModesInspection, error) {
	value, err := p.inspect(raw, validate)
	return care.DepartureModesInspection(value), err
}

func (p departurePolicy) DayOrder() []string { return p.days() }
