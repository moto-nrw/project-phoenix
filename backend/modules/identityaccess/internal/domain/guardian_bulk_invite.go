package domain

// BulkInviteMaxStudents caps one bulk run; a whole school fits, a runaway
// request does not.
const BulkInviteMaxStudents = 2000

// Reasons a guardian with a full role could not be invited in a bulk run.
const (
	BulkInviteProblemMissingEmail   = "missing_email"
	BulkInviteProblemInvalidEmail   = "invalid_email"
	BulkInviteProblemDuplicateEmail = "duplicate_email"
)

// BulkInviteRequest invites the guardians of the given children to the
// parents portal in one run (#3378). DryRun classifies without writing or
// mailing, so the school sees the numbers before it sends.
type BulkInviteRequest struct {
	StudentIDs []int64
	CreatedBy  int64
	// ResendOpen mails guardians whose invitation is still open again and
	// restarts the expiry window; without it they are skipped.
	ResendOpen bool
	DryRun     bool
}

// BulkInviteProblem names one guardian the run could not reach and why.
type BulkInviteProblem struct {
	GuardianProfileID int64
	GuardianName      string
	StudentNames      []string
	Reason            string
}

// BulkInviteResult counts guardians, never children: a guardian of three
// selected children is invited once and counted once.
type BulkInviteResult struct {
	DryRun bool
	// Invited received (or, in a dry run, would receive) a fresh invitation.
	// A dry run cannot tell a new invitation from an existing account without
	// writing, so it counts both here.
	Invited int
	// LinkedExistingAccount already owned a platform account under the same
	// address and got the parents portal login hint instead of a token.
	LinkedExistingAccount int
	// Resent had an open invitation that was mailed again.
	Resent int
	// SkippedActive already use the parents portal.
	SkippedActive int
	// SkippedOpen have an open invitation and ResendOpen was off.
	SkippedOpen int
	// SkippedRestricted are contacts without a full guardian role on any of
	// the selected children (pickup only, emergency contact, social worker).
	SkippedRestricted int
	Problems          []BulkInviteProblem
}
