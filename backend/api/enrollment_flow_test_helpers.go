//go:build testhooks

package api

import (
	"context"
	"log/slog"

	deliveryContract "github.com/moto-nrw/project-phoenix/modules/delivery/contract"

	auditModels "github.com/moto-nrw/project-phoenix/modules/auditlog/records"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	careplanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	deliveryCompose "github.com/moto-nrw/project-phoenix/modules/delivery/compose"
	delivery "github.com/moto-nrw/project-phoenix/modules/delivery/runtime"
	peopleCompose "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"
	peoplePorts "github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
)

// Test bindings of the enrollment flow suites (#3565): the ports their
// doubles implement and the root compositions they drive over the test
// database, for suites that may not import the compositions themselves.

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
	runtime, err := deliveryCompose.NewModule(deliveryCompose.ModuleDependencies{
		DB: db.Pool(), People: enrollmentFlowGuardianDirectory{},
		Observe: func(deliveryContract.Observation) {},
	})
	if err != nil {
		return nil, err
	}
	if _, err := deliveryCompose.NewWorker(runtime, enrollmentFlowDeliveryProvider{}); err != nil {
		return nil, err
	}
	return runtime.Module, nil
}

type enrollmentFlowDeliveryProvider struct{}

func (enrollmentFlowDeliveryProvider) SendEmail(context.Context, deliveryContract.ClaimedIntent) (deliveryContract.ProviderResult, error) {
	return deliveryContract.ProviderResult{}, nil
}

func (enrollmentFlowDeliveryProvider) SendPush(context.Context, deliveryContract.ClaimedIntent) (deliveryContract.ProviderResult, error) {
	return deliveryContract.ProviderResult{}, nil
}

type enrollmentFlowGuardianDirectory struct{}

func (enrollmentFlowGuardianDirectory) ResolveGuardianDisplays(context.Context, []int64) ([]deliveryContract.GuardianDisplay, error) {
	return nil, nil
}

// NewEnrollmentFlowStudentAudit records student changes with the request's
// audit actor, the way the root binds the decision flow's student audit.
func NewEnrollmentFlowStudentAudit(db transactionruntime.Database, actor peoplePorts.RequestAuditActor) peoplePorts.StudentAuditService {
	return peopleCompose.NewStudentAuditService(actor, NewStudentAudit(db))
}

// NewEnrollmentFlowPickupExcusal composes Care Plan's pickup auto-excusal the
// way the root binds it; extensions also records the pickup extensions the
// Timetable owner keeps.
func NewEnrollmentFlowPickupExcusal(db transactionruntime.Database, records careplan.Capability, baselines careplan.PickupBaselineReader, tt timetable.Capability, extensions bool) (careplan.PickupAutoExcusal, error) {
	adapter := NewPickupExcusalTimetable(tt, NewPartialAbsencePreview(db))
	deps := careplanCompose.PickupExcusalDependencies{
		DB: db.Pool(), Records: records, Baselines: baselines, Blocks: NewLoggedStudentPresence(db, slog.Default()), Preview: adapter,
	}
	if extensions {
		deps.Extensions = adapter
	}
	return careplanCompose.NewPickupAutoExcusal(deps)
}
