//go:build testhooks

package api

import (
	"context"

	auditCompose "github.com/moto-nrw/project-phoenix/modules/auditlog/compose"
	auditModels "github.com/moto-nrw/project-phoenix/modules/auditlog/records"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	carePlanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	peopleCompose "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"
	peoplePorts "github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
	"github.com/moto-nrw/project-phoenix/modules/securityruntime"
	settingsCompose "github.com/moto-nrw/project-phoenix/modules/settings/compose"
	configModels "github.com/moto-nrw/project-phoenix/modules/settings/compose/vocabulary"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	"github.com/moto-nrw/project-phoenix/tenant"
)

type CareLifecycleTestModule struct {
	CareLifecycle careplan.CareLifecycle
	StudentAudit  peoplePorts.StudentAuditService
	Settings      settingsCompose.SettingsService
}

func NewCareLifecycleTestModule(db transactionruntime.Database, unit tenant.UnitOfWork) (CareLifecycleTestModule, error) {
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
	audit := peopleCompose.NewStudentAuditService(requestTokenReads.AuditActor, NewStudentAudit(db))
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
	db transactionruntime.Database, records careplan.Capability, students peoplePorts.StudentRepository,
	userContext securityruntime.StudentAccessUserContext,
	edits auditModels.StudentFieldEditRepository, accessLog auditModels.DataAccessLogRepository,
) (careplan.StudentDocuments, error) {
	deps, err := StudentDocumentDependencies(records, students, userContext, edits, accessLog)
	if err != nil {
		return nil, err
	}
	deps.DB = db.Pool()
	return carePlanCompose.NewStudentDocuments(deps)
}
