package application

import "github.com/moto-nrw/project-phoenix/modules/schoolcalendar/internal/ports"

func NewCompositionObserver(observe func(ports.Observation)) func(ports.Observation) {
	return func(observation ports.Observation) {
		observation.Err = MapPublicError(observation.Err)
		observe(observation)
	}
}
