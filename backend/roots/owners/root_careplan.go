package owners

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	carePlanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	peoplePorts "github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	"github.com/moto-nrw/project-phoenix/modules/securityruntime"
	settingsCompose "github.com/moto-nrw/project-phoenix/modules/settings/compose"
	configModels "github.com/moto-nrw/project-phoenix/modules/settings/records"
	studentDirectoryView "github.com/moto-nrw/project-phoenix/modules/studentdirectoryview/compose"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	timetableCompose "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
)

// CarePlanRows are the adapters that bind Care Plan to the owners the root
// composed (#4011): the status days, the pickup students and locks, the
// pickup excusal's timetable reads, the arrival rules, the owners the care
// lifecycle drives around a care exit, the companion graph and the retained
// excused-absence request rows.
type CarePlanRows struct {
	StatusDays             *StudentStatusDayRepository
	PickupStudents         carePlanCompose.PickupBulkStudents
	PickupStudentLocks     carePlanCompose.PickupStudentLocks
	PickupExcusalTimetable PickupExcusalTimetable
	ArrivalRules           carePlanCompose.ArrivalScheduleRules
	ArrivalDependencies    carePlanCompose.ArrivalScheduleDependencies
	LifecycleOwners        carePlanCompose.CareLifecycleOwners
	Companions             careplan.StudentCompanions
	ExcusedRequestRows     ExcusedRequestReads
}

// buildCarePlanRows binds Care Plan to the owners the root composed. The
// class timetable supplies the regular arrival time of the arrival baselines
// and schedules (#2414, ADR 0005); the enrollment change requests'
// multi-child approval takes the companion lock order through the same
// companion graph.
func BuildCarePlanRows(
	db transactionruntime.Database, logger *slog.Logger, studentAudit peoplePorts.StudentAuditService,
	carePlan careplan.Capability, persons peopledirectory.Capability, membership schoolmembership.Capability,
	timetableOwner timetable.Capability, calendar schoolcalendar.Query, classArrivals timetable.ClassArrivals, people PeopleRows,
) (CarePlanRows, error) {
	classExceptions, err := NewClassArrivalExceptions(classArrivals, persons)
	if err != nil {
		return CarePlanRows{}, err
	}
	arrivalRules, arrivalDependencies, err := ArrivalScheduleBinding(persons, ClassArrivalPlans(classArrivals), classExceptions, logger.With("service", "arrival-schedule"))
	if err != nil {
		return CarePlanRows{}, err
	}
	pickupStudents, err := PickupPlanStudents(persons)
	if err != nil {
		return CarePlanRows{}, err
	}
	companions, err := carePlanCompose.NewCompanions(carePlan, NewCompanionStudents(people.Students, persons, studentAudit))
	if err != nil {
		return CarePlanRows{}, err
	}
	lifecycleOwners := NewCareLifecycleOwners(people.Students, people.Persons, membership, persons, timetableOwner, calendar,
		NewUnloggedStudentPresence(db), NewPresenceProjectionReads(db), NewEnrollmentBookingProjection(NewEnrollmentBookings()),
		NewCareEndRecorder(studentAudit))
	return CarePlanRows{
		StatusDays:             NewStudentStatusDayRepository(carePlan),
		PickupStudents:         pickupStudents,
		PickupStudentLocks:     PickupStudentLocks{Students: persons},
		PickupExcusalTimetable: NewPickupExcusalTimetable(timetableOwner, NewPartialAbsencePreview(db)),
		ArrivalRules:           arrivalRules,
		ArrivalDependencies:    arrivalDependencies,
		LifecycleOwners:        lifecycleOwners,
		Companions:             companions,
		ExcusedRequestRows:     NewExcusedAbsenceRequestRepository(carePlan),
	}, nil
}

// buildClassArrivals builds the class timetable's arrival plans the arrival
// baselines and schedules read; their queries log at debug level.
func BuildClassArrivals(db transactionruntime.Database, logger *slog.Logger) (timetable.ClassArrivals, error) {
	return timetableCompose.NewClassArrivals(db.Pool(), func(observation timetableCompose.Observation) {
		logger.Debug("class arrival query",
			"operation", observation.Operation,
			"duration", observation.Duration,
			"queries", observation.Stats.Queries,
			"rows", observation.Stats.Rows,
			"error", observation.Err)
	})
}

// CarePlanSchedules are Care Plan's care lifecycle and schedules: the pickup
// and arrival baselines, the care-day derivation (#1747), the pickup auto
// excusal (#2360, #3261), the pickup and arrival schedules and the partial
// absences.
type CarePlanSchedules struct {
	Lifecycle        careplan.CareLifecycle
	PickupBaselines  careplan.PickupBaselineReader
	ArrivalBaselines careplan.ArrivalBaselineReader
	CareDay          careplan.CareDayQuery
	AutoExcusal      careplan.PickupAutoExcusal
	Pickups          careplan.PickupScheduleService
	PartialAbsences  careplan.PartialAbsenceService
	Arrivals         careplan.ArrivalScheduleService
}

