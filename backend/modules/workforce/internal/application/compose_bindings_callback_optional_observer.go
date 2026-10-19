package application

import (
	portsCallback "github.com/moto-nrw/project-phoenix/modules/workforce/internal/ports"
)

func NewOptionalObserver(observe func(portsCallback.Observation)) func(observation portsCallback.Observation) {
	return func(observation portsCallback.Observation) {
		if observe != nil {
			observation.Err = MapError(observation.Err)
			observe(observation)
		}
	}
}
