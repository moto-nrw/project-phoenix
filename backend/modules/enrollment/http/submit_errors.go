package enrollmenthttp

import (
	"net/http"

	"github.com/go-chi/render"

	"github.com/moto-nrw/project-phoenix/api/common"
	capability "github.com/moto-nrw/project-phoenix/modules/enrollment"
)

// mapSubmitError translates service-layer sentinel errors into HTTP
// status codes. Unknown errors fall through to 500.
func mapSubmitError(w http.ResponseWriter, r *http.Request, err error) {
	renderer := submitErrorRenderer(err)
	if resp, ok := renderer.(*common.ErrResponse); ok && resp.Code == common.CodeEnrollmentSubmissionRateLimited {
		// 429 Too Many Requests. Hard-coded retry hint avoids leaking
		// the exact remaining seconds.
		w.Header().Set("Retry-After", "3600")
	}
	common.RenderError(w, r, renderer)
}

// submitErrorRenderer classifies a refused submission. The first matching
// rule wins, so every specific error that wraps ErrInvalidSubmission
// precedes the generic rule.
var submitErrorRenderer = common.RulesRenderer(append(append(
	submitEligibilityErrorRules(),
	submitSelectionErrorRules()...),
	submitConflictErrorRules()...,
), func(err error) render.Renderer {
	// A captcha the server cannot check (missing secret, provider down)
	// lands here as a server error, not as the parent's mistake.
	return common.ErrorInternalServer(err)
})

// submitEligibilityErrorRules covers the phase gate and the child
// eligibility.
func submitEligibilityErrorRules() []common.ErrorRule {
	return []common.ErrorRule{
		forbiddenWithCode(capability.ErrEnrollmentDisabled, common.CodeEnrollmentDisabled),
		forbiddenWithCode(capability.ErrEnrollmentWindowClosed, common.CodeEnrollmentWindowClosed),
		forbiddenWithCode(capability.ErrLateInviteInvalid, common.CodeEnrollmentLateInviteInvalid),
		forbiddenWithCode(capability.ErrPhaseNotEligible, common.CodeEnrollmentPhaseNotEligible),
		// The two child-level eligibility errors wrap ErrInvalidSubmission,
		// so their specific matches must precede the generic rule.
		invalidWithCode(capability.ErrChildClassNotEligible, common.CodeEnrollmentClassNotEligible),
		invalidWithCode(capability.ErrChildGradeNotEligible, common.CodeEnrollmentGradeNotEligible),
		invalidWithCode(capability.ErrChildAlreadyEnrolled, common.CodeEnrollmentChildAlreadyEnrolled),
		invalidWithCode(capability.ErrChildNotEnrolled, common.CodeEnrollmentChildNotEnrolled),
		invalidWithCode(capability.ErrChildEnrollmentAmbiguous, common.CodeEnrollmentChildAmbiguous),
		// Per-child re-enrollment authorization failure (#1663): a guardian
		// account lacking parent_portal.enrollment.submit on the matched
		// student. It does NOT wrap ErrInvalidSubmission — it is a 403, not
		// a 400.
		forbiddenWithCode(capability.ErrChildEnrollmentNotPermitted, common.CodeEnrollmentChildNotPermitted),
	}
}

