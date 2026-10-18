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
	configModels "github.com/moto-nrw/project-phoenix/modules/settings/records"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// CareLifecycleTestModule is the Care Plan lifecycle; the student audit it
// records through comes beside it from NewCareLifecycleTestModule (#4080).
type CareLifecycleTestModule struct {
	CareLifecycle careplan.CareLifecycle
}

func NewCareLifecycleTestModule(db transactionruntime.Database, unit tenant.UnitOfWork) (CareLifecycleTestModule, peoplePorts.StudentAuditService, error) {
	settings, err := NewSettingsTestModule(db, unit)
	if err != nil {
		return CareLifecycleTestModule{}, nil, err
	}
	command, err := auditCompose.NewCommand(NewTestAuditStore(db), func(auditCompose.AppendObservation) {})
	if err != nil {
		return CareLifecycleTestModule{}, nil, err
	}
	r, err := NewCareLifecycleTestRepositories(db, command)
	if err != nil {
		return CareLifecycleTestModule{}, nil, err
	}
	audit := peopleCompose.NewStudentAuditService(requestTokenReads.AuditActor, NewStudentAudit(db))
	recurrenceLock, err := newTimetableTestRecurrenceLock(db)
	if err != nil {
		return CareLifecycleTestModule{}, nil, err
	}
	lifecycle, err := r.NewCareLifecycle(CareLifecycleTestConfig{
		Audit:                 audit,
		LockCareBookingWrites: recurrenceLock.LockRecurrenceWrites,
		BookingsAuthoritative: func(ctx context.Context) (bool, error) {
			return settings.Settings.ResolveBool(ctx, configModels.KeyEnrollmentBookingsAuthoritative)
		},
	})
	if err != nil {
		return CareLifecycleTestModule{}, nil, err
	}
	return CareLifecycleTestModule{CareLifecycle: lifecycle}, audit, nil
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
