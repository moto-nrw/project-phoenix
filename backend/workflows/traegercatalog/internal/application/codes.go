package application

import (
	"errors"

	"github.com/moto-ogs/project-phoenix/modules/careplan"
	"github.com/moto-ogs/project-phoenix/modules/enrollment"
)

// Registered wire codes a school's refusal is recorded with. The workflow
// may not import api/common, so it names each code once with its
// registered value (docs/agents/error-registry.md).
const (
	codeCopyRejected           = "enrollment.traeger_copy_rejected"
	codePhaseNameExists        = "enrollment.phase_name_exists"
	codePhaseCareOfferingClash = "enrollment.phase_care_offering_conflict"
	codePhaseBookingBounds     = "enrollment.phase_booking_bounds_conflict"
	codePhasePendingWindow     = "enrollment.phase_pending_child_window_conflict"
	codePickupTimesRequired    = "enrollment.care_offering_pickup_times_required"
	codeOfferingDaysRequired   = "enrollment.care_offering_days_required"
	codeOfferingPeriodMismatch = "enrollment.care_offering_template_period_mismatch"
)

// codedError is a refusal that carries its registered code.
type codedError interface {
	ErrorCode() string
}

// refusals map the owners' refusals of a school's copy to their codes.
var refusals = []struct {
	err  error
	code string
}{
	{enrollment.ErrPhaseDuplicateName, codePhaseNameExists},
	{enrollment.ErrPhaseCareOfferingConflict, codePhaseCareOfferingClash},
	{enrollment.ErrPhaseBookingBoundsConflict, codePhaseBookingBounds},
	{enrollment.ErrPhasePendingChildWindowConflict, codePhasePendingWindow},
	{enrollment.ErrInvalidPhase, codeCopyRejected},
	{careplan.ErrCareOfferingPickupTimesRequired, codePickupTimesRequired},
	{careplan.ErrCareOfferingDaysRequired, codeOfferingDaysRequired},
	{careplan.ErrCareOfferingTemplatePeriodMismatch, codeOfferingPeriodMismatch},
	{careplan.ErrCareOfferingGroupRuleConflict, codeCopyRejected},
	{careplan.ErrCareOfferingSourceConflict, codeCopyRejected},
	{careplan.ErrCareOfferingConfigInvalid, codeCopyRejected},
	{careplan.ErrInvalidCareOffering, codeCopyRejected},
}

// rejectionCode reports whether err is a refusal a school records, and its
// code. Anything else is a failure of the rollout itself.
func rejectionCode(err error) (string, bool) {
	if err == nil || errors.Is(err, errPreviewDiscard) || errors.Is(err, errStale) {
		return "", false
	}
	var coded codedError
	if errors.As(err, &coded) && coded.ErrorCode() != "" {
		return coded.ErrorCode(), true
	}
	for _, refusal := range refusals {
		if errors.Is(err, refusal.err) {
			return refusal.code, true
		}
	}
	return "", false
}
