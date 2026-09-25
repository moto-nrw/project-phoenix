package services

import (
	"log/slog"
	"time"

	careplanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/modules/classday"
	deliveryCompose "github.com/moto-nrw/project-phoenix/modules/delivery/compose"
	enrollmentOwner "github.com/moto-nrw/project-phoenix/modules/enrollment"
	auditSvc "github.com/moto-nrw/project-phoenix/services/audit"
	"github.com/moto-nrw/project-phoenix/services/enrollment"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

type ClassDayTestModule struct {
	ActiveTestModule
	EnrollmentReports enrollmentOwner.Reports
	// ClassDay is the school-portal capability composed the way the service
	// factory composes it.
	ClassDay classday.ClassDay
}

func NewClassDayTestModule(db *bun.DB, unit tenant.UnitOfWork, clocks ...func() time.Time) (ClassDayTestModule, error) {
	active, err := NewActiveTestModule(db, unit, clocks...)
	if err != nil {
		return ClassDayTestModule{}, err
	}
	care, err := NewCareLifecycleTestModule(db, unit)
	if err != nil {
		return ClassDayTestModule{}, err
	}
	command, err := auditSvc.NewCommand(repositories.NewTestAuditStore(db), func(auditSvc.AppendObservation) {})
	if err != nil {
		return ClassDayTestModule{}, err
	}
	r, err := repositories.NewEnrollmentTestRepositories(db, command)
	if err != nil {
		return ClassDayTestModule{}, err
	}
	careplanCompose.WireCareParticipation(active.CareDay, care.CareLifecycle)
	reports := newEnrollmentReports(enrollmentReportSources{
		Owner: r.Enrollment(), Offerings: enrollment.NewCareOfferingRepository(r.CarePlan), AccessLog: r.DataAccessLog,
		Students: r.Student, Persons: r.Person, Groups: repositories.NewGroupNames(r.Group), StudentGuardians: r.StudentGuardian,
		Companions: repositories.NewStudentCompanionRepository(r.CarePlan), ClassListEntries: NewClassListEntryRosterReader(r.Membership),
		PickupSchedules: active.PickupSchedule, CareParticipation: care.CareLifecycle, Settings: active.Settings,
	})
	classDay := newClassDay(classDaySources{
		Caller: active.UserContext, Reports: reports, StatusDays: r.StudentStatusDay,
		PickupTimes: active.PickupSchedule, ArrivalTimes: active.ArrivalSchedule, CareDays: active.CareDay,
		Companions: repositories.NewStudentCompanionRepository(r.CarePlan), Students: r.Student, Persons: r.Person,
		StudentGuardians: r.StudentGuardian, AccessLog: r.DataAccessLog, ClassArrivalExceptions: active.ArrivalSchedule,
		Settings: active.Settings, BlockStarts: active.TimetableOperations,
		Broadcaster: deliveryCompose.NewRealtimeHub(slog.Default()), Logger: slog.Default(),
	})
	return ClassDayTestModule{ActiveTestModule: active, EnrollmentReports: reports, ClassDay: classDay}, nil
}
