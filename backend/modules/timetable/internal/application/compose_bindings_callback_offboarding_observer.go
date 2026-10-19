package application

import (
	portsCallback "github.com/moto-nrw/project-phoenix/modules/timetable/internal/ports"
)

func NewOffboardingObserver(observe func(portsCallback.Observation)) func(observation portsCallback.Observation) {
	return func(observation portsCallback.Observation) {
		observation.Err = MapError(observation.Err)
		observe(observation)
	}
}
