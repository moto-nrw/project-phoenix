package application

import "github.com/moto-nrw/project-phoenix/modules/schoolstructure/internal/ports"

func NewObserver(observe ports.Observer) ports.Observer {
	return func(observation ports.Observation) { observation.Err = MapError(observation.Err); observe(observation) }
}
