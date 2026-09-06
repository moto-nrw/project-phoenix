package enrollment

// Per-child status values matching the column CHECK constraint.
//
// The rollover statuses (pending_renewal, auto_renewed,
// pending_admin_review) were added in migration 1.15.62 for the annual
// phase rollover flow:
//   - pending_renewal: opt-in rollover, waiting for parent to confirm.
//     Demoted to withdrawn by the deadline worker.
//   - auto_renewed: opt-out rollover, waiting for parent to decline.
//     Promoted to submitted (or approved when rollover_auto_approve)
//     by the deadline worker.
//   - pending_admin_review: rollover couldn't be auto-resolved (grade
//     above max, missing grade level, etc). Never resolved by the
//     deadline worker — admin must decide via the review queue.
const (
	ChildStatusSubmitted          = "submitted"
	ChildStatusUnderReview        = "under_review"
	ChildStatusApproved           = "approved"
	ChildStatusWaitlisted         = "waitlisted"
	ChildStatusRejected           = "rejected"
	ChildStatusWithdrawn          = "withdrawn"
	ChildStatusPendingRenewal     = "pending_renewal"
	ChildStatusAutoRenewed        = "auto_renewed"
	ChildStatusPendingAdminReview = "pending_admin_review"
)

// ReviewReason codes set when a rollover row lands in
// pending_admin_review. Free text would be tempting but a constant
// set lets the frontend show localised labels without parsing.
const (
	ReviewReasonGradeAboveMax = "grade_above_max"
	ReviewReasonNoGradeLevel  = "no_grade_level"
)

// Activation mode values matching the column CHECK constraint.
const (
	ChildActivationImmediate = "immediate"
	ChildActivationScheduled = "scheduled"
)
