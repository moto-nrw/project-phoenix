package identityaccess

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// The caller-context outcomes consumers match on. Their texts are part of
// the /api/me wire format, which renders the wrapping CallerError text.
var (
	ErrCallerNotAuthenticated   = errors.New("user not authenticated")
	ErrCallerNotFound           = errors.New("user not found")
	ErrCallerNotAuthorized      = errors.New("user not authorized")
	ErrCallerNotLinkedToPerson  = errors.New("user account not linked to a person")
	ErrCallerNotLinkedToStaff   = errors.New("user not linked to a staff member")
	ErrCallerNotLinkedToTeacher = errors.New("user not linked to a teacher")
	ErrCallerGroupNotFound      = errors.New("group not found")
)

// CallerError names the failed caller-context operation.
type CallerError struct {
	Op  string
	Err error
}

func (e *CallerError) Error() string { return fmt.Sprintf("usercontext.%s: %v", e.Op, e.Err) }

func (e *CallerError) Unwrap() error { return e.Err }

// CallerGroupsPartialError reports a group set that loaded only in part:
// the groups returned alongside it are the ones that did load.
type CallerGroupsPartialError struct {
	Op           string
	SuccessCount int
	FailureCount int
	FailedIDs    []int64
	LastErr      error
}

func (e *CallerGroupsPartialError) Error() string {
	return fmt.Sprintf("usercontext.%s: partial failure - %d succeeded, %d failed (last error: %v)",
		e.Op, e.SuccessCount, e.FailureCount, e.LastErr)
}

func (e *CallerGroupsPartialError) Unwrap() error { return e.LastErr }

// SSESetupError rejects a live-update subscription with the HTTP status it
// maps to: 401 for an account without a person, 403 for a caller who is
// neither staff nor an effective admin.
type SSESetupError struct {
	Message string
	Status  int
}

func (e *SSESetupError) Error() string { return fmt.Sprintf("SSE setup: %s", e.Message) }

// SetupStatus is the HTTP status the rejection maps to.
func (e *SSESetupError) SetupStatus() int { return e.Status }

// SetupMessage is the client-facing text of the rejection.
func (e *SSESetupError) SetupMessage() string { return e.Message }

// Caller is the authenticated principal of a request as the inbound
// boundary verified it. Authenticated reports verified claims at all;
// AccountID is zero for claims without an account. TenantID is the tenant
// of the request context, ClaimsTenantID the tenant the token was issued
// for. AdminRole is the admin role claim, AdminWildcard a system-wide admin
// permission (admin:* or *:*).
type Caller struct {
	Authenticated  bool
	AccountID      int64
	TenantID       int64
	ClaimsTenantID int64
	Scope          string
	SchoolScope    bool
	AdminRole      bool
	AdminWildcard  bool
}

