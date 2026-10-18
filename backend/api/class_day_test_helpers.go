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
	enrollmentCompose "github.com/moto-nrw/project-phoenix/modules/enrollment/compose"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// ClassDayTestModule is the Class Day view over the active graph; the
// graph's owners come from the NewActiveTest* constructors (#4080).
type ClassDayTestModule struct {
	// ClassDay is the school-portal capability composed the way the service
	// factory composes it.
	ClassDay classday.ClassDay
}

func NewClassDayTestModule(db transactionruntime.Database, unit tenant.UnitOfWork, clocks ...func() time.Time) (ClassDayTestModule, error) {
	active, err := NewActiveTestModule(db, unit, clocks...)
	if err != nil {
		return ClassDayTestModule{}, err
	}
	activeSettings := NewActiveTestSettings(db, unit, clocks...)
	activeCaller := NewActiveTestCaller(db, unit, clocks...)
	activeTimetable := NewActiveTestTimetable(db, unit, clocks...)
	care, _, err := NewCareLifecycleTestModule(db, unit)
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
		PickupSchedules: active.PickupSchedule, CareParticipation: care.CareLifecycle, Settings: activeSettings.Settings,
	})
	classDay := NewClassDay(classdayCompose.ClassDayDependencies{
		Caller: activeCaller, PickupTimes: active.PickupSchedule, ArrivalTimes: active.ArrivalSchedule, CareDays: active.CareDay,
		Companions: NewStudentCompanionRepository(rCare.CarePlan), ClassArrivalExceptions: active.ArrivalSchedule,
		BlockStarts: activeTimetable.TimetableOperations,
	}, reports, rCare.StudentStatusDay, r.Student, r.Person, r.StudentGuardian, rAudit.DataAccessLog, activeSettings.Settings,
		deliveryCompose.NewRealtimeHub(slog.Default()), slog.Default())
	return ClassDayTestModule{ClassDay: classDay}, nil
}
