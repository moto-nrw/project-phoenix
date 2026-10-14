package api

import (
	"context"
	"log/slog"

	auditModels "github.com/moto-nrw/project-phoenix/modules/auditlog/compose/vocabulary"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	careplanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	"github.com/moto-nrw/project-phoenix/modules/delivery"
	deliveryCompose "github.com/moto-nrw/project-phoenix/modules/delivery/compose"
	enrollmentCompose "github.com/moto-nrw/project-phoenix/modules/enrollment/compose"
	peopleCompose "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	"github.com/moto-nrw/project-phoenix/services"
)

// Test bindings of the enrollment flow suites (#3565): the ports their
// doubles implement and the root compositions they drive over the test
// database, for suites that may not import the compositions themselves.

// Ports of the intake and change requests the flow suites wrap in doubles.
type (
	EnrollmentIntakeRequests        = enrollmentCompose.IntakeRequests
	EnrollmentIntakeChildren        = enrollmentCompose.IntakeChildren
	EnrollmentIntakeGuardians       = enrollmentCompose.IntakeGuardians
	EnrollmentIntakeLateInvites     = enrollmentCompose.IntakeLateInvites
	EnrollmentIntakeCatalog         = enrollmentCompose.IntakeCatalog
	EnrollmentSubmissionRateLimiter = enrollmentCompose.SubmissionRateLimiter
	EnrollmentGuardianAuthorizer    = enrollmentCompose.GuardianStudentAuthorizer
	EnrollmentChangeRequestRecords  = enrollmentCompose.ChangeRequestRecords
)

// Audit rows and ports of the decision flow the flow suites record into and
// read back.
type (
	EnrollmentFlowDataAccessLog           = auditModels.DataAccessLog
	EnrollmentFlowDataAccessLogs          = auditModels.DataAccessLogRepository
	EnrollmentFlowDeletionAudit           = auditModels.EnrollmentDeletion
	EnrollmentFlowDeletionAudits          = auditModels.EnrollmentDeletionRepository
	EnrollmentFlowOfferingAdjustmentAudit = auditModels.EnrollmentOfferingAdjustment
	EnrollmentFlowRestorationAudit        = auditModels.EnrollmentRestoration
)

// Resource types of the enrollment exports in the data-access log.
const (
	EnrollmentFlowPhaseExportResource   = auditModels.ResourceTypeEnrollmentPhaseExport
	EnrollmentFlowStudentExportResource = auditModels.ResourceTypeEnrollmentStudentExport
)

// NewEnrollmentFlowDelivery composes the Delivery platform over the test
// database with a provider that accepts every send.
func NewEnrollmentFlowDelivery(db transactionruntime.Database) (*delivery.Module, error) {
	runtime, err := deliveryCompose.New(deliveryCompose.Dependencies{
		DB: db.Pool(), Provider: enrollmentFlowDeliveryProvider{}, People: enrollmentFlowGuardianDirectory{},
		Observe: func(delivery.Observation) {},
	})
	if err != nil {
		return nil, err
	}
	return runtime.Module, nil
}

type enrollmentFlowDeliveryProvider struct{}

func (enrollmentFlowDeliveryProvider) SendEmail(context.Context, delivery.ClaimedIntent) (delivery.ProviderResult, error) {
	return delivery.ProviderResult{}, nil
}

func (enrollmentFlowDeliveryProvider) SendPush(context.Context, delivery.ClaimedIntent) (delivery.ProviderResult, error) {
	return delivery.ProviderResult{}, nil
}

type enrollmentFlowGuardianDirectory struct{}

func (enrollmentFlowGuardianDirectory) ResolveGuardianDisplays(context.Context, []int64) ([]delivery.GuardianDisplay, error) {
	return nil, nil
}

// NewEnrollmentFlowStudentAudit records student changes with the request's
// audit actor, the way the root binds the decision flow's student audit.
func NewEnrollmentFlowStudentAudit(db transactionruntime.Database, actor peopleCompose.RequestAuditActor) peopleCompose.StudentAuditService {
	return peopleCompose.NewStudentAuditService(actor, NewStudentAudit(db))
}

// NewEnrollmentFlowPickupExcusal composes Care Plan's pickup auto-excusal the
// way the root binds it; extensions also records the pickup extensions the
// Timetable owner keeps.
func NewEnrollmentFlowPickupExcusal(db transactionruntime.Database, records careplan.Capability, baselines careplan.PickupBaselineReader, tt timetable.Capability, extensions bool) (careplan.PickupAutoExcusal, error) {
	adapter := services.NewPickupExcusalTimetable(tt, services.NewPartialAbsencePreview(db.Pool()))
	deps := careplanCompose.PickupExcusalDependencies{
		DB: db.Pool(), Records: records, Baselines: baselines, Blocks: services.NewLoggedStudentPresence(db.Pool(), slog.Default()), Preview: adapter,
	}
	if extensions {
		deps.Extensions = adapter
	}
	return careplanCompose.NewPickupAutoExcusal(deps)
}
