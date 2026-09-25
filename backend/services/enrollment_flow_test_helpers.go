package services

import (
	"context"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	careplanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	"github.com/moto-nrw/project-phoenix/modules/delivery"
	deliveryCompose "github.com/moto-nrw/project-phoenix/modules/delivery/compose"
	enrollmentCompose "github.com/moto-nrw/project-phoenix/modules/enrollment/compose"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/services/users"
	"github.com/uptrace/bun"
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
func NewEnrollmentFlowDelivery(db *bun.DB) (*delivery.Module, error) {
	runtime, err := deliveryCompose.New(deliveryCompose.Dependencies{
		DB: db, Provider: enrollmentFlowDeliveryProvider{}, People: enrollmentFlowGuardianDirectory{},
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
func NewEnrollmentFlowStudentAudit(db *bun.DB, actor users.RequestAuditActor) users.StudentAuditService {
	return users.NewStudentAuditService(actor, repositories.NewStudentAudit(db))
}

// NewEnrollmentFlowPickupExcusal composes Care Plan's pickup auto-excusal the
// way the root binds it; extensions also records the pickup extensions the
// Timetable owner keeps.
func NewEnrollmentFlowPickupExcusal(db *bun.DB, records careplan.Capability, baselines careplan.PickupBaselineReader, tt timetable.Capability, extensions bool) (careplan.PickupAutoExcusal, error) {
	adapter := newPickupExcusalTimetable(tt, db)
	deps := careplanCompose.PickupExcusalDependencies{
		DB: db, Records: records, Baselines: baselines, Blocks: newStudentPresence(db, slog.Default()), Preview: adapter,
	}
	if extensions {
		deps.Extensions = adapter
	}
	return careplanCompose.NewPickupAutoExcusal(deps)
}
