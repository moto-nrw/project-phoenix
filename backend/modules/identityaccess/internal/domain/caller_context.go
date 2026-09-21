package domain

import (
	"errors"
	"fmt"
	"time"
)

// The caller-context outcomes. Their texts are part of the /api/me wire
// format: the HTTP adapter renders the wrapping CallerError text.
var (
	ErrCallerNotAuthenticated   = errors.New("user not authenticated")
	ErrCallerNotFound           = errors.New("user not found")
	ErrCallerNotAuthorized      = errors.New("user not authorized")
	ErrCallerNotLinkedToPerson  = errors.New("user account not linked to a person")
	ErrCallerNotLinkedToStaff   = errors.New("user not linked to a staff member")
	ErrCallerNotLinkedToTeacher = errors.New("user not linked to a teacher")
	ErrCallerGroupNotFound      = errors.New("group not found")
)

// Caller is the authenticated principal of a request, resolved at the
// inbound boundary. Authenticated reports that the request carries verified
// claims at all; AccountID is zero for claims without an account.
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

// EffectiveAdmin reports the admin role or a system-wide admin permission.
func (c Caller) EffectiveAdmin() bool { return c.AdminRole || c.AdminWildcard }

// CallerPerson is the person linked to the caller's account.
type CallerPerson struct {
	ID        int64
	FirstName string
	LastName  string
	TagID     *string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// CallerVisit is an open or closed visit of one room session.
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

// CallerGroup is one of the caller's educational groups.
type CallerGroup struct {
	ID              int64
	ViaSubstitution bool
}

// CallerNavigation is the identity projection the shared frontend context
// reads in one request. Group-derived sections are optional: a failed
// section is listed in UnavailableSections instead of failing the whole.
type CallerNavigation struct {
	Groups               []CallerGroup
	SupervisedSessionIDs []int64
	StaffID              int64
	Incomplete           bool
	UnavailableSections  []string
}

// SSESubscription is the resolved topic set for a live-update client.
type SSESubscription struct {
	StaffID        int64
	ActiveGroupIDs []string
	EduTopics      []string
	AllTopics      []string
}

// SSESetupError carries the HTTP status a rejected subscription maps to.
type SSESetupError struct {
	Message string
	Status  int
}

func (e *SSESetupError) Error() string { return fmt.Sprintf("SSE setup: %s", e.Message) }

// StudentAccess is the per-request decision whether the caller may see
// unredacted student data (#2329).
type StudentAccess struct {
	Admin bool
	Staff bool
}

// CallerProfile is the profile the caller edits about themself.
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

// CallerError names the failed caller-context operation. The text format is
// kept from the retained user-context service.
type CallerError struct {
	Op  string
	Err error
}

func (e *CallerError) Error() string { return fmt.Sprintf("usercontext.%s: %v", e.Op, e.Err) }

func (e *CallerError) Unwrap() error { return e.Err }

// CallerGroupsPartialError reports a group set that loaded only in part.
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

// ReviewPermissions are the route-level permission facts the parent request
// review policy narrows.
type ReviewPermissions struct {
	AdminWildcard                bool
	AbsenceReadPrerequisiteUnmet bool
	CanReviewExcused             bool
}

// Review access levels: why a caller's parent request queue may be empty,
// which is the difference between "nothing to do" and "your school has not
// given you this".
const (
	ReviewAccessAdmin       = "admin"
	ReviewAccessTeam        = "team"
	ReviewAccessGroupLeader = "group_leader"
	ReviewAccessNone        = "none"
)
