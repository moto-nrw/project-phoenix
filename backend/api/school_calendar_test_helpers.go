package api

import (
	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar"
	schoolCalendarCompose "github.com/moto-nrw/project-phoenix/modules/schoolcalendar/compose"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
)

// NewSchoolCalendar composes the calendar owner behind the legacy composition
// seam for graphs that do not record observations. Production roots compose
// the module themselves (api/base.go) so runtime evidence is kept. The
// period administration runs without gate, guard and federal state; test
// graphs that bind those use NewSchoolCalendarWithAdministration.
func NewSchoolCalendar(db transactionruntime.Database) (*schoolcalendar.Module, error) {
	return schoolCalendarCompose.New(schoolCalendarCompose.Dependencies{
		DB:      db.Pool(),
		Observe: func(schoolCalendarCompose.Observation) {},
	})
}
