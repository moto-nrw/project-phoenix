//go:build testhooks

package api

import (
	"context"
	"time"

	securitycontract "github.com/moto-nrw/project-phoenix/modules/securityruntime/ports"

	owners "github.com/moto-nrw/project-phoenix/roots/owners"

	auditCompose "github.com/moto-nrw/project-phoenix/modules/auditlog/compose"
	auditModels "github.com/moto-nrw/project-phoenix/modules/auditlog/records"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	carePlanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	peopleCompose "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"
	peoplePorts "github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"

	configModels "github.com/moto-nrw/project-phoenix/modules/settings/records"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// CareLifecycleTestModule is the Care Plan lifecycle; the student audit it
// records through comes beside it from NewCareLifecycleTestModule (#4080).
type CareLifecycleTestModule struct {
	CareLifecycle careplan.CareLifecycle
}

// An optional clock pins the lifecycle's today to the suite's pinned day,
// as the care participation decides past days against it (#6041).
func NewCareLifecycleTestModule(db transactionruntime.Database, unit tenant.UnitOfWork, clocks ...func() time.Time) (CareLifecycleTestModule, peoplePorts.StudentAuditService, error) {
	settings, err := owners.NewSettingsTestModule(db, unit)
	if err != nil {
		return CareLifecycleTestModule{}, nil, err
	}
	command, err := auditCompose.NewCommand(owners.NewTestAuditStore(db), func(auditCompose.AppendObservation) {})
	if err != nil {
		return CareLifecycleTestModule{}, nil, err
	}
	r, err := NewCareLifecycleTestRepositories(db, command)
	if err != nil {
		return CareLifecycleTestModule{}, nil, err
	}
	audit := peopleCompose.NewStudentAuditService(requestTokenReads.AuditActor, NewStudentAudit(db))
	recurrenceLock, err := owners.NewTimetableTestRecurrenceLock(db)
	if err != nil {
		return CareLifecycleTestModule{}, nil, err
	}
	lifecycle, err := r.NewCareLifecycle(CareLifecycleTestConfig{
		Audit:                 audit,
		LockCareBookingWrites: recurrenceLock.LockRecurrenceWrites,
		BookingsAuthoritative: func(ctx context.Context) (bool, error) {
			return settings.Settings.ResolveBool(ctx, configModels.KeyEnrollmentBookingsAuthoritative)
		},
		Today: calendar.CalendarDateClock(clocks...),
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
	userContext securitycontract.StudentAccessUserContext,
	edits auditModels.StudentFieldEditRepository, accessLog auditModels.DataAccessLogRepository,
) (careplan.StudentDocuments, error) {
	deps, err := owners.StudentDocumentDependencies(records, students, userContext, edits, accessLog, studentDocumentPermissions())
	if err != nil {
		return nil, err
	}
	deps.DB = db.Pool()
	documents, err := carePlanCompose.NewStudentDocuments(deps)
	if err != nil {
		return nil, err
	}
	return documents, nil
}

// NewStudentAccidentReportsTestModule composes the accident reports kept
// among the child documents (#4771) over the same collaborators.
func NewStudentAccidentReportsTestModule(
	db transactionruntime.Database, records careplan.Capability, students peoplePorts.StudentRepository,
	userContext securitycontract.StudentAccessUserContext,
	edits auditModels.StudentFieldEditRepository, accessLog auditModels.DataAccessLogRepository,
) (careplan.StudentAccidentReports, error) {
	deps, err := owners.StudentDocumentDependencies(records, students, userContext, edits, accessLog, studentDocumentPermissions())
	if err != nil {
		return nil, err
	}
	deps.DB = db.Pool()
	reports, err := carePlanCompose.NewStudentDocuments(deps)
	if err != nil {
		return nil, err
	}
	return reports, nil
}

// NewSchoolMembershipTestModule composes School Membership for router tests
// that drive a child's lifecycle the way the real writes do. An optional
// clock pins its today, which keeps the first-care bound of a membership
// (#6041).
func NewSchoolMembershipTestModule(db transactionruntime.Database, clocks ...func() time.Time) (schoolmembership.Capability, error) {
	return owners.NewSchoolMembership(db, clocks...)
}
