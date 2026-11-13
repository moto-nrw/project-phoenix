package application

import (
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
)

func NewToday() func() string { return func() string { return calendar.TodayDate().String() } }
