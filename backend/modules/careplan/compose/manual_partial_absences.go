package compose

import (
	applicationBindings "github.com/moto-nrw/project-phoenix/modules/careplan/internal/application"
	carePlanBindingPorts "github.com/moto-nrw/project-phoenix/modules/careplan/ports"
)

func NewManualPartialAbsences(source carePlanBindingPorts.PickupExceptionReader) *ManualPartialAbsences {
	return &ManualPartialAbsences{ManualPartialAbsences: applicationBindings.ManualPartialAbsences{Source: source}}
}

type ManualPartialAbsences struct {
	applicationBindings.ManualPartialAbsences
}
