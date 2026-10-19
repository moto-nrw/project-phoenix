package application

import (
	ports "github.com/moto-nrw/project-phoenix/modules/facilities/internal/ports"
)

// Observer reports every observation with the public error.
func Observer(observe func(ports.Observation)) ports.Observer {
	return func(observation ports.Observation) {
		observation.Err = MapError(observation.Err)
		observe(observation)
	}
}
