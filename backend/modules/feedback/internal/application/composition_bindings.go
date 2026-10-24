package application

import (
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/feedback"
	"github.com/moto-nrw/project-phoenix/modules/feedback/internal/domain"
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
