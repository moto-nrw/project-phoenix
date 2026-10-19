//go:build testhooks

package compose

import (
	"github.com/moto-nrw/project-phoenix/modules/classday/internal/application"
	"github.com/moto-nrw/project-phoenix/modules/classday/internal/ports"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

type Rules application.Rules

func (r Rules) RowCareDay(completed bool, row ports.RosterFacts, verdict ports.CareDay) ports.CareDay {
	return application.Rules(r).RowCareDay(completed, row, verdict)
}
func (r Rules) EnrolledOn(student ports.StudentFacts, date, today calendar.Date) bool {
	return application.Rules(r).EnrolledOn(student, date, today)
}
