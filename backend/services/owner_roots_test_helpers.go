package services

import (
	"github.com/uptrace/bun"

	enrollmentCompose "github.com/moto-nrw/project-phoenix/modules/enrollment/compose"
	facilitiesModule "github.com/moto-nrw/project-phoenix/modules/facilities"
	facilitiesCompose "github.com/moto-nrw/project-phoenix/modules/facilities/compose"
	timetableCompose "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
	reminder "github.com/moto-nrw/project-phoenix/workflows/reminderdelivery"
	reminderCompose "github.com/moto-nrw/project-phoenix/workflows/reminderdelivery/compose"
	shiftplansyncCompose "github.com/moto-nrw/project-phoenix/workflows/shiftplansync/compose"
)

// NewCalendarReminderCommand binds the existing notification adapters to the
// reminder workflow without constructing a repository or service factory.
func NewCalendarReminderCommand(db *bun.DB, source CalendarReminderSource) reminder.Command {
	return reminderCompose.NewCommand(db, calendarReminderDelivery(source))
}

// NewFacilities composes the unobserved room owner for the test and CLI
// compositions that reach Facilities through the legacy composition.
func NewFacilities(db *bun.DB) (facilitiesModule.Capability, error) {
	return facilitiesCompose.NewUnobserved(db)
}

// NewManualPlanningQuery forwards to Enrollment's manual-planning query for
// the Care Plan contract suites, which may not import Enrollment's
// composition; the legacy composition calls the owner directly.
func NewManualPlanningQuery(db *bun.DB) *enrollmentCompose.ManualPlanningQuery {
	return enrollmentCompose.NewManualPlanningQuery(db)
}

// NewSickCascadeTimetableRows binds the sick cascade's Betreuungsplan rows to
// the retained repositories and its day lock to the day-wide staffing lock
// the Timetable owner's staffing writes take (#1840, #3551). The
// shift-plan-sync workflow tests compose the cascade with it too.
func NewSickCascadeTimetableRows(rows timetableCompose.OwnerRows, db *bun.DB) shiftplansyncCompose.TimetableRows {
	return rows.SickCascadeRows(timetableCompose.SubstituteDayLock(db))
}
