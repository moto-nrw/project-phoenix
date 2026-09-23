package identityaccess

import "context"

// Reasons a guardian with a full role could not be invited in a bulk run.
const (
	BulkInviteProblemMissingEmail   = "missing_email"
	BulkInviteProblemInvalidEmail   = "invalid_email"
	BulkInviteProblemDuplicateEmail = "duplicate_email"
)

// BulkInviteRequest invites the guardians of the given children to the
// parents portal in one run (#3378). DryRun classifies without writing or
// mailing. ResendOpen mails guardians with an open invitation again and
// restarts its expiry window; without it they are skipped.
type BulkInviteRequest struct {
	StudentIDs []int64
	CreatedBy  int64
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

// BulkInviteResult counts guardians, never children. Only guardians with a
// full role on a selected child are invited; restrictive contacts are
// counted in SkippedRestricted and never upgraded. A dry run counts accounts
// that already own the address under Invited, because telling them apart
// needs the write.
type BulkInviteResult struct {
	DryRun                bool
	Invited               int
	LinkedExistingAccount int
	Resent                int
	SkippedActive         int
	SkippedOpen           int
	SkippedRestricted     int
	Problems              []BulkInviteProblem
}

func (m *Module) BulkInviteToStudents(ctx context.Context, req BulkInviteRequest) (*BulkInviteResult, error) {
	return m.engine.BulkInviteToStudents(ctx, req)
}
