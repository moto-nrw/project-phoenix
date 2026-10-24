package application

import (
	"github.com/moto-nrw/project-phoenix/modules/workforce/internal/ports"
)

func NewObserver(observe func(ports.Observation)) func(observation ports.Observation) {
	return func(observation ports.Observation) {
		observation.Err = MapError(observation.Err)
		observe(observation)
	}
}