// bookingsAuthoritative reports whether the approved bookings supply the
// care days (enrollment.bookings_authoritative, #2414, ADR 0005).
func bookingsAuthoritative(settings settingsCompose.SettingsService) func(context.Context) (bool, error) {
	return func(ctx context.Context) (bool, error) {
		return settings.ResolveBool(ctx, configModels.KeyEnrollmentBookingsAuthoritative)
	}
}

// buildCarePlanSchedules builds the care lifecycle first, so the care days
// and the person directory receive its participation decision at
// construction, then the baselines, the care days and the schedules. The
// care days intersect the timetable assignments with the children's care
// plans and are shared by every reader.
func BuildCarePlanSchedules(
	db transactionruntime.Database, logger *slog.Logger, carePlan careplan.Capability, persons peopledirectory.Capability,
	rows CarePlanRows, classArrivals timetable.ClassArrivals, approved careplan.ApprovedBookingReader,
	settings settingsCompose.SettingsService, lockRecurrence func(context.Context) error, presence *studentpresence.Module,
) (CarePlanSchedules, error) {
	authoritative := bookingsAuthoritative(settings)
	lifecycle, err := carePlanCompose.NewCareLifecycle(carePlanCompose.CareLifecycleDependencies{
		StudentDirectory: studentDirectoryView.NewReader(),
		DB:               db.Pool(), Records: carePlan, Owners: rows.LifecycleOwners, LockCareBookingWrites: lockRecurrence,
		BookingsAuthoritative: authoritative, Fingerprint: securityruntime.Fingerprint,
		Logger: logger.With("service", "care_lifecycle"),
	})
	if err != nil {
		return CarePlanSchedules{}, fmt.Errorf("compose care lifecycle: %w", err)
	}
	pickupBaselines, err := carePlanCompose.NewPickupBaselines(carePlan, approved, authoritative)
	if err != nil {
		return CarePlanSchedules{}, err
	}
	// The arrival mirror: the class timetable supplies the regular time and,
	// with enrollment.bookings_authoritative on, the approved bookings supply
	// the care days, so a stale row on an unbooked weekday stops marking a
	// child expected.
	arrivalBaselines, err := NewArrivalBaselines(carePlan, persons, classArrivals, approved, authoritative)
	if err != nil {
		return CarePlanSchedules{}, err
	}
	careDay, err := carePlanCompose.NewCareDayQuery(carePlanCompose.CareDayDependencies{
		ArrivalBaselines: arrivalBaselines, Records: carePlan, PickupBaselines: pickupBaselines, CareParticipation: lifecycle,
	})
	if err != nil {
		return CarePlanSchedules{}, err
	}
	schedules := CarePlanSchedules{
		Lifecycle: lifecycle, PickupBaselines: pickupBaselines, ArrivalBaselines: arrivalBaselines, CareDay: careDay,
	}
	return buildCarePlanPlans(db, logger, carePlan, rows, presence, schedules)
}

// buildCarePlanPlans binds the pickup auto excusal, the pickup and arrival
// schedules and the partial absences over the baselines.
func buildCarePlanPlans(db transactionruntime.Database, logger *slog.Logger, carePlan careplan.Capability, rows CarePlanRows, presence *studentpresence.Module, schedules CarePlanSchedules) (CarePlanSchedules, error) {
	excusal, err := carePlanCompose.NewPickupAutoExcusal(carePlanCompose.PickupExcusalDependencies{
		DB: db.Pool(), Records: carePlan, Baselines: schedules.PickupBaselines, Blocks: presence,
		Preview: rows.PickupExcusalTimetable, Extensions: rows.PickupExcusalTimetable,
	})
	if err != nil {
		return CarePlanSchedules{}, err
	}
	pickups, err := carePlanCompose.NewPickupSchedules(db.Pool(), carePlan, schedules.PickupBaselines, excusal,
		rows.PickupStudents, logger.With("service", "pickup-schedule"))
	if err != nil {
		return CarePlanSchedules{}, err
	}
	partialAbsences, err := carePlanCompose.NewPartialAbsences(db.Pool(), carePlan, presence, excusal)
	if err != nil {
		return CarePlanSchedules{}, err
	}
	arrivals, err := carePlanCompose.NewArrivalSchedules(db.Pool(), carePlan, schedules.ArrivalBaselines, rows.ArrivalRules, rows.ArrivalDependencies)
	if err != nil {
		return CarePlanSchedules{}, err
	}
	schedules.AutoExcusal, schedules.Pickups, schedules.PartialAbsences, schedules.Arrivals = excusal, pickups, partialAbsences, arrivals
	return schedules, nil
}
