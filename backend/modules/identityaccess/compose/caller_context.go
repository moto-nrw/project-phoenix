package compose

import (
	"context"
	"errors"
	"log/slog"
	"os"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/application"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/ports"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// CallerPeople is the People Directory seam of the caller context: the
// person linked to an account and the caller's own name edits. found is
// false, without an error, when the account has no person in the tenant.
type CallerPeople interface {
	FindPersonByAccount(ctx context.Context, accountID int64) (person identityaccess.CallerPerson, found bool, err error)
	CreatePerson(ctx context.Context, accountID int64, firstName, lastName string) error
	RenamePerson(ctx context.Context, personID int64, firstName, lastName *string) error
}

// CallerMembership is the School Membership seam: the staff member of a
// person and the teacher of a staff member.
type CallerMembership interface {
	FindStaffByPerson(ctx context.Context, personID int64) (staffID int64, found bool, err error)
	FindTeacherByStaff(ctx context.Context, staffID int64) (teacherID int64, found bool, err error)
}

// CallerStructure is the School Structure seam: a teacher's groups, the
// groups a staff member substitutes for on the school's current day
// (flagging an unassigned regular staff slot) and a staff member's school
// classes.
type CallerStructure interface {
	TeacherGroupIDs(ctx context.Context, teacherID int64) ([]int64, error)
	SubstitutedGroups(ctx context.Context, staffID int64) (map[int64]bool, error)
	SchoolClasses(ctx context.Context, staffID int64) ([]string, error)
}

// CallerActivities is the Timetable seam: the activity groups a staff
// member supervises as planned.
type CallerActivities interface {
	SupervisedActivityGroupIDs(ctx context.Context, staffID int64) ([]int64, error)
}

// CallerSessions is the Student Presence seam: the room sessions a caller
// reaches and their visits.
type CallerSessions interface {
	OpenSessionIDsForActivities(ctx context.Context, activityGroupIDs []int64) ([]int64, error)
	SupervisedSessionIDs(ctx context.Context, staffID int64) ([]int64, error)
	SessionExists(ctx context.Context, sessionID int64) (bool, error)
	SessionVisits(ctx context.Context, sessionID int64) ([]identityaccess.CallerVisit, error)
}

// CallerLiveTopics is the Student Presence seam of live-update clients:
// every open room session of the school, or a staff member's own.
type CallerLiveTopics interface {
	OpenSessionIDs(ctx context.Context) ([]int64, error)
	StaffSessionIDs(ctx context.Context, staffID int64) ([]int64, error)
}

// CallerOverview decides the school-wide operational overview scope
// (#2380) for a caller whose staff identity staff verifies.
type CallerOverview interface {
	HasOperationalOverview(ctx context.Context, staff interface {
		HasCurrentStaff(context.Context) (bool, error)
	}, assignmentBound, admin bool) (bool, error)
}

// CallerContextDependencies binds the caller context at the composition
// root. Caller resolves the verified principal of the request; the tenant
// of the request context is added here. Memo resolves the request's memo
// slot; it returns nil for a context without one. LiveTopics and Overview
// are optional: without LiveTopics no subscription resolves, without
// Overview the school-wide subscription scope is never granted.
type CallerContextDependencies struct {
	Caller     func(context.Context) identityaccess.Caller
	Memo       func(context.Context) CallerMemo
	Accounts   identityaccess.AccountProfiles
	People     CallerPeople
	Membership CallerMembership
	Structure  CallerStructure
	Activities CallerActivities
	Sessions   CallerSessions
	LiveTopics CallerLiveTopics
	Overview   CallerOverview
	Review     *ParentRequestReviewDependencies
	Logger     *slog.Logger
}

// ParentRequestReviewDependencies binds the parent request review policy.
// Permissions evaluates the route-level permission facts; AbsenceReadRequired
// is the error a caller holding users:absence without users:read is refused
// with, so the queues and the write gate refuse alike.
type ParentRequestReviewDependencies struct {
	Permissions func([]string) identityaccess.ReviewPermissions
	Settings    interface {
		ResolveBool(ctx context.Context, key string) (bool, error)
		ResolveString(ctx context.Context, key string) (string, error)
	}
	GroupLeaderSettingKey string
	AbsenceSettingKey     string
	AbsenceReadRequired   error
}

// NewCallerContext composes the caller-context capability.
func NewCallerContext(deps CallerContextDependencies) (identityaccess.CallerContext, error) {
	if deps.Caller == nil || deps.Accounts == nil || deps.People == nil || deps.Membership == nil ||
		deps.Structure == nil || deps.Activities == nil || deps.Sessions == nil {
		return identityaccess.CallerContext{}, errors.New("identity access compose: caller context dependencies are required")
	}
	appDeps := application.CallerContextDependencies{
		Caller: func(ctx context.Context) domain.Caller {
			caller := domain.Caller(deps.Caller(ctx))
			caller.TenantID = tenant.FromContext(ctx)
			return caller
		},
		Memo:       requestMemo(deps.Memo),
		Accounts:   callerAccounts{deps.Accounts},
		People:     callerPeople{deps.People},
		Membership: deps.Membership,
		Structure:  deps.Structure,
		Activities: deps.Activities,
		Sessions:   callerSessions{deps.Sessions},
		Write:      transaction{}.RunWrite,
		RemoveFile: os.Remove,
		Logger:     deps.Logger,
	}
	if deps.LiveTopics != nil {
		appDeps.LiveTopics = deps.LiveTopics
	}
	if deps.Overview != nil {
		appDeps.Overview = callerOverview{deps.Overview}
	}
	app, err := application.NewCallerContext(appDeps)
	if err != nil {
		return identityaccess.CallerContext{}, err
	}
	engine := callerEngine{app: app}
	result := identityaccess.CallerContext{CallerIdentities: engine, CallerReach: engine, CallerProfiles: engine}
	if deps.Review != nil {
		review, err := newParentRequestReview(app, *deps.Review)
		if err != nil {
			return identityaccess.CallerContext{}, err
		}
		result.ParentRequestReviews = review
	}
	return result, nil
}

func newParentRequestReview(caller *application.CallerContext, deps ParentRequestReviewDependencies) (reviewEngine, error) {
	if deps.Permissions == nil {
		return reviewEngine{}, errors.New("identity access compose: review permissions are required")
	}
	appDeps := application.ParentRequestReviewDependencies{
		Permissions: func(permissions []string) domain.ReviewPermissions {
			return domain.ReviewPermissions(deps.Permissions(permissions))
		},
		GroupLeaderSettingKey: deps.GroupLeaderSettingKey,
		AbsenceSettingKey:     deps.AbsenceSettingKey,
		AbsenceReadRequired:   deps.AbsenceReadRequired,
	}
	if deps.Settings != nil {
		appDeps.Settings = deps.Settings
	}
	review, err := application.NewParentRequestReview(caller, appDeps)
	return reviewEngine{review: review}, err
}

type reviewEngine struct {
	review *application.ParentRequestReview
}

func (e reviewEngine) ReviewScope(ctx context.Context, permissions []string) (bool, []int64, error) {
	schoolWide, groups, err := e.review.Scope(ctx, permissions)
	return schoolWide, groups, mapCallerError(err)
}

func (e reviewEngine) AbsenceReviewScope(ctx context.Context, permissions []string) (bool, []int64, error) {
	schoolWide, groups, err := e.review.AbsenceScope(ctx, permissions)
	return schoolWide, groups, mapCallerError(err)
}

func (e reviewEngine) ReviewAccessLevel(ctx context.Context, permissions []string) (string, error) {
	level, err := e.review.AccessLevel(ctx, permissions)
	return level, mapCallerError(err)
}

// CallerMemo is the request-scoped slot the caller context memoizes the
// identity chain in (#2099): Entry returns the value kept for (tenant,
// account), creating it on first use, and Evict drops it.
type CallerMemo interface {
	Entry(tenantID, accountID int64, create func() any) any
	Evict(tenantID, accountID int64)
}

// requestMemo binds the application memo to the slot the root resolves
// from the request context; a request without one is not memoized.
func requestMemo(memo func(context.Context) CallerMemo) func(context.Context) ports.CallerMemo {
	return func(ctx context.Context) ports.CallerMemo {
		if memo == nil {
			return nil
		}
		if slot := memo(ctx); slot != nil {
			return slot
		}
		return nil
	}
}

type callerAccounts struct {
	profiles identityaccess.AccountProfiles
}

func (a callerAccounts) FindAccountMetadata(ctx context.Context, accountID int64) (domain.AccountMetadata, error) {
	value, err := a.profiles.FindAccountMetadata(ctx, accountID)
	return domain.AccountMetadata(value), err
}

func (a callerAccounts) SetAccountUsername(ctx context.Context, accountID int64, username string) error {
	return a.profiles.SetAccountUsername(ctx, accountID, username)
}

func (a callerAccounts) SetAccountAvatar(ctx context.Context, accountID int64, avatar string) error {
	return a.profiles.SetAccountAvatar(ctx, accountID, avatar)
}

func (a callerAccounts) FindAccountProfile(ctx context.Context, accountID int64) (domain.AccountProfile, bool, error) {
	bio, settings, found, err := a.profiles.FindAccountProfile(ctx, accountID)
	return domain.AccountProfile{Bio: bio, Settings: settings}, found, err
}

func (a callerAccounts) SetAccountBio(ctx context.Context, accountID int64, bio string) error {
	return a.profiles.SetAccountBio(ctx, accountID, bio)
}

type callerPeople struct{ people CallerPeople }

func (p callerPeople) FindPersonByAccount(ctx context.Context, accountID int64) (domain.CallerPerson, bool, error) {
	person, found, err := p.people.FindPersonByAccount(ctx, accountID)
	return domain.CallerPerson(person), found, err
}

func (p callerPeople) CreatePerson(ctx context.Context, accountID int64, firstName, lastName string) error {
	return p.people.CreatePerson(ctx, accountID, firstName, lastName)
}

func (p callerPeople) RenamePerson(ctx context.Context, personID int64, firstName, lastName *string) error {
	return p.people.RenamePerson(ctx, personID, firstName, lastName)
}

type callerSessions struct{ CallerSessions }

func (s callerSessions) SessionVisits(ctx context.Context, sessionID int64) ([]domain.CallerVisit, error) {
	visits, err := s.CallerSessions.SessionVisits(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	result := make([]domain.CallerVisit, 0, len(visits))
	for _, visit := range visits {
		result = append(result, domain.CallerVisit(visit))
	}
	return result, nil
}

type callerOverview struct{ overview CallerOverview }

func (o callerOverview) HasOperationalOverview(ctx context.Context, staff ports.StaffPresence, assignmentBound, admin bool) (bool, error) {
	return o.overview.HasOperationalOverview(ctx, staff, assignmentBound, admin)
}

// mapCallerError turns the application's caller-context errors into the
// public ones, keeping the failed operation and the wrapped cause.
// A context a use case wraps around such an error keeps its text, and
// errors.Is/As reach the public error inside it.
func mapCallerError(err error) error {
	if err == nil {
		return nil
	}
	if mapped, ok := mapCallerErrorValue(err); ok {
		return mapped
	}
	inner := errors.Unwrap(err)
	if inner == nil {
		return err
	}
	mappedInner := mapCallerError(inner)
	if mappedInner == inner {
		return err
	}
	return &mappedCallerError{text: err.Error(), cause: mappedInner}
}

// mapCallerErrorValue translates one of the application's caller-context
// errors itself, not an error wrapping one.
func mapCallerErrorValue(err error) (error, bool) {
	switch e := err.(type) {
	case *domain.CallerError:
		return &identityaccess.CallerError{Op: e.Op, Err: mapCallerError(e.Err)}, true
	case *domain.CallerGroupsPartialError:
		return &identityaccess.CallerGroupsPartialError{
			Op: e.Op, SuccessCount: e.SuccessCount, FailureCount: e.FailureCount,
			FailedIDs: e.FailedIDs, LastErr: e.LastErr,
		}, true
	case *domain.SSESetupError:
		return &identityaccess.SSESetupError{Message: e.Message, Status: e.Status}, true
	}
	for _, pair := range callerSentinels {
		if err == pair[0] {
			return pair[1], true
		}
	}
	return nil, false
}

// mappedCallerError keeps the text of a wrapped caller-context error while
// its cause is the public error.
type mappedCallerError struct {
	text  string
	cause error
}

func (e *mappedCallerError) Error() string { return e.text }

func (e *mappedCallerError) Unwrap() error { return e.cause }

var callerSentinels = [][2]error{
	{domain.ErrCallerNotAuthenticated, identityaccess.ErrCallerNotAuthenticated},
	{domain.ErrCallerNotFound, identityaccess.ErrCallerNotFound},
	{domain.ErrCallerNotAuthorized, identityaccess.ErrCallerNotAuthorized},
	{domain.ErrCallerNotLinkedToPerson, identityaccess.ErrCallerNotLinkedToPerson},
	{domain.ErrCallerNotLinkedToStaff, identityaccess.ErrCallerNotLinkedToStaff},
	{domain.ErrCallerNotLinkedToTeacher, identityaccess.ErrCallerNotLinkedToTeacher},
	{domain.ErrCallerGroupNotFound, identityaccess.ErrCallerGroupNotFound},
}
