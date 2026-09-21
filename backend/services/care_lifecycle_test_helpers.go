package services

import (
	"context"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	configModels "github.com/moto-nrw/project-phoenix/models/config"
	"github.com/moto-nrw/project-phoenix/modules/careplan/legacy/carelifecycle"
	"github.com/moto-nrw/project-phoenix/modules/timetable/legacy/timetableplanning"
	auditSvc "github.com/moto-nrw/project-phoenix/services/audit"
	"github.com/moto-nrw/project-phoenix/services/config"
	"github.com/moto-nrw/project-phoenix/services/users"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

type CareLifecycleTestModule struct {
	CareLifecycle carelifecycle.CareLifecycleService
	StudentAudit  users.StudentAuditService
	Settings      config.SettingsService
}

func NewCareLifecycleTestModule(db *bun.DB, unit tenant.UnitOfWork) (CareLifecycleTestModule, error) {
	settings, err := NewSettingsTestModule(db, unit)
	if err != nil {
		return CareLifecycleTestModule{}, err
	}
	command, err := auditSvc.NewCommand(repositories.NewTestAuditStore(db), func(auditSvc.AppendObservation) {})
	if err != nil {
		return CareLifecycleTestModule{}, err
	}
	r, err := repositories.NewCareLifecycleTestRepositories(db, command)
	if err != nil {
		return CareLifecycleTestModule{}, err
	}
	audit := users.NewStudentAuditService(requestAuditActor, repositories.NewStudentAudit(db))
	membership, err := repositories.NewSchoolMembership(db)
	if err != nil {
		return CareLifecycleTestModule{}, err
	}
	service := carelifecycle.NewCareLifecycleService(carelifecycle.CareLifecycleDependencies{
		StudentRepo: repositories.NewCareStudents(r.Student, membership), PersonRepo: r.Person, CareExitRepo: r.CareExit, CleanupRepo: r.CareExitCleanup,
		WithdrawalRepo: r.CareWithdrawal, TagReleaser: r.TagReleaser, AuditService: audit,
		LockCareBookingWrites: func(ctx context.Context) error { return timetableplanning.LockTenantRecurrenceWrites(ctx, db) },
		BookingsAuthoritative: func(ctx context.Context) (bool, error) {
			return settings.Settings.ResolveBool(ctx, configModels.KeyEnrollmentBookingsAuthoritative)
		},
		DB: db, Logger: slog.Default(),
	})
	return CareLifecycleTestModule{CareLifecycle: service, StudentAudit: audit, Settings: settings.Settings}, nil
}
