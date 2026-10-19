package application

import (
	"github.com/moto-nrw/project-phoenix/modules/appointments/internal/ports"
)

func NewObserver(observe func(ports.Observation)) func(observation ports.Observation) {
	return func(observation ports.Observation) {
		observation.Err = MapPublicError(observation.Err)
		observe(observation)
	}
}
