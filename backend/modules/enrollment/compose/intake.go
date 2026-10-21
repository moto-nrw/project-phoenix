package compose

import (
	"database/sql"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	application "github.com/moto-nrw/project-phoenix/modules/enrollment/internal/application"
)

// Ports the composition root binds for Enrollment's intake and change
// requests (#3565).
type (
	IntakeDependencies             = application.IntakeDependencies
	IntakeRequests                 = application.IntakeRequests
	RequestParentChanges           = application.RequestParentChanges
	IntakeChildren                 = application.IntakeChildren
	IntakeGuardians                = application.IntakeGuardians
	IntakeLateInvites              = application.IntakeLateInvites
	IntakeCatalog                  = application.IntakeCatalog
	IntakeOfferings                = application.IntakeOfferings
	IntakeSettings                 = application.IntakeSettings
	AdminMailSubscribers           = application.AdminMailSubscribers
	LegalSettings                  = application.LegalSettings
	SubmissionRateLimiter          = application.SubmissionRateLimiter
	StudentMatches                 = application.StudentMatches
	GuardianStudentAuthorizer      = application.GuardianStudentAuthorizer
	GuardianInvitationAvailability = application.GuardianInvitationAvailability
	ManualEnrollmentDecider        = application.ManualEnrollmentDecider

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

// PublicDecisions hands the decision flow directly to the HTTP adapters.
// The flow marks refusals with Enrollment's public values at its own edge.
func PublicDecisions(decisions *Decisions) enrollment.Decisions {
	return decisions
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
