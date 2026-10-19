package application

import (
	deliveryContract "github.com/moto-nrw/project-phoenix/modules/delivery/contract"
	"github.com/moto-nrw/project-phoenix/modules/delivery/internal/domain"
)

func NewObservationAdapter(observe func(deliveryContract.Observation)) func(domain.Observation) {
	return func(observation domain.Observation) {
		observe(deliveryContract.Observation{
			Operation: observation.Operation, Transport: deliveryContract.Transport(observation.Transport),
			Template: observation.Template, Duration: observation.Duration, Count: observation.Count, Err: observation.Err,
		})
	}
}
