package compose

import (
	"context"
	"database/sql"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/modules/enrollment/internal/application"
)

// Ports the composition root binds for Enrollment's intake and change
// requests (#3565).
type (
	IntakeDependencies        = application.IntakeDependencies
	IntakeRequests            = application.IntakeRequests
	IntakeChildren            = application.IntakeChildren
	IntakeGuardians           = application.IntakeGuardians
	IntakeLateInvites         = application.IntakeLateInvites
	IntakeCatalog             = application.IntakeCatalog
	IntakeOfferings           = application.IntakeOfferings
	IntakeSettings            = application.IntakeSettings
	LegalSettings             = application.LegalSettings
	SubmissionRateLimiter     = application.SubmissionRateLimiter
	StudentMatches            = application.StudentMatches
	GuardianStudentAuthorizer = application.GuardianStudentAuthorizer
	ManualEnrollmentDecider   = application.ManualEnrollmentDecider

	ChangeRequestDependencies = application.ChangeRequestDependencies
	ChangeRequestRecords      = application.ChangeRequestRecords
	CareBookingChanges        = application.CareBookingChanges
	CareBookingGates          = application.CareBookingGates
	CompanionGraphCoordinator = application.CompanionGraphCoordinator
	ReviewerNames             = application.ReviewerNames

	CarePlanOfferings   = application.CarePlanOfferings
	CareOfferingRecords = application.CareOfferingRecords
	CareOfferingRows    = application.CareOfferingRows

	CareOfferingCatalogAdministration = application.CareOfferingCatalogAdministration
)

// Intake is Enrollment's parent-facing intake: the public
// IntakeSubmissions, IntakeStatus and IntakeForms capabilities in one value.
type Intake = application.Intake

// NewIntake composes the intake in the tenant runtime.
func NewIntake(deps IntakeDependencies) *Intake {
	deps.Runtime = tenantRuntime()
	return application.NewIntake(deps)
}

// NewChangeRequests composes the change requests in the tenant runtime.
func NewChangeRequests(deps ChangeRequestDependencies) enrollment.ChangeRequests {
	deps.Runtime = tenantRuntime()
	return application.NewChangeRequests(deps)
}

// NewCareOfferingRecords reads Care Plan's offerings in enrollment rows.
func NewCareOfferingRecords(carePlan CarePlanOfferings) *CareOfferingRecords {
	return application.NewCareOfferingRecords(carePlan)
}

// NewCareOfferingRows serves Care Plan's catalog administration to the
// enrollment routes in enrollment rows.
func NewCareOfferingRows(catalog CareOfferingCatalogAdministration) *CareOfferingRows {
	return application.NewCareOfferingRows(catalog)
}

// PublicDecisions hands the decision flow to the HTTP adapters: refusals that
// originate in the selection contract or in Care Plan come back marked with
// Enrollment's public values as well.
func PublicDecisions(decisions *Decisions) enrollment.Decisions {
	return publicDecisions{inner: decisions}
}

type publicDecisions struct{ inner *Decisions }

func (d publicDecisions) DecisionRequests(ctx context.Context, filters enrollment.DecisionRequestFilters) ([]*enrollment.DecisionSummary, error) {
	out, err := d.inner.DecisionRequests(ctx, filters)
	return out, application.PublicError(err)
}

func (d publicDecisions) StudentDecisionRequests(ctx context.Context, studentID int64) ([]*enrollment.DecisionSummary, error) {
	out, err := d.inner.StudentDecisionRequests(ctx, studentID)
	return out, application.PublicError(err)
}

func (d publicDecisions) DecisionRequest(ctx context.Context, requestID int64) (*enrollment.DecisionSummary, error) {
	out, err := d.inner.DecisionRequest(ctx, requestID)
	return out, application.PublicError(err)
}

func (d publicDecisions) Decide(ctx context.Context, input enrollment.DecideInput) (*enrollment.DecideOutcome, error) {
	out, err := d.inner.Decide(ctx, input)
	return out, application.PublicError(err)
}

func (d publicDecisions) RestoreWithdrawn(ctx context.Context, requestID, restoredBy int64) (*enrollment.RestoreOutcome, error) {
	out, err := d.inner.RestoreWithdrawn(ctx, requestID, restoredBy)
	return out, application.PublicError(err)
}

func (d publicDecisions) UpdateChildOfferings(ctx context.Context, input enrollment.UpdateChildOfferingsInput) (*enrollment.RequestChild, error) {
	out, err := d.inner.UpdateChildOfferings(ctx, input)
	return out, application.PublicError(err)
}

func (d publicDecisions) ListOfferingAdjustments(ctx context.Context, requestID, requestChildID int64) ([]*enrollment.OfferingAdjustmentRecord, error) {
	out, err := d.inner.ListOfferingAdjustments(ctx, requestID, requestChildID)
	return out, application.PublicError(err)
}

func (d publicDecisions) ListChildOfferings(ctx context.Context, requestID int64) (map[int64]enrollment.ChildOfferingSet, error) {
	out, err := d.inner.ListChildOfferings(ctx, requestID)
	return out, application.PublicError(err)
}

func (d publicDecisions) ExportPhase(ctx context.Context, phaseID, actorAccountID int64, actorRole, format, childStatusFilter string) (*enrollment.PhaseExport, error) {
	out, err := d.inner.ExportPhase(ctx, phaseID, actorAccountID, actorRole, format, childStatusFilter)
	return out, application.PublicError(err)
}

func (d publicDecisions) ExportStudent(ctx context.Context, studentID, actorAccountID int64, actorRole, format string) (*enrollment.StudentEnrollmentExport, error) {
	out, err := d.inner.ExportStudent(ctx, studentID, actorAccountID, actorRole, format)
	return out, application.PublicError(err)
}

func (d publicDecisions) RecordPhaseExportAudit(ctx context.Context, actorAccountID int64, actorRole string, phase *enrollment.Phase, format, statusFilter string, requestCount, childCount int) error {
	return application.PublicError(d.inner.RecordPhaseExportAudit(ctx, actorAccountID, actorRole, phase, format, statusFilter, requestCount, childCount))
}

// OfferingChangeRecords reads Care Plan's offering change requests in
// enrollment rows for the guardian portal's request sharing.
type (
	OfferingChangeRecords   = application.OfferingChangeRecords
	CarePlanOfferingChanges = application.CarePlanOfferingChanges
)

// NewOfferingChangeRecords binds the reads to Care Plan.
func NewOfferingChangeRecords(carePlan CarePlanOfferingChanges) *OfferingChangeRecords {
	return application.NewOfferingChangeRecords(carePlan, sql.ErrNoRows)
}
