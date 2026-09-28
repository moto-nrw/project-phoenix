package enrollment

import (
	"errors"
	"fmt"
)

// The intake's refusals as the HTTP adapters classify them (#3565). Handlers
// render err.Error(), so every text is byte-identical to the value it
// replaces.
//
// Some refusals originate in the selection contract or in Care Plan, which
// the adapters do not reach. The intake, the change requests and the
// decision flow return those errors marked with the values below as well:
// errors.Is matches both the originating value and the Enrollment value, and
// the text stays the originating one.
var (
	// ErrInvalidSubmission is the broad category of a correctable
	// submission (400).
	ErrInvalidSubmission              = errors.New("invalid submission")
	ErrCareOfferingClosed             = errors.New("one or more selected care offerings are not currently accepting applications")
	ErrCareOfferingUnavailable        = errors.New("one or more selected care offerings are not available for this child")
	ErrCareOfferingMissing            = errors.New("care offering selection is required for every child")
	ErrCareOfferingExactlyOneRequired = errors.New("exactly one care offering must be selected for every child")
	ErrRequiredCareOfferingMissing    = errors.New("a required care offering was not selected for every child")
	ErrCareOfferingRule               = fmt.Errorf("%w: care offering selection rule not satisfied", ErrInvalidSubmission)
	ErrSelectedDayNotAvailable        = fmt.Errorf("%w: selected day is not available for this offering", ErrInvalidSubmission)
	ErrDaySelectionRequired           = fmt.Errorf("%w: offering requires the parent to pick at least one day", ErrInvalidSubmission)
	ErrDaySelectionNotAllowed         = fmt.Errorf("%w: offering does not allow parent day selection (days_of_week_mode=fixed)", ErrInvalidSubmission)

	// ErrInvalidGuardianEmail is an invalid guardian email; the handler
	// attaches enrollment.invalid_email for per-field marking.
	ErrInvalidGuardianEmail = fmt.Errorf("%w: guardian email has an invalid format", ErrInvalidSubmission)
	// ErrPickupTimeNotAllowed is a weekday_schedule time outside the field's
	// fixed pickup times.
	ErrPickupTimeNotAllowed = fmt.Errorf("%w: pickup time not allowed", ErrInvalidSubmission)
	// ErrDepartureModeLimitExceeded is more than one departure mode per
	// weekday for a child of a restricted grade (Heimweg-Beschränkung,
	// #2381).
	ErrDepartureModeLimitExceeded = fmt.Errorf("%w: only one departure mode per weekday allowed", ErrInvalidSubmission)
	// ErrChildClassNotEligible and ErrChildGradeNotEligible refuse a child
	// outside the phase's eligible classes or grades (#1663).
	ErrChildClassNotEligible = fmt.Errorf("%w: child school class is not eligible for this phase", ErrInvalidSubmission)
	ErrChildGradeNotEligible = fmt.Errorf("%w: child grade level is not eligible for this phase", ErrInvalidSubmission)
	// ErrChildAlreadyEnrolled refuses an already enrolled child in a
	// new_students phase; ErrChildNotEnrolled refuses a child without an
	// enrolled record in an existing_students phase.
	ErrChildAlreadyEnrolled = fmt.Errorf("%w: child is already enrolled at this school", ErrInvalidSubmission)
	ErrChildNotEnrolled     = fmt.Errorf("%w: child is not enrolled at this school", ErrInvalidSubmission)
	// ErrChildEnrollmentAmbiguous refuses a child that matches more than one
	// enrolled record by name and birthday (#1663).
	ErrChildEnrollmentAmbiguous = fmt.Errorf("%w: child matches multiple enrolled students at this school", ErrInvalidSubmission)

	// Care Plan refusals the enrollment routes map.
	ErrCareOfferingsDisabled                  = errors.New("care offerings are disabled for this tenant")
	ErrOfferingAdjustmentInvalid              = errors.New("offering adjustment is invalid")
	ErrCompleteWithdrawalConfirmationRequired = errors.New("Alle Betreuungstage werden entfernt. Bitte bestätigen Sie die Komplett-Abmeldung.") //nolint:staticcheck // user-facing German message
	ErrDecisionRequestNotFound                = errors.New("enrollment request not found")
	ErrDecisionChildNotFound                  = errors.New("request child not found")
	ErrCareOfferingNotFound                   = errors.New("care offering not found")
	ErrCareOfferingInvalid                    = errors.New("invalid care offering configuration")
	ErrCareOfferingTemplatePeriodMismatch     = errors.New("care offering phase must be within the linked timetable template period")
	ErrCareOfferingGroupRuleConflict          = errors.New("care offerings in the same selection group must share one selection rule")
	ErrCareOfferingDaysRequired               = errors.New("available_days must contain at least one day")
	ErrCareOfferingPickupTimesRequired        = errors.New("active care offering requires pickup_times for every weekday")
)
