package services

import (
	"context"

	peopleCompose "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"
	settingsCompose "github.com/moto-nrw/project-phoenix/modules/settings/compose"

	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	configModels "github.com/moto-nrw/project-phoenix/models/config"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/securityruntime"
	auditSvc "github.com/moto-nrw/project-phoenix/services/audit"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

type CareLifecycleTestModule struct {
	CareLifecycle careplan.CareLifecycle
	StudentAudit  peopleCompose.StudentAuditService
	Settings      settingsCompose.SettingsService
}

func NewCareLifecycleTestModule(db *bun.DB, unit tenant.UnitOfWork) (CareLifecycleTestModule, error) {
	settings, err := NewSettingsTestModule(db, unit)
	if err != nil {
		return CareLifecycleTestModule{}, err
	}
	command, err := auditSvc.NewCommand(NewTestAuditStore(db), func(auditSvc.AppendObservation) {})
	if err != nil {
		return CareLifecycleTestModule{}, err
	}
	r, err := NewCareLifecycleTestRepositories(db, command)
	if err != nil {
		return CareLifecycleTestModule{}, err
	}
	audit := peopleCompose.NewStudentAuditService(requestAuditActor, NewStudentAudit(db))
	recurrenceLock, err := NewTimetableRecurrenceLock(db)
	if err != nil {
		return CareLifecycleTestModule{}, err
	}
	lifecycle, err := r.NewCareLifecycle(CareLifecycleTestConfig{
		Audit:                 audit,
		LockCareBookingWrites: recurrenceLock.LockRecurrenceWrites,
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
