package application

import (
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/feedback"
	"github.com/moto-nrw/project-phoenix/modules/feedback/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/feedback/internal/ports"
)

func NewCurrentDate(current func() feedback.Date) func() domain.Date {
	return func() domain.Date {
		date, err := domain.ParseDate(string(current()))
		if err != nil {
			panic(fmt.Sprintf("feedback compose: invalid current date: %v", err))
		}
		return date
	}
}
func NewPublicObserver(observe func(ports.Observation)) func(ports.Observation) {
	return func(observation ports.Observation) {
		observation.Err = MapPublicError(observation.Err, 0)
		observe(observation)
	}
}
