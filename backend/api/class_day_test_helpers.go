//go:build testhooks

package api

import (
	"context"
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
	deliveryContract "github.com/moto-nrw/project-phoenix/modules/delivery/contract"
	"github.com/moto-nrw/project-phoenix/modules/documentrendering/lists"
	enrollmentCompose "github.com/moto-nrw/project-phoenix/modules/enrollment/compose"
	schoolCalendarCompose "github.com/moto-nrw/project-phoenix/modules/schoolcalendar/compose"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
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
	// Unreported records the notices about children missing without a
	// report (#4791) instead of handing them to Delivery.
	Unreported *RecordingUnreportedNotifier
}

// RecordingUnreportedNotifier keeps every notice about children missing
// without a report (#4791) as plain values.
type RecordingUnreportedNotifier struct {
	Reports []RecordedUnreportedNotice
}

// RecordedUnreportedNotice is one notice the class day handed to Delivery.
type RecordedUnreportedNotice struct {
	TenantID       int64
	Date           string
	StudentIDs     []int64
	ActorAccountID int64
}

// NotifyUnreportedAbsences records the report.
func (r *RecordingUnreportedNotifier) NotifyUnreportedAbsences(_ context.Context, report deliveryContract.UnreportedAbsenceReport) error {
	r.Reports = append(r.Reports, RecordedUnreportedNotice{
		TenantID: report.TenantID, Date: report.Date.String(), StudentIDs: report.StudentIDs, ActorAccountID: report.ActorAccountID,
	})
	return nil
}

func NewClassDayTestModule(db transactionruntime.Database, unit tenant.UnitOfWork, clocks ...func() time.Time) (ClassDayTestModule, error) {
	return NewClassDayTestModuleWithRealtime(db, unit, nil, clocks...)
}

// NewClassDayTestModuleWithRealtime lets integration tests observe the same
// hub that production binds, without collecting another owner on the module.
func NewClassDayTestModuleWithRealtime(db transactionruntime.Database, unit tenant.UnitOfWork, hub *realtime.Hub, clocks ...func() time.Time) (ClassDayTestModule, error) {
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
	people, err := owners.NewPeopleDirectory(db)
	if err != nil {
		return ClassDayTestModule{}, err
	}
	presence, err := NewCheckinTestPresence(db)
	if err != nil {
		return ClassDayTestModule{}, err
	}
	if hub == nil {
		hub = deliveryCompose.NewRealtimeHub(slog.Default())
	}
	members, err := owners.NewMembershipTestRepositories(db)
	if err != nil {
		return ClassDayTestModule{}, err
	}
	unreported := &RecordingUnreportedNotifier{}
	schoolDays, err := owners.BuildSchoolCalendarRecords(db, func(schoolCalendarCompose.Observation) {}, activeSettings.Settings)
	if err != nil {
		return ClassDayTestModule{}, err
	}
	classDay := NewClassDay(classdayCompose.ClassDayDependencies{
		Caller: activeCaller, PickupTimes: active.PickupSchedule, ArrivalTimes: active.ArrivalSchedule, CareDays: active.CareDay,
		Companions: owners.NewStudentCompanionRepository(rCare.CarePlan), ClassArrivalExceptions: active.ArrivalSchedule,
		BlockStarts:                   activeTimetable.TimetableOperations,
		MorningAttendanceDependencies: classDayTestMorning(db, hub, people, rCare, command, presence, unreported, clocks),
		TripStatusDays:                classDayTestTrips(db, hub, people, rCare, clocks),
		DayHintDependencies:           classdayCompose.DayHintDependencies{DayHints: people, Today: calendar.CalendarDateClock(clocks...)},
		HomeworkDependencies:          classdayCompose.HomeworkDependencies{Homework: NewClassDayHomework(owners.MustNewUnobservedTimetable(db))},
		ClassHandoverDependencies: classdayCompose.ClassHandoverDependencies{
			Handovers: presence, HandoverStaffNames: classHandoverStaffNames(members.Staff),
		},
		AbsenceListDependencies: NewClassDayAbsenceList(presence, rCare.StudentStatusDay, schoolDays, rAudit.DataAccessLog, lists.NewRenderer()),
	}, reports, rCare.StudentStatusDay, r.Student, r.Person, r.StudentGuardian, rAudit.DataAccessLog, activeSettings.Settings,
		hub, slog.Default())
	return ClassDayTestModule{ClassDay: classDay, Unreported: unreported}, nil
}

// classDayTestMorning is the morning attendance seam with the absence
// reports (#4791) over the test graph, composed like buildClassDayViews.
func classDayTestMorning(db transactionruntime.Database, hub *realtime.Hub, people *peopleCompose.Directory, rCare owners.TimetableCarePlanRows,
	command *auditCompose.Command, presence studentpresence.Owner, unreported *RecordingUnreportedNotifier, clocks []func() time.Time,
) classdayCompose.MorningAttendanceDependencies {
	return withClassDayAbsenceReports(classdayCompose.MorningAttendanceDependencies{Store: presence, Now: classDayTestClock(clocks)},
		presenceStudentsHTTP.NewSchoolAbsenceReports(presenceStudentsHTTP.ResourceConfig{
			Broadcaster: hub, Logger: slog.Default(), PeopleDirectory: people, StudentStatusDayService: classDayTestStatusDays(db, rCare, clocks),
		}, presence), owners.NewTestAuditPersistence(db, command).StudentFieldEdit, unreported)
}

// classDayTestTrips is the class-trip write seam (#4799) over the test graph,
// composed like buildClassDayViews composes it.
func classDayTestTrips(db transactionruntime.Database, hub *realtime.Hub, people *peopleCompose.Directory, rCare owners.TimetableCarePlanRows, clocks []func() time.Time) classdayCompose.ClassTripStatusDays {
	return NewClassDayTripStatusDays(presenceStudentsHTTP.NewSchoolClassTrips(presenceStudentsHTTP.ResourceConfig{
		Broadcaster: hub, Logger: slog.Default(), PeopleDirectory: people,
		StudentStatusDayService: classDayTestStatusDays(db, rCare, clocks),
	}))
}

// classDayTestStatusDays is the OGS's status-day write over the test graph.
func classDayTestStatusDays(db transactionruntime.Database, rCare owners.TimetableCarePlanRows, clocks []func() time.Time) studentpresence.StatusDays {
	return presenceservice.NewStatusDays(rCare.StudentStatusDay, owners.NewManualPartialAbsenceDates(rCare.CarePlan),
		db.Pool(), rCare.CarePlan.LockExceptionDay, clockOrNow(clocks))
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
