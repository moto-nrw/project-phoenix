package enrollmenttest

import (
	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/modules/enrollment/compose"
)

// The Enrollment contract values Care Plan's contract suite drives the
// intake, the decision flow and the change requests with (#3565), so the
// suite names them without importing the owner.
type (
	PublicDecisions             = enrollment.Decisions
	DecideInput                 = enrollment.DecideInput
	DecideOutcome               = enrollment.DecideOutcome
	UpdateChildOfferingsInput   = enrollment.UpdateChildOfferingsInput
	OfferingAdjustmentSelection = enrollment.OfferingAdjustmentSelection
	ApprovedChildChanges        = enrollment.ApprovedChildChanges
	ApprovedChildSync           = enrollment.ApprovedChildSync

	IntakeSubmissions  = enrollment.IntakeSubmissions
	IntakeStatus       = enrollment.IntakeStatus
	SubmitRequest      = enrollment.SubmitRequest
	SubmitChild        = enrollment.SubmitChild
	SubmitOfferingDays = enrollment.SubmitOfferingDays
	SubmitResult       = enrollment.SubmitResult
	CareBookingInput   = enrollment.CareBookingInput
	School             = enrollment.School

	ChangeRequests           = enrollment.ChangeRequests
	CreateChangeRequestInput = enrollment.CreateChangeRequestInput
	ReviewChangeRequestInput = enrollment.ReviewChangeRequestInput

	SourcedTemplateResyncer         = enrollment.SourcedTemplateResyncer
	CareExitOfferingSnapshotRestore = enrollment.CareExitOfferingSnapshotRestore
	SubmittedOfferingChoice         = enrollment.SubmittedOfferingChoice
	OfferingGradeCount              = enrollment.OfferingGradeCount

	Intake                = compose.Intake
	CareOfferingRows      = compose.CareOfferingRows
	OfferingChangeRecords = compose.OfferingChangeRecords
)

// DecisionApproved is the approval decision.
const DecisionApproved = enrollment.DecisionApproved

// The public refusals the suite asserts with errors.Is.
var (
	ErrCareOfferingInvalid                    = enrollment.ErrCareOfferingInvalid
	ErrCareOfferingNotFound                   = enrollment.ErrCareOfferingNotFound
	ErrCareOfferingsDisabled                  = enrollment.ErrCareOfferingsDisabled
	ErrCareOfferingGroupRuleConflict          = enrollment.ErrCareOfferingGroupRuleConflict
	ErrCareOfferingPickupTimesRequired        = enrollment.ErrCareOfferingPickupTimesRequired
	ErrCareOfferingTemplatePeriodMismatch     = enrollment.ErrCareOfferingTemplatePeriodMismatch
	ErrCompleteWithdrawalConfirmationRequired = enrollment.ErrCompleteWithdrawalConfirmationRequired
	ErrOfferingAdjustmentInvalid              = enrollment.ErrOfferingAdjustmentInvalid
)

// NewCareOfferingRows serves Care Plan's catalog administration in
// enrollment rows, as the enrollment routes use it.
func NewCareOfferingRows(catalog compose.CareOfferingCatalogAdministration) *CareOfferingRows {
	return compose.NewCareOfferingRows(catalog)
}

// NewOfferingChangeRecords reads Care Plan's offering change requests in
// enrollment rows.
func NewOfferingChangeRecords(carePlan compose.CarePlanOfferingChanges) *OfferingChangeRecords {
	return compose.NewOfferingChangeRecords(carePlan)
}
