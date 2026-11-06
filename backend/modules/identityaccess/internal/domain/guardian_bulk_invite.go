package domain

// BulkInviteMaxStudents caps one bulk run; a whole school fits, a runaway
// request does not.
const BulkInviteMaxStudents = 2000

// Reasons a guardian with a full role could not be invited in a bulk run.
const (
	BulkInviteProblemMissingEmail   = "missing_email"
	BulkInviteProblemInvalidEmail   = "invalid_email"
	BulkInviteProblemDuplicateEmail = "duplicate_email"
	// BulkInviteProblemEmailNotQueued: the outbox did not take the mail, so
	// this guardian's writes rolled back and a later run can try again (#4602).
	BulkInviteProblemEmailNotQueued = "email_not_queued"
	// BulkInviteProblemAddressBlocked: the school's list would hold the mail back.
	BulkInviteProblemAddressBlocked = "address_blocked"
	// BulkInviteProblemMailDelayed: the mail server still tries the last mail.
	BulkInviteProblemMailDelayed = "mail_delayed"
)
