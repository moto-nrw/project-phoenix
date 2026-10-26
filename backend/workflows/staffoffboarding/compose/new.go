package compose

import (
	"context"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	"github.com/moto-nrw/project-phoenix/realtime"
	"github.com/moto-nrw/project-phoenix/workflows/staffoffboarding"
	"github.com/uptrace/bun"
)

// StaffOffboardingPreview is the access snapshot the workflow decides on. It
// contains no credential material: the revision is an exact encoding of the
// access decisions, not an authorization credential.
type StaffOffboardingPreview struct {
	AccountID         int64
	ActiveMembership  bool
	RoleIDs           []int64
	Tokens            int64
	PreserveGuardian  bool
	DeactivateAccount bool
	Revision          string
}

// StaffOffboardingResult is what the revocation actually removed.
type StaffOffboardingResult struct {
	RolesRevoked            int64
	TokensRevoked           int64
	GuardianAccessPreserved bool
	AccountDeactivated      bool
}

// Access is the consumer-owned port over the Identity & Access offboarding
// capability: the preview snapshots roles, sessions and
// membership at the school in context, the execution revokes them against
// that snapshot on the workflow's transaction. The composition root binds it
// (#3364).
type Access interface {
	PreviewStaffOffboarding(context.Context, int64) (StaffOffboardingPreview, error)
	ExecuteStaffOffboarding(context.Context, int64, string) (StaffOffboardingResult, error)
}

type Dependencies struct {
	DB     *bun.DB
	Access Access
	// People is the People Directory the person is unlinked in.
	People staffoffboarding.People
	// Membership, Retirement, Timetable and LockSupervision are the School
	// Membership, Timetable and Student Presence owners the root builds; the
	// composition builds no other owner's composition (#4004).
	Membership      schoolmembership.Query
	Retirement      staffoffboarding.Membership
	Timetable       staffoffboarding.Timetable
	LockSupervision func(context.Context, int64, string) ([]int64, error)
	Authorize       func(context.Context) (staffoffboarding.Actor, error)
	Cleanup         func(context.Context, int64) error
	Broadcaster     realtime.Broadcaster
	Logger          *slog.Logger
	// Audit appends the offboarding's audit records in the caller's
	// transaction, as StaffDeletionRecord builds them; the root binds both.
	Audit               AuditLog
	StaffDeletionRecord func(staffoffboarding.Actor, staffoffboarding.Result) any
}

// AuditLog is the Audit owner's append command.
type AuditLog interface {
	Append(context.Context, any) error
}

// New constructs the workflow over the ports already bound by the root.
func New(deps staffoffboarding.Dependencies) (*staffoffboarding.Workflow, error) {
	return staffoffboarding.New(deps)
}
