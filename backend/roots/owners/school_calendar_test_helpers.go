//go:build testhooks

package owners

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar"
	schoolCalendarCompose "github.com/moto-nrw/project-phoenix/modules/schoolcalendar/compose"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
)

// NewSchoolCalendar composes the calendar owner for graphs that do not record observations. Production roots compose
// the module themselves so runtime evidence is kept. The period
// administration runs without gate and guard and the tenant holiday reads
// without a federal state; test graphs that bind those use
// NewSchoolCalendarWithAdministration.
func NewSchoolCalendar(db transactionruntime.Database) (*schoolcalendar.Module, error) {
	observe := func(schoolCalendarCompose.Observation) {}
	records, err := schoolCalendarCompose.NewRecords(schoolCalendarCompose.RecordsDependencies{
		DB: db.Pool(), Observe: observe,
		FederalState: func(context.Context) (string, error) {
			return "", errors.New("school calendar: federal state resolver is not bound")
		},
	})
	if err != nil {
		return nil, err
	}
	return schoolCalendarCompose.NewAdministration(records, schoolCalendarCompose.AdministrationDependencies{
		DB: db.Pool(), Observe: observe, RecurrenceGate: func(context.Context) error { return nil },
		CareOfferingGuard: func(context.Context, int64, *schoolcalendar.CalendarPeriodFields) error { return nil },
	})
}
