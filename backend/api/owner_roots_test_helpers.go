package api

import (
	enrollmentCompose "github.com/moto-nrw/project-phoenix/modules/enrollment/compose"
	facilitiesModule "github.com/moto-nrw/project-phoenix/modules/facilities"
	facilitiesCompose "github.com/moto-nrw/project-phoenix/modules/facilities/compose"
	timetableCompose "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	"github.com/moto-nrw/project-phoenix/services"
	reminder "github.com/moto-nrw/project-phoenix/workflows/reminderdelivery"
	reminderCompose "github.com/moto-nrw/project-phoenix/workflows/reminderdelivery/compose"
	shiftplansyncCompose "github.com/moto-nrw/project-phoenix/workflows/shiftplansync/compose"
)

// NewCalendarReminderCommand binds the existing notification adapters to the
// reminder workflow without constructing a repository or service factory.
func NewCalendarReminderCommand(db transactionruntime.Database, source services.CalendarReminderSource) reminder.Command {
	return reminderCompose.NewCommand(db.Pool(), services.CalendarReminderDelivery(source))
}

// NewFacilities composes the unobserved room owner for the test and CLI
// compositions that reach Facilities through the legacy composition.
func NewFacilities(db transactionruntime.Database) (facilitiesModule.Capability, error) {
	return facilitiesCompose.NewUnobserved(db.Pool())
}

// NewManualPlanningQuery forwards to Enrollment's manual-planning query for
// the Care Plan contract suites, which may not import Enrollment's
// composition; the legacy composition calls the owner directly.
func NewManualPlanningQuery(db transactionruntime.Database) *enrollmentCompose.ManualPlanningQuery {
	return enrollmentCompose.NewManualPlanningQuery(db.Pool())
}

// NewSickCascadeTimetableRows binds the sick cascade's Betreuungsplan rows to
// the retained repositories and its day lock to the day-wide staffing lock
// the Timetable owner's staffing writes take (#1840, #3551). The
// shift-plan-sync workflow tests compose the cascade with it too.
func NewSickCascadeTimetableRows(rows timetableCompose.OwnerRows, db transactionruntime.Database) shiftplansyncCompose.TimetableRows {
	return rows.SickCascadeRows(timetableCompose.SubstituteDayLock(db.Pool()))
}
