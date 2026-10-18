//go:build testhooks

package api

import (
	"context"
	"time"

	deliveryContract "github.com/moto-nrw/project-phoenix/modules/delivery/contract"

	deliveryCompose "github.com/moto-nrw/project-phoenix/modules/delivery/compose"
	facilitiesModule "github.com/moto-nrw/project-phoenix/modules/facilities"
	facilitiesCompose "github.com/moto-nrw/project-phoenix/modules/facilities/compose"
	facilitiesAdapter "github.com/moto-nrw/project-phoenix/modules/facilities/compose/repositoryadapter"
	timetableCompose "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	reminder "github.com/moto-nrw/project-phoenix/workflows/reminderdelivery"
	reminderCompose "github.com/moto-nrw/project-phoenix/workflows/reminderdelivery/compose"

	shiftplansyncCompose "github.com/moto-nrw/project-phoenix/workflows/shiftplansync/compose"
)

// NewCalendarReminderCommand binds the existing notification adapters to the
// reminder workflow without constructing a repository or service factory.
func NewCalendarReminderCommand(db transactionruntime.Database, source CalendarReminderSource) reminder.Command {
	runtime, err := deliveryCompose.NewModule(deliveryCompose.ModuleDependencies{
		DB: db.Pool(), People: enrollmentFlowGuardianDirectory{}, Observe: func(deliveryContract.Observation) {},
	})
	if err != nil {
		panic(err)
	}
	return reminderCompose.NewCommand(reminderCommandRuntime(db), reminderDeliveryDependencies(db, runtime, source))
}

// NewFacilities composes the unobserved room owner for the test and CLI
// compositions that reach Facilities through the root.
func NewFacilities(db transactionruntime.Database) (facilitiesModule.Capability, error) {
	return facilitiesCompose.NewUnobserved(db.Pool())
}

// NewFacilitiesRoomRepository binds the retained room repository adapter to
// a fresh unobserved Facilities owner, for the shared fixture package's
// tenant isolation suite.
func NewFacilitiesRoomRepository(db transactionruntime.Database) (*facilitiesAdapter.Repository, error) {
	rooms, err := NewFacilities(db)
	if err != nil {
		return nil, err
	}
	return facilitiesCompose.NewRoomRepository(rooms), nil
}

// NewSickCascadeTimetableRows binds the sick cascade's Betreuungsplan rows to
// the retained repositories and its day lock to the day-wide staffing lock
// the Timetable owner's staffing writes take (#1840, #3551). The
// shift-plan-sync workflow tests compose the cascade with it too.
func NewSickCascadeTimetableRows(rows timetableCompose.OwnerRows, db transactionruntime.Database) shiftplansyncCompose.TimetableRows {
	return rows.SickCascadeRows(timetableCompose.SubstituteDayLock(db.Pool()))
}

// NewCalendarReminderOutboxFixture exposes preparation and the real worker so
// transaction and process-death tests can omit the post-commit continuation.
func NewCalendarReminderOutboxFixture(db transactionruntime.Database, source CalendarReminderSource) (func(context.Context, time.Time, time.Time) (int, func(context.Context) error, error), func(context.Context) error) {
	runtime, err := deliveryCompose.NewModule(deliveryCompose.ModuleDependencies{DB: db.Pool(), People: enrollmentFlowGuardianDirectory{}, Observe: func(deliveryContract.Observation) {}})
	if err != nil {
		panic(err)
	}
	deps := reminderDeliveryDependencies(db, runtime, source)
	provider := reminderPushProvider{deliver: reminderCompose.NewPushDelivery(reminderCommandRuntime(db), deps)}
	worker, err := deliveryCompose.NewWorker(runtime, provider)
	if err != nil {
		panic(err)
	}
	return reminderCompose.NewPreparation(reminderCommandRuntime(db), deps).PrepareAppointmentReminders, func(ctx context.Context) error { _, err := worker.RunOnce(ctx, 2, 6); return err }
}