// CallerPerson is the person linked to the caller's account.
type CallerPerson struct {
	ID        int64
	FirstName string
	LastName  string
	TagID     *string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// CallerVisit is one visit of a room session.
type CallerVisit struct {
	ID            int64
	CreatedAt     time.Time
	UpdatedAt     time.Time
	TenantID      int64
	StudentID     int64
	ActiveGroupID int64
	EntryTime     time.Time
	ExitTime      *time.Time
}

// CallerGroup is one of the caller's educational groups. ViaSubstitution is
// true when an active substitution with an unassigned regular staff slot is
// the caller's access path.
type CallerGroup struct {
	ID              int64
	ViaSubstitution bool
}

// CallerNavigation is the identity projection the shared frontend context
// reads in one request. StaffID is zero for a caller who is no staff
// member. A section that failed to load is named in UnavailableSections.
type CallerNavigation struct {
	Groups               []CallerGroup
	SupervisedSessionIDs []int64
	StaffID              int64
	Incomplete           bool
	UnavailableSections  []string
}

// SSESubscription is the resolved topic set of a live-update client: the
// staff member (0 for effective admins without a staff record), the room
// session topics, the educational-group topics and their deduplicated union.
type SSESubscription struct {
	StaffID        int64
	ActiveGroupIDs []string
	EduTopics      []string
	AllTopics      []string
}

// StudentAccess is the per-request decision whether the caller may see
// unredacted student data (#2329): admins by their admin permission,
// everyone else by a verified staff record in the tenant. Guests and
// guardians stay redacted.
type StudentAccess struct {
	Admin bool
	Staff bool
}

// HasFullAccess reports whether the caller sees unredacted student data.
func (a StudentAccess) HasFullAccess() bool { return a.Admin || a.Staff }

// CallerProfile is the caller's own profile. Person is nil when the account
// has no person in the tenant.
type CallerProfile struct {
	Account  AccountMetadata
	Person   *CallerPerson
	Bio      string
	Settings string
}

// CallerProfileUpdate names the self-editable fields; nil leaves a field.
type CallerProfileUpdate struct {
	FirstName *string
	LastName  *string
	Username  *string
	Bio       *string
}

// CallerIdentities resolves the caller's Account → Person → Staff → Teacher
// chain. The chain is memoized per request when the context carries a
// RequestIdentityCache. A clean "not linked" outcome is reported as
// ErrCallerNotLinkedToPerson, ErrCallerNotLinkedToStaff or
// ErrCallerNotLinkedToTeacher, a request without claims as
// ErrCallerNotAuthenticated; every other error means the chain could not be
// read.
type CallerIdentities interface {
	Account(context.Context) (AccountMetadata, error)
	Person(context.Context) (CallerPerson, error)
	StaffID(context.Context) (int64, error)
	TeacherID(context.Context) (int64, error)
	HasCurrentStaff(context.Context) (bool, error)
	StudentAccess(context.Context) StudentAccess
}

// CallerReach answers which groups, room sessions and live-update topics
// the caller reaches. A caller who is no staff member reaches none; room
// session reads of another caller's session fail with
// ErrCallerNotAuthorized, of a missing one with ErrCallerGroupNotFound.
type CallerReach interface {
	MyGroupIDs(context.Context) ([]int64, error)
	SubstitutedGroupIDs(context.Context) (map[int64]bool, error)
	MySchoolClasses(context.Context) ([]string, error)
	MyActivityGroupIDs(context.Context) ([]int64, error)
	MyActiveSessionIDs(context.Context) ([]int64, error)
	MySupervisedSessionIDs(context.Context) ([]int64, error)
	GroupStudentIDs(ctx context.Context, sessionID int64) ([]int64, error)
	GroupVisits(ctx context.Context, sessionID int64) ([]CallerVisit, error)
	Navigation(context.Context) (CallerNavigation, error)
	SSESubscription(context.Context) (SSESubscription, error)
}

// CallerProfiles reads and applies the caller's self-edits. Each edit drops
// the caller's request memo before re-reading the profile it returns.
type CallerProfiles interface {
	Profile(context.Context) (CallerProfile, error)
	UpdateProfile(context.Context, CallerProfileUpdate) (CallerProfile, error)
	UpdateAvatar(ctx context.Context, avatarURL string) (CallerProfile, error)
}

// CallerContext is the caller-context capability: who the caller of a
// request is, what they reach and their own profile. ParentRequestReviews
// is nil when the capability was composed without a review policy.
type CallerContext struct {
	CallerIdentities
	CallerReach
	CallerProfiles
	ParentRequestReviews
}

// ReviewPermissions are the route-level permission facts the parent request
// review policy narrows: a system-wide admin permission, users:absence held
// without users:read, and the permission to review excused absences at all.
type ReviewPermissions struct {
	AdminWildcard                bool
	AbsenceReadPrerequisiteUnmet bool
	CanReviewExcused             bool
}

// Review access levels reported by ReviewAccessLevel. They tell a client
// why its parent request queue may be empty, which is the difference between
// "nothing to do" and "your school has not given you this".
const (
	ReviewAccessAdmin       = "admin"
	ReviewAccessTeam        = "team"
	ReviewAccessGroupLeader = "group_leader"
	ReviewAccessNone        = "none"
)

// ParentRequestReviews narrows parent request review behind the route-level
// permissions. Administrators stay school-wide; group leaders review their
// own groups only where the school enables it. ReviewScope serves every
// request kind, AbsenceReviewScope the sick and excused requests, whose
// school setting may widen the scope to all staff. A scope is school-wide
// or exactly the returned group IDs, sorted and without duplicates.
type ParentRequestReviews interface {
	ReviewScope(ctx context.Context, permissions []string) (schoolWide bool, groupIDs []int64, err error)
	AbsenceReviewScope(ctx context.Context, permissions []string) (schoolWide bool, groupIDs []int64, err error)
	ReviewAccessLevel(ctx context.Context, permissions []string) (string, error)
}
