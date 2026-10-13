package services

import (
	"context"

	auditCompose "github.com/moto-nrw/project-phoenix/modules/auditlog/compose"
	peopleCompose "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"
	settingsCompose "github.com/moto-nrw/project-phoenix/modules/settings/compose"

	auditModels "github.com/moto-nrw/project-phoenix/modules/auditlog/compose/vocabulary"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	carePlanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	userModels "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose/vocabulary"
	"github.com/moto-nrw/project-phoenix/modules/securityruntime"
	configModels "github.com/moto-nrw/project-phoenix/modules/settings/compose/vocabulary"
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
	command, err := auditCompose.NewCommand(NewTestAuditStore(db), func(auditCompose.AppendObservation) {})
	if err != nil {
		return CareLifecycleTestModule{}, err
	}
	r, err := NewCareLifecycleTestRepositories(db, command)
	if err != nil {
		return CareLifecycleTestModule{}, err
	}
	audit := peopleCompose.NewStudentAuditService(requestAuditActor, NewStudentAudit(db))
	recurrenceLock, err := newTimetableTestRecurrenceLock(db)
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
	deps, err := studentDocumentDependencies(records, students, userContext, edits, accessLog)
	if err != nil {
		return nil, err
	}
	deps.DB = db
	return carePlanCompose.NewStudentDocuments(deps)
}
