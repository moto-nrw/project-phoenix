package api

import (
	"fmt"

	owners "github.com/moto-nrw/project-phoenix/roots/owners"

	realtime "github.com/moto-nrw/project-phoenix/realtime"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	arrivalTimetable "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
	timetablePorts "github.com/moto-nrw/project-phoenix/modules/timetable/ports"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
)

// TimetablePlanning are the Timetable owner's planning reads and writes: the
// planner's data, the template writes (#3424 slice S2) including "Dieser
// und alle folgenden", the series conversion and the attendance
// corrections.
type TimetablePlanning struct {
	Planning        arrivalTimetable.Planning
	SeriesConverter timetable.InstanceSeriesConversion
}

// buildTimetablePlanning binds the planning over the lifecycle. A split
// caps the old template, creates a successor and re-plans the window
// through the materialization, keeping the Vertretungsplan overrides;
// Enrollment's roster resync reconciles offering-sourced rosters (#2137,
// #2147).
func buildTimetablePlanning(
	db transactionruntime.Database, env owners.RootEnvironment, hub *realtime.Hub, owner owners.TimetableOwner, lifecycle owners.TimetableLifecycle,
	schedules owners.CarePlanSchedules, presence owners.PresenceOwner, timetableOwner timetable.Capability, offerings owners.CarePlanOfferings,
	audit owners.AuditOwner, people owners.PeopleRows,
) (TimetablePlanning, error) {
	data, err := NewTimetableData(lifecycle.Rows, schedules.ArrivalBaselines, schedules.PickupBaselines,
		presence.Presence, timetableOwner, presence.SessionRecords, timetableOwner,
		owner.BlockMetadata,
		true, env.Logger.With("service", "timetable-data"))
	if err != nil {
		return TimetablePlanning{}, fmt.Errorf("compose timetable data: %w", err)
	}
	templateDeps, err := TimetableTemplateDependencies(owner.TemplateRows, owner.PlanningTracks, lifecycle.Materialization,
		lifecycle.Instances.SeriesDeviations(), offerings.Catalog, timetableOfferingRosterResync(offerings.Bookings),
		owner.RecurrenceLock, hub, env.Logger.With("service", "timetable-templates"), env.Today)
	if err != nil {
		return TimetablePlanning{}, fmt.Errorf("compose timetable templates: %w", err)
	}
	templateDeps.Transactions = arrivalTimetable.NewTenantTransactions(db.Pool())
	templates, err := arrivalTimetable.NewTemplateAdministration(templateDeps)
	if err != nil {
		return TimetablePlanning{}, fmt.Errorf("compose timetable templates: %w", err)
	}
	converter, err := arrivalTimetable.NewInstanceSeriesConversion(timetablePorts.InstanceSeriesConversionDependencies{
		Transactions: arrivalTimetable.NewTenantTransactions(db.Pool()), InstanceRepo: owner.Repositories.ActivityInstance, Lifecycle: lifecycle.Instances,
		Templates: templates, RecurrenceLock: owner.RecurrenceLock,
	})
	if err != nil {
		return TimetablePlanning{}, fmt.Errorf("compose timetable series conversion: %w", err)
	}
	corrections, err := NewTimetableAttendanceCorrections(lifecycle.Rows, audit.Persistence.AttendanceCorrection,
		timetableCorrectionPeople{rows: people.Persons}, env.Logger.With("service", "attendance-correction"))
	if err != nil {
		return TimetablePlanning{}, fmt.Errorf("compose timetable attendance corrections: %w", err)
	}
	return TimetablePlanning{
		Planning: arrivalTimetable.Planning{
			Templates: templates, RecurrenceLock: owner.RecurrenceLock, Data: data,
			ConflictDetection: lifecycle.Conflicts, AttendanceCorrections: corrections, Deviations: lifecycle.Deviations,
		},
		SeriesConverter: converter,
	}, nil
}
