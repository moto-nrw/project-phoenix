package careplan

import "errors"

// The error contract of the Care Plan decisions on offering changes, course
// requests, pickup adjustments and offering adjustments (#3558). Handlers
// render err.Error() to staff and parents, so every text is byte-identical to
// the services/enrollment value it replaces; services/enrollment points its
// legacy names at these values until the decisions move here.
var (
	// ErrOfferingChangeDisabled means the school has post-enrollment changes
	// switched off.
	ErrOfferingChangeDisabled = errors.New("enrollment: post-enrollment offering changes are disabled")
	// ErrOfferingChangeInvalid marks a request the caller can correct: no
	// selection, an unknown offering, a bad effective date.
	ErrOfferingChangeInvalid = errors.New("enrollment: invalid offering change request")
	// ErrOfferingChangeNoEnrollment means the child has no approved enrollment
	// an offering change could be applied to.
	ErrOfferingChangeNoEnrollment = errors.New("enrollment: child has no approved enrollment")
	// ErrOfferingChangeForbidden means the caller does not own the request.
	ErrOfferingChangeForbidden = errors.New("enrollment: offering change request forbidden")
	// ErrOfferingChangeCapacityFull means an offering in the request has no free
	// slot left. Raised at approval time, when it actually matters.
	ErrOfferingChangeCapacityFull = errors.New("enrollment: care offering is at capacity")
	// ErrOfferingChangeDateOutOfRange means the reviewer confirmed a date the
	// switch cannot take effect on: before today, or outside the care period the
	// request belongs to.
	ErrOfferingChangeDateOutOfRange = errors.New("enrollment: confirmed effective date is out of range")

	// ErrCareOfferingsDisabled means the tenant has care offerings switched
	// off. Enrollment paths return the same value.
	ErrCareOfferingsDisabled = errors.New("care offerings are disabled for this tenant")
	// ErrOfferingAdjustmentInvalid means a staff offering adjustment names
	// offerings or days that cannot be applied. Enrollment decisions return the
	// same value.
	ErrOfferingAdjustmentInvalid = errors.New("offering adjustment is invalid")
	// ErrCompleteWithdrawalConfirmationRequired means the change removes every
	// care day and the caller has not confirmed the complete withdrawal.
	ErrCompleteWithdrawalConfirmationRequired = errors.New("Alle Betreuungstage werden entfernt. Bitte bestätigen Sie die Komplett-Abmeldung.") //nolint:staticcheck // user-facing German message

	// ErrCourseRequestsDisabled means the school has parent course requests
	// switched off (or the offering-change machinery they run on).
	ErrCourseRequestsDisabled = errors.New("enrollment: parent course requests are disabled")
	// ErrCourseNotFound means the id is not a course the child may request:
	// unknown, not bound to an AG, or not part of the child's care period.
	ErrCourseNotFound = errors.New("enrollment: course not found")
	// ErrCourseAlreadyBooked means the child already holds that course.
	ErrCourseAlreadyBooked = errors.New("enrollment: course is already booked")
	// ErrCourseRequestNotOwn means the pending request is not a course request
	// the caller submitted, so it must not be withdrawn here.
	ErrCourseRequestNotOwn = errors.New("enrollment: not an own course request")

	// ErrPickupResetNoOffering protects a manual pickup row when no
	// booking-derived pickup time would replace it on the requested date.
	ErrPickupResetNoOffering = errors.New("für diesen Tag gibt es keine Angebots-Gehzeit")

	ErrPickupAdjustmentInvalid            = errors.New("pickup adjustment: invalid input")
	ErrPickupAdjustmentResolutionRequired = errors.New("pickup adjustment: explicit resolution is required")
	ErrPickupAdjustmentStale              = errors.New("pickup adjustment: preview is stale")
	ErrPickupAdjustmentFutureManualReset  = errors.New("pickup adjustment: manual pickup times can only be reset today")
	ErrPickupAdjustmentBulkConfirmation   = errors.New("pickup adjustment: bulk exceptions require confirmation")
	ErrPickupAdjustmentUnauthorized       = errors.New("pickup adjustment: student is not authorized")
	ErrPickupAdjustmentStudentNotFound    = errors.New("pickup adjustment: student not found")
)
