package ports

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
)

// The caller-context ports. Identity & Access owns the account; every other
// link of the caller's chain belongs to another owner and is bound at the
// composition root: the person to People Directory, staff and teacher to
// School Membership, groups, substitutions and school classes to School
// Structure, planned supervisions to Timetable and room sessions to Student
// Presence.

// CallerAccounts reads and edits the account facts of the caller.
type CallerAccounts interface {
	FindAccountMetadata(context.Context, int64) (domain.AccountMetadata, error)
	SetAccountUsername(context.Context, int64, string) error
	SetAccountAvatar(context.Context, int64, string) error
	FindAccountProfile(context.Context, int64) (domain.AccountProfile, bool, error)
	SetAccountBio(context.Context, int64, string) error
}

// CallerPeople reads and self-edits the person linked to an account. found
// is false, without an error, when the account has no person in the tenant.
type CallerPeople interface {
	FindPersonByAccount(ctx context.Context, accountID int64) (person domain.CallerPerson, found bool, err error)
	CreatePerson(ctx context.Context, accountID int64, firstName, lastName string) error
	RenamePerson(ctx context.Context, personID int64, firstName, lastName *string) error
}

// CallerMembership resolves the staff member and teacher of a person.
type CallerMembership interface {
	FindStaffByPerson(ctx context.Context, personID int64) (staffID int64, found bool, err error)
	FindTeacherByStaff(ctx context.Context, staffID int64) (teacherID int64, found bool, err error)
}

// CallerStructure reads the groups and school classes of a staff member.
// SubstitutedGroups answers for the school's current day and flags the
// groups whose regular staff slot is unassigned.
type CallerStructure interface {
	TeacherGroupIDs(ctx context.Context, teacherID int64) ([]int64, error)
	SubstitutedGroups(ctx context.Context, staffID int64) (map[int64]bool, error)
	SchoolClasses(ctx context.Context, staffID int64) ([]string, error)
}

// CallerActivities reads the activity groups a staff member supervises as
// planned.
type CallerActivities interface {
	SupervisedActivityGroupIDs(ctx context.Context, staffID int64) ([]int64, error)
}

// CallerSessions reads the room sessions a caller may reach.
type CallerSessions interface {
	OpenSessionIDsForActivities(ctx context.Context, activityGroupIDs []int64) ([]int64, error)
	SupervisedSessionIDs(ctx context.Context, staffID int64) ([]int64, error)
	SessionExists(ctx context.Context, sessionID int64) (bool, error)
	SessionVisits(ctx context.Context, sessionID int64) ([]domain.CallerVisit, error)
}

// CallerLiveTopics reads the room sessions a live-update client subscribes
// to: every open session of the school, or the staff member's own.
type CallerLiveTopics interface {
	OpenSessionIDs(ctx context.Context) ([]int64, error)
	StaffSessionIDs(ctx context.Context, staffID int64) ([]int64, error)
}

// StaffPresence answers whether the caller is a verified staff member.
type StaffPresence interface {
	HasCurrentStaff(context.Context) (bool, error)
}

// CallerOverview decides the school-wide operational overview scope
// (#2380). A nil overview skips the school-wide path.
type CallerOverview interface {
	HasOperationalOverview(ctx context.Context, staff StaffPresence, assignmentBound, admin bool) (bool, error)
}

// CallerMemo is the request-scoped slot the identity chain memoizes into.
// Entry returns the value kept for (tenant, account), creating it with
// create on first use; Evict drops it. The value belongs to the
// application, the slot only keys and guards it.
type CallerMemo interface {
	Entry(tenantID, accountID int64, create func() any) any
	Evict(tenantID, accountID int64)
}

// ReviewSettings resolves the tenant settings the parent request review
// policy reads.
type ReviewSettings interface {
	ResolveBool(ctx context.Context, key string) (bool, error)
	ResolveString(ctx context.Context, key string) (string, error)
}
