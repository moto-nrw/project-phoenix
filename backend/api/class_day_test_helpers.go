package api

import (
	"log/slog"
	"time"

	schoolStructureCompose "github.com/moto-nrw/project-phoenix/modules/schoolstructure/compose"

	auditCompose "github.com/moto-nrw/project-phoenix/modules/auditlog/compose"
	careplanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	"github.com/moto-nrw/project-phoenix/modules/classday"
	deliveryCompose "github.com/moto-nrw/project-phoenix/modules/delivery/compose"
	enrollmentOwner "github.com/moto-nrw/project-phoenix/modules/enrollment"
	enrollmentCompose "github.com/moto-nrw/project-phoenix/modules/enrollment/compose"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	"github.com/moto-nrw/project-phoenix/services"
	"github.com/moto-nrw/project-phoenix/tenant"
)

type ClassDayTestModule struct {
	ActiveTestModule
	EnrollmentReports enrollmentOwner.Reports
	// ClassDay is the school-portal capability composed the way the service
	// factory composes it.
	ClassDay classday.ClassDay
}

func NewClassDayTestModule(db transactionruntime.Database, unit tenant.UnitOfWork, clocks ...func() time.Time) (ClassDayTestModule, error) {
	active, err := NewActiveTestModule(db, unit, clocks...)
	if err != nil {
		return ClassDayTestModule{}, err
	}
	care, err := NewCareLifecycleTestModule(db, unit)
	if err != nil {
		return ClassDayTestModule{}, err
	}
	command, err := auditCompose.NewCommand(NewTestAuditStore(db), func(auditCompose.AppendObservation) {})
	if err != nil {
		return ClassDayTestModule{}, err
	}
	r, err := NewEnrollmentTestRepositories(db, command)
	if err != nil {
		return ClassDayTestModule{}, err
	}
	careplanCompose.WireCareParticipation(active.CareDay, care.CareLifecycle)
	reports := services.NewEnrollmentReports(services.EnrollmentReportSources{
		Owner: r.Enrollment(), Offerings: enrollmentCompose.NewCareOfferingRecords(r.CarePlan), AccessLog: r.DataAccessLog,
		Students: r.Student, Persons: r.Person, Groups: schoolStructureCompose.NewGroupNames(r.Group), StudentGuardians: r.StudentGuardian,
		Companions: services.NewStudentCompanionRepository(r.CarePlan), ClassListEntries: services.NewClassListEntryRosterReader(r.Membership),
		PickupSchedules: active.PickupSchedule, CareParticipation: care.CareLifecycle, Settings: active.Settings,
	})
	classDay := services.NewClassDay(services.ClassDaySources{
		Caller: active.UserContext, Reports: reports, StatusDays: r.StudentStatusDay,
		PickupTimes: active.PickupSchedule, ArrivalTimes: active.ArrivalSchedule, CareDays: active.CareDay,
		Companions: services.NewStudentCompanionRepository(r.CarePlan), Students: r.Student, Persons: r.Person,
		StudentGuardians: r.StudentGuardian, AccessLog: r.DataAccessLog, ClassArrivalExceptions: active.ArrivalSchedule,
		Settings: active.Settings, BlockStarts: active.TimetableOperations,
		Broadcaster: deliveryCompose.NewRealtimeHub(slog.Default()), Logger: slog.Default(),
	})
	return ClassDayTestModule{ActiveTestModule: active, EnrollmentReports: reports, ClassDay: classDay}, nil
}
