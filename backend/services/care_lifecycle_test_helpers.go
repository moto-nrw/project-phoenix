package services

import (
	"context"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	configModels "github.com/moto-nrw/project-phoenix/models/config"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/securityruntime"
	"github.com/moto-nrw/project-phoenix/modules/timetable/legacy/timetableplanning"
	auditSvc "github.com/moto-nrw/project-phoenix/services/audit"
	"github.com/moto-nrw/project-phoenix/services/config"
	"github.com/moto-nrw/project-phoenix/services/users"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

type CareLifecycleTestModule struct {
	CareLifecycle careplan.CareLifecycle
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
	lifecycle, err := r.NewCareLifecycle(repositories.CareLifecycleTestConfig{
		Audit:                 audit,
		LockCareBookingWrites: func(ctx context.Context) error { return timetableplanning.LockTenantRecurrenceWrites(ctx, db) },
		BookingsAuthoritative: func(ctx context.Context) (bool, error) {
			return settings.Settings.ResolveBool(ctx, configModels.KeyEnrollmentBookingsAuthoritative)
		},
	})
	if err != nil {
		return CareLifecycleTestModule{}, err
	}
	return CareLifecycleTestModule{CareLifecycle: lifecycle, StudentAudit: audit, Settings: settings.Settings}, nil
}

// NewStudentDocumentsTestModule composes the production child document
// capability over the given collaborators, so a suite can stand in the
// caller's staff identity or leave an audit repository out.
func NewStudentDocumentsTestModule(
	db *bun.DB, records careplan.Capability, students userModels.StudentRepository,
	userContext securityruntime.StudentAccessUserContext,
	edits auditModels.StudentFieldEditRepository, accessLog auditModels.DataAccessLogRepository,
) (careplan.StudentDocuments, error) {
	return newStudentDocuments(db, records, students, userContext, edits, accessLog)
}
