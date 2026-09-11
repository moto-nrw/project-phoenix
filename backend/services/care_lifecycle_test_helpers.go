package services

import (
	"context"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	configModels "github.com/moto-nrw/project-phoenix/models/config"
	auditSvc "github.com/moto-nrw/project-phoenix/services/audit"
	"github.com/moto-nrw/project-phoenix/services/config"
	"github.com/moto-nrw/project-phoenix/services/schedule"
	"github.com/moto-nrw/project-phoenix/services/users"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

type CareLifecycleTestModule struct {
	CareLifecycle users.CareLifecycleService
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
	audit := users.NewStudentAuditService(r.StudentFieldEdit, slog.Default())
	service := users.NewCareLifecycleService(users.CareLifecycleDependencies{
		StudentRepo: r.Student, PersonRepo: r.Person, CareExitRepo: r.CareExit, CleanupRepo: r.CareExitCleanup,
		WithdrawalRepo: r.CareWithdrawal, TagReleaser: r.GradeTransition, AuditService: audit,
		LockCareBookingWrites: func(ctx context.Context) error { return schedule.LockTenantRecurrenceWrites(ctx, db) },
		BookingsAuthoritative: func(ctx context.Context) (bool, error) {
			return settings.Settings.ResolveBool(ctx, configModels.KeyEnrollmentBookingsAuthoritative)
		},
		DB: db, Logger: slog.Default(),
	})
	return CareLifecycleTestModule{CareLifecycle: service, StudentAudit: audit, Settings: settings.Settings}, nil
}
