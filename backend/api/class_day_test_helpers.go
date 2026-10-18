//go:build testhooks

package api

import (
	"log/slog"
	"time"

	classdayCompose "github.com/moto-nrw/project-phoenix/modules/classday/compose"

	schoolStructureCompose "github.com/moto-nrw/project-phoenix/modules/schoolstructure/compose"

	auditCompose "github.com/moto-nrw/project-phoenix/modules/auditlog/compose"
	careplanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
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
	rCare := NewEnrollmentCarePlanRows(db)
	rAudit := NewEnrollmentAuditRows(db, command)
	rMembership := NewSchoolMembershipSuiteRepositories(db)
	careplanCompose.WireCareParticipation(active.CareDay, care.CareLifecycle)
	reports := NewEnrollmentReports(EnrollmentReportSources{
		Owner: r.Enrollment(), Offerings: enrollmentCompose.NewCareOfferingRecords(rCare.CarePlan), AccessLog: rAudit.DataAccessLog,
		Students: r.Student, Persons: r.Person, Groups: schoolStructureCompose.NewGroupNames(NewSchoolStructureSuiteRepositories(db).Group), StudentGuardians: r.StudentGuardian,
		Companions: NewStudentCompanionRepository(rCare.CarePlan), ClassListEntries: NewClassListEntryRosterReader(rMembership.Membership),
		PickupSchedules: active.PickupSchedule, CareParticipation: care.CareLifecycle, Settings: active.Settings,
	})
	classDay := NewClassDay(classdayCompose.ClassDayDependencies{
		Caller: active.UserContext, PickupTimes: active.PickupSchedule, ArrivalTimes: active.ArrivalSchedule, CareDays: active.CareDay,
		Companions: NewStudentCompanionRepository(rCare.CarePlan), ClassArrivalExceptions: active.ArrivalSchedule,
		BlockStarts: active.TimetableOperations,
	}, reports, rCare.StudentStatusDay, r.Student, r.Person, r.StudentGuardian, rAudit.DataAccessLog, active.Settings,
		deliveryCompose.NewRealtimeHub(slog.Default()), slog.Default())
	return ClassDayTestModule{ActiveTestModule: active, EnrollmentReports: reports, ClassDay: classDay}, nil
}
