package services

import (
	"log/slog"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	deliveryCompose "github.com/moto-nrw/project-phoenix/modules/delivery/compose"
	auditSvc "github.com/moto-nrw/project-phoenix/services/audit"
	"github.com/moto-nrw/project-phoenix/services/enrollment"
	"github.com/moto-nrw/project-phoenix/services/schedule"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

type ClassDayTestModule struct {
	ActiveTestModule
	EnrollmentReport          enrollment.ReportService
	ClassDayArrivalExceptions enrollment.ClassDayArrivalExceptionService
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
	schedule.WireCareParticipation(active.CareDay, care.CareLifecycle)
	report := enrollment.NewReportService(enrollment.ReportServiceConfig{
		RequestRepo: r.Request, RequestChildRepo: r.RequestChild, RequestGuardianRepo: r.RequestGuardian,
		RequestChildOfferingRepo: r.RequestChildOffering, CareOfferingRepo: r.CareOffering, FormSchemaRepo: r.FormSchema,
		PhaseRepo: r.Phase, DataAccessLogRepo: r.DataAccessLog, StudentRepo: r.Student, StudentGuardianRepo: r.StudentGuardian,
		StudentCompanionRepo: r.StudentCompanion, PersonRepo: r.Person, EducationGroupRepo: r.Group, StudentStatusDayRepo: r.StudentStatusDay,
		ClassListEntryRepo: r.ClassListEntry, PickupScheduleSvc: active.PickupSchedule, ArrivalScheduleSvc: active.ArrivalSchedule,
		ClassArrivalExceptions: active.ArrivalSchedule, CareDaySvc: active.CareDay, Settings: active.Settings, CareParticipation: care.CareLifecycle,
	})
	exceptions := enrollment.NewClassDayArrivalExceptionService(enrollment.ClassDayArrivalExceptionConfig{
		ArrivalSchedule: active.ArrivalSchedule, Settings: active.Settings, BlockStarts: active.TimetableOperations,
		Broadcaster: deliveryCompose.NewRealtimeHub(slog.Default()), Logger: slog.Default(),
	})
	return ClassDayTestModule{ActiveTestModule: active, EnrollmentReport: report, ClassDayArrivalExceptions: exceptions}, nil
}
