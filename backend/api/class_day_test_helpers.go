//go:build testhooks

package api

import (
	"log/slog"
	"time"

	owners "github.com/moto-nrw/project-phoenix/roots/owners"

	classdayCompose "github.com/moto-nrw/project-phoenix/modules/classday/compose"

	schoolStructureCompose "github.com/moto-nrw/project-phoenix/modules/schoolstructure/compose"

	auditCompose "github.com/moto-nrw/project-phoenix/modules/auditlog/compose"
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
	activeTimetable, _ := NewActiveTestTimetable(db, unit, clocks...)
	care, _, err := NewCareLifecycleTestModule(db, unit)
	if err != nil {
		return ClassDayTestModule{}, err
	}
	command, err := auditCompose.NewCommand(owners.NewTestAuditStore(db), func(auditCompose.AppendObservation) {})
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
	reports := owners.NewEnrollmentReports(owners.EnrollmentReportSources{
		Owner: r.Enrollment(), Offerings: enrollmentCompose.NewCareOfferingRecords(rCare.CarePlan), AccessLog: rAudit.DataAccessLog,
		Students: r.Student, Persons: r.Person, Groups: schoolStructureCompose.NewGroupNames(owners.NewSchoolStructureSuiteRepositories(db).Group), StudentGuardians: r.StudentGuardian,
		Companions: owners.NewStudentCompanionRepository(rCare.CarePlan), ClassListEntries: owners.NewClassListEntryRosterReader(rMembership.Membership),
		PickupSchedules: active.PickupSchedule, CareParticipation: care.CareLifecycle, Settings: activeSettings.Settings,
	})
	presence, err := NewCheckinTestPresence(db)
	if err != nil {
		return ClassDayTestModule{}, err
	}
	classDay := NewClassDay(classdayCompose.ClassDayDependencies{
		Caller: activeCaller, PickupTimes: active.PickupSchedule, ArrivalTimes: active.ArrivalSchedule, CareDays: active.CareDay,
		Companions: owners.NewStudentCompanionRepository(rCare.CarePlan), ClassArrivalExceptions: active.ArrivalSchedule,
		BlockStarts:                   activeTimetable.TimetableOperations,
		MorningAttendanceDependencies: classdayCompose.MorningAttendanceDependencies{Store: presence, Now: classDayTestClock(clocks)},
		HomeworkDependencies:          classdayCompose.HomeworkDependencies{Homework: NewClassDayHomework(owners.MustNewUnobservedTimetable(db))},
	}, reports, rCare.StudentStatusDay, r.Student, r.Person, r.StudentGuardian, rAudit.DataAccessLog, activeSettings.Settings,
		deliveryCompose.NewRealtimeHub(slog.Default()), slog.Default())
	return ClassDayTestModule{ClassDay: classDay}, nil
}

// classDayTestClock is the first clock a test passed, nil for the wall clock.
func classDayTestClock(clocks []func() time.Time) func() time.Time {
	if len(clocks) == 0 {
		return nil
	}
	return clocks[0]
}
