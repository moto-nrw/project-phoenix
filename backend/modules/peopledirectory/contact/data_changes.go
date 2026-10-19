package contact

// Change-request lifecycle states. Track A (direct edit) writes rows directly
// as auto_applied — the live record was already updated, the row is the audit
// trail. Track B (review) writes pending rows that a staff decision moves to
// approved (and applies) or rejected.
const (
	DataChangeStatusAutoApplied = "auto_applied"
	DataChangeStatusPending     = "pending"
	DataChangeStatusApproved    = "approved"
	DataChangeStatusRejected    = "rejected"
	// DataChangeStatusDone closes a request that only covered days already
	// gone: nothing to apply, and "abgelehnt" would misstate what happened.
	DataChangeStatusDone = "done"
	// DataChangeStatusCareEnded closes an open request whose child left the
	// OGS before anybody decided it (#2487). Deliberately its own value:
	// "abgelehnt" would tell the family the office looked at the change and
	// said no, which is not what happened.
	DataChangeStatusCareEnded = "care_ended"
)

// Target identifies which underlying record a change applies to. For person /
// student / departure the student_id is enough to locate the row; for
// guardian_profile / guardian_phone the TargetRefID carries the specific
// sub-record id (the submitting guardian's own profile / phone).
const (
	DataChangeTargetPerson          = "person"
	DataChangeTargetStudent         = "student"
	DataChangeTargetGuardianProfile = "guardian_profile"
	DataChangeTargetGuardianPhone   = "guardian_phone"
	DataChangeTargetDeparture       = "departure"
)
