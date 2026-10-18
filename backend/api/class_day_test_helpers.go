//go:build testhooks

package api

import (
	"log/slog"
	"time"

	classdayCompose "github.com/moto-nrw/project-phoenix/modules/classday/compose"

	schoolStructureCompose "github.com/moto-nrw/project-phoenix/modules/schoolstructure/compose"

	auditCompose "github.com/moto-nrw/project-phoenix/modules/auditlog/compose"
	"github.com/moto-nrw/project-phoenix/modules/classday"
	deliveryCompose "github.com/moto-nrw/project-phoenix/modules/delivery/compose"
	enrollmentOwner "github.com/moto-nrw/project-phoenix/modules/enrollment"
	enrollmentCompose "github.com/moto-nrw/project-phoenix/modules/enrollment/compose"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
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
	reports := NewEnrollmentReports(EnrollmentReportSources{
		Owner: r.Enrollment(), Offerings: enrollmentCompose.NewCareOfferingRecords(r.CarePlan), AccessLog: r.DataAccessLog,
		Students: r.Student, Persons: r.Person, Groups: schoolStructureCompose.NewGroupNames(r.Group), StudentGuardians: r.StudentGuardian,
		Companions: NewStudentCompanionRepository(r.CarePlan), ClassListEntries: NewClassListEntryRosterReader(r.Membership),
		PickupSchedules: active.PickupSchedule, CareParticipation: care.CareLifecycle, Settings: active.Settings,
	})
	classDay := NewClassDay(classdayCompose.ClassDayDependencies{
		Caller: active.UserContext, PickupTimes: active.PickupSchedule, ArrivalTimes: active.ArrivalSchedule, CareDays: active.CareDay,
		Companions: NewStudentCompanionRepository(r.CarePlan), ClassArrivalExceptions: active.ArrivalSchedule,
		BlockStarts: active.TimetableOperations,
	}, reports, r.StudentStatusDay, r.Student, r.Person, r.StudentGuardian, r.DataAccessLog, active.Settings,
		deliveryCompose.NewRealtimeHub(slog.Default()), slog.Default())
	return ClassDayTestModule{ActiveTestModule: active, EnrollmentReports: reports, ClassDay: classDay}, nil
}
