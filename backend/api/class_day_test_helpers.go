//go:build testhooks

package api

import (
	"log/slog"
	"time"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"

	owners "github.com/moto-nrw/project-phoenix/roots/owners"

	classdayCompose "github.com/moto-nrw/project-phoenix/modules/classday/compose"
	peopleCompose "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"
	"github.com/moto-nrw/project-phoenix/realtime"

	schoolStructureCompose "github.com/moto-nrw/project-phoenix/modules/schoolstructure/compose"

	auditCompose "github.com/moto-nrw/project-phoenix/modules/auditlog/compose"
	"github.com/moto-nrw/project-phoenix/modules/classday"
	deliveryCompose "github.com/moto-nrw/project-phoenix/modules/delivery/compose"
	enrollmentOwner "github.com/moto-nrw/project-phoenix/modules/enrollment"
	enrollmentCompose "github.com/moto-nrw/project-phoenix/modules/enrollment/compose"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence/compose/presenceservice"
	presenceStudentsHTTP "github.com/moto-nrw/project-phoenix/modules/studentpresence/http"
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
	reports := classDayTestReports(db, r, rCare, rAudit, active, care, activeSettings)
	people, err := owners.NewPeopleDirectory(db)
	if err != nil {
		return ClassDayTestModule{}, err
	}
	presence, err := NewCheckinTestPresence(db)
	if err != nil {
		return ClassDayTestModule{}, err
	}
	hub := deliveryCompose.NewRealtimeHub(slog.Default())
	members, err := owners.NewMembershipTestRepositories(db)
	if err != nil {
		return ClassDayTestModule{}, err
	}
	_, staffGroups, err := NewUserContextTestRepositories(db, members)
	if err != nil {
		return ClassDayTestModule{}, err
	}
	classDay := NewClassDay(classdayCompose.ClassDayDependencies{
		Caller: activeCaller, PickupTimes: active.PickupSchedule, ArrivalTimes: active.ArrivalSchedule, CareDays: active.CareDay,
		Companions: owners.NewStudentCompanionRepository(rCare.CarePlan), ClassArrivalExceptions: active.ArrivalSchedule,
		BlockStarts:                   activeTimetable.TimetableOperations,
		MorningAttendanceDependencies: classdayCompose.MorningAttendanceDependencies{Store: presence, Now: classDayTestClock(clocks)},
		TripStatusDays:                classDayTestTrips(db, hub, people, rCare, clocks),
		StudentNoteDependencies: classdayCompose.StudentNoteDependencies{
			DayHints: people, Today: calendar.CalendarDateClock(clocks...), SharedNotes: people, ClassTeachers: staffGroups,
		},
		HomeworkDependencies: classdayCompose.HomeworkDependencies{Homework: NewClassDayHomework(owners.MustNewUnobservedTimetable(db))},
		ClassHandoverDependencies: classdayCompose.ClassHandoverDependencies{
			Handovers: presence, HandoverStaffNames: classHandoverStaffNames(members.Staff),
		},
	}, reports, rCare.StudentStatusDay, r.Student, r.Person, r.StudentGuardian, rAudit.DataAccessLog, activeSettings.Settings,
		hub, slog.Default())
	return ClassDayTestModule{ClassDay: classDay}, nil
}

// classDayTestReports is Enrollment's day roster over the test graph, composed
// like the service factory composes it.
func classDayTestReports(
	db transactionruntime.Database, r EnrollmentTestRepositories, rCare owners.TimetableCarePlanRows, rAudit *EnrollmentAuditRows,
	active ActiveTestModule, care CareLifecycleTestModule, activeSettings ActiveSettings,
) enrollmentOwner.Reports {
	rMembership := NewSchoolMembershipSuiteRepositories(db)
	return owners.NewEnrollmentReports(owners.EnrollmentReportSources{
		Owner: r.Enrollment(), Offerings: enrollmentCompose.NewCareOfferingRecords(rCare.CarePlan), AccessLog: rAudit.DataAccessLog,
		Students: r.Student, Persons: r.Person, Groups: schoolStructureCompose.NewGroupNames(owners.NewSchoolStructureSuiteRepositories(db).Group), StudentGuardians: r.StudentGuardian,
		Companions: owners.NewStudentCompanionRepository(rCare.CarePlan), ClassListEntries: owners.NewClassListEntryRosterReader(rMembership.Membership),
		PickupSchedules: active.PickupSchedule, CareParticipation: care.CareLifecycle, Settings: activeSettings.Settings,
	})
}

// classDayTestTrips is the class-trip write seam (#4799) over the test graph,
// composed like buildClassDayViews composes it.
func classDayTestTrips(db transactionruntime.Database, hub *realtime.Hub, people *peopleCompose.Directory, rCare owners.TimetableCarePlanRows, clocks []func() time.Time) classdayCompose.ClassTripStatusDays {
	return NewClassDayTripStatusDays(presenceStudentsHTTP.NewSchoolClassTrips(presenceStudentsHTTP.ResourceConfig{
		Broadcaster: hub, Logger: slog.Default(), PeopleDirectory: people,
		StudentStatusDayService: presenceservice.NewStatusDays(rCare.StudentStatusDay, owners.NewManualPartialAbsenceDates(rCare.CarePlan),
			db.Pool(), rCare.CarePlan.LockExceptionDay, clockOrNow(clocks)),
	}))
}

// clockOrNow is the first clock a test passed, or the wall clock.
func clockOrNow(clocks []func() time.Time) func() time.Time {
	if clock := classDayTestClock(clocks); clock != nil {
		return clock
	}
	return time.Now
}

// classDayTestClock is the first clock a test passed, nil for the wall clock.
func classDayTestClock(clocks []func() time.Time) func() time.Time {
	if len(clocks) == 0 {
		return nil
	}
	return clocks[0]
}
