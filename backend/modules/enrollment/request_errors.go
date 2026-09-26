package enrollment

import "errors"

// Sentinels of the parent-facing submission lifecycle. The HTTP layer maps
// them to status codes; the messages are part of the wire text of a refused
// request. The selection sentinels live in the selection contract.
var (
	ErrEnrollmentDisabled     = errors.New("enrollment is not enabled for this tenant")
	ErrEnrollmentWindowClosed = errors.New("enrollment window is closed")
	ErrLateInviteInvalid      = errors.New("late invite is invalid")
	ErrCareOfferingFull       = errors.New("one or more selected care offerings are at capacity")
	ErrRateLimited            = errors.New("too many submission attempts; please retry later")
	ErrRequestNotFound        = errors.New("enrollment request not found")
	ErrInvalidGuardianPhone   = errors.New("guardian phone number has an invalid format")
	ErrEditNotAllowed         = errors.New("request can no longer be edited")
	ErrWithdrawNotAllowed     = errors.New("child cannot be withdrawn in its current state")
	ErrDuplicateEnrollment    = errors.New("an active enrollment already exists for this parent and child in this phase")
	// ErrExistingStudentAlreadyRequested rejects an existing_students
	// submission (or parent edit) whose child matched an already-enrolled
	// student that ANOTHER active request in the same phase already targets.
	// The email-based duplicate check keys on guardian_email, so two guardians
	// with different emails submitting the same child both slip through it yet
	// pin the same matched_student_id; approving both would renew/overwrite one
	// live student twice and duplicate its care-offering enrollments. Enforced
	// unconditionally (independent of the block/warn/ignore duplicate policy)
	// because it protects a live student record, not just parent convenience
	// (#1663). Mapped to 409 Conflict.
	ErrExistingStudentAlreadyRequested = errors.New("another active enrollment request already targets this student in this phase")
	// ErrPhaseNotEligible is the audience gate (#1663): a linked_parents
	// phase rejects anonymous submissions (the parent handler additionally
	// verifies the guardian link before stamping GuardianAccountID).
	ErrPhaseNotEligible = errors.New("phase is not open for this applicant")
	// ErrChildEnrollmentNotPermitted rejects an existing_students re-enrollment
	// submitted from the parents portal when the authenticated guardian account
	// does NOT hold parent_portal.enrollment.submit on the SPECIFIC already
	// enrolled student the child matched (#1663). Guardian parent-portal
	// permissions are relationship-scoped: a parent authorized to re-enroll one
	// child must not be able to renew a DIFFERENT child at the same school just
	// because the school-wide GuardianSubmitEligible audience flag is set. It is
	// an authorization failure (mapped to 403), NOT an invalid submission.
	ErrChildEnrollmentNotPermitted = errors.New("guardian is not permitted to re-enroll this child")
	// ErrPhaseAudienceRestricted is the public form-load gate for
	// audience-restricted phases (#1663): a linked_parents or
	// existing_students phase cannot be bootstrapped anonymously, so the
	// unauthenticated public path rejects it. It maps to a plain 404 so an
	// anonymous caller cannot distinguish a restricted phase from a
	// non-existent one; the parents portal loads these phases through its
	// own authenticated bootstrap path instead.
	ErrPhaseAudienceRestricted = errors.New("phase is not available for public enrollment")
)
