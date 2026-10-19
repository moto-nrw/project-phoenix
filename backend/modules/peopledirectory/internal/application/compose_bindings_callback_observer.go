package application

import (
	internalports "github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/ports"
)

func NewObserver(observe func(internalports.Observation)) func(observation internalports.Observation) {
	return func(observation internalports.Observation) {
		observation.Err = MapError(observation.Err)
		observe(observation)
	}
}
