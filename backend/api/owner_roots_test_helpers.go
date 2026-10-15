package api

import (
	facilitiesModule "github.com/moto-nrw/project-phoenix/modules/facilities"
	facilitiesCompose "github.com/moto-nrw/project-phoenix/modules/facilities/compose"
	timetableCompose "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	reminder "github.com/moto-nrw/project-phoenix/workflows/reminderdelivery"
	reminderCompose "github.com/moto-nrw/project-phoenix/workflows/reminderdelivery/compose"
	shiftplansyncCompose "github.com/moto-nrw/project-phoenix/workflows/shiftplansync/compose"
)

// NewCalendarReminderCommand binds the existing notification adapters to the
// reminder workflow without constructing a repository or service factory.
func NewCalendarReminderCommand(db transactionruntime.Database, source CalendarReminderSource) reminder.Command {
	return reminderCompose.NewCommand(db.Pool(), CalendarReminderDelivery(source))
}

// NewFacilities composes the unobserved room owner for the test and CLI
// compositions that reach Facilities through the legacy composition.
func NewFacilities(db transactionruntime.Database) (facilitiesModule.Capability, error) {
	return facilitiesCompose.NewUnobserved(db.Pool())
}

// NewSickCascadeTimetableRows binds the sick cascade's Betreuungsplan rows to
// the retained repositories and its day lock to the day-wide staffing lock
// the Timetable owner's staffing writes take (#1840, #3551). The
// shift-plan-sync workflow tests compose the cascade with it too.
func NewSickCascadeTimetableRows(rows timetableCompose.OwnerRows, db transactionruntime.Database) shiftplansyncCompose.TimetableRows {
	return rows.SickCascadeRows(timetableCompose.SubstituteDayLock(db.Pool()))
}