// submitSelectionErrorRules covers the offering selection, the guardian
// contact and the day choices, then the generic invalid submission.
func submitSelectionErrorRules() []common.ErrorRule {
	return []common.ErrorRule{
		invalidWithCode(capability.ErrCareOfferingUnavailable, common.CodeEnrollmentCareOfferingUnavailable),
		invalidWithCode(capability.ErrCareOfferingMissing, common.CodeEnrollmentCareOfferingMissing),
		invalidWithCode(capability.ErrCareOfferingExactlyOneRequired, common.CodeEnrollmentCareOfferingExactlyOne),
		invalidWithCode(capability.ErrRequiredCareOfferingMissing, common.CodeEnrollmentRequiredCareOfferingMissing),
		invalidWithCode(capability.ErrCareOfferingsDisabled, common.CodeEnrollmentCareOfferingsDisabled),
		invalidWithCode(capability.ErrInvalidGuardianPhone, common.CodeEnrollmentInvalidPhone),
		// Must precede the generic ErrInvalidSubmission rule: the email
		// error wraps ErrInvalidSubmission, so the specific match has to win.
		invalidWithCode(capability.ErrInvalidGuardianEmail, common.CodeEnrollmentInvalidEmail),
		// Must precede the generic ErrInvalidSubmission rule: the pickup
		// error wraps ErrInvalidSubmission, so the specific match has to win.
		invalidWithCode(capability.ErrPickupTimeNotAllowed, common.CodeEnrollmentPickupTimeNotAllowed),
		// Heimweg-Beschränkung (#2381) also wraps ErrInvalidSubmission.
		invalidWithCode(capability.ErrDepartureModeLimitExceeded, common.CodeEnrollmentDepartureModeLimit),
		// The three offering-day errors (#1885) also wrap
		// ErrInvalidSubmission, so their specific matches must precede the
		// generic rule.
		invalidWithCode(capability.ErrSelectedDayNotAvailable, common.CodeEnrollmentSelectedDayNotAvailable),
		invalidWithCode(capability.ErrDaySelectionRequired, common.CodeEnrollmentDaySelectionRequired),
		invalidWithCode(capability.ErrDaySelectionNotAllowed, common.CodeEnrollmentDaySelectionNotAllowed),
		invalidWithCode(capability.ErrCareOfferingClosed, common.CodeEnrollmentCareOfferingClosed),
		{Target: capability.ErrInvalidSubmission, Render: common.ErrorInvalidRequest},
	}
}

// submitConflictErrorRules covers capacity, duplicates, the rate limit and
// the captcha.
func submitConflictErrorRules() []common.ErrorRule {
	return []common.ErrorRule{
		// 409 Conflict: the request is well-formed but a selected
		// offering is at capacity and the tenant's overflow mode is
		// 'reject'. Return a JSON envelope with a stable code so the
		// frontend can render a friendly German message; the previous
		// http.Error() emitted plain text and the form fell back to
		// "(HTTP 409)".
		conflictWithCode(capability.ErrCareOfferingFull, common.CodeEnrollmentCareOfferingFull),
		// 409 Conflict: the same guardian email already has an active
		// (non-rejected, non-withdrawn) enrollment for one of these
		// children in this phase. JSON envelope so the frontend's
		// readError helper surfaces the German message instead of
		// falling back to "(HTTP 409)".
		conflictWithCode(capability.ErrDuplicateEnrollment, common.CodeEnrollmentRequestDuplicate),
		// 409 Conflict: another active request in this phase already
		// targets the same already-enrolled student this child matched (a
		// different guardian email, so the email-scoped duplicate check
		// missed it). Distinct German message so parents understand the
		// child is already being re-enrolled.
		conflictWithCode(capability.ErrExistingStudentAlreadyRequested, common.CodeEnrollmentChildAlreadyRequested),
		// 429 Too Many Requests; mapSubmitError adds the retry hint.
		{Target: capability.ErrRateLimited, Render: func(err error) render.Renderer {
			return common.ErrorTooManyRequestsWithCode(err, common.CodeEnrollmentSubmissionRateLimited)
		}},
		invalidWithCode(capability.ErrCaptchaRequired, common.CodeEnrollmentCaptchaRequired),
		invalidWithCode(capability.ErrCaptchaFailed, common.CodeEnrollmentCaptchaFailed),
	}
}

func forbiddenWithCode(target error, code string) common.ErrorRule {
	return common.ErrorRule{Target: target, Render: func(err error) render.Renderer {
		return common.ErrorForbiddenWithCode(err, code)
	}}
}

func invalidWithCode(target error, code string) common.ErrorRule {
	return common.ErrorRule{Target: target, Render: func(err error) render.Renderer {
		return common.ErrorInvalidRequestWithCode(err, code)
	}}
}

func conflictWithCode(target error, code string) common.ErrorRule {
	return common.ErrorRule{Target: target, Render: func(err error) render.Renderer {
		return common.ErrorConflictWithCode(err, code)
	}}
}
