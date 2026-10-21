package application

import (
	"context"
	"errors"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/ports"
	identityPorts "github.com/moto-nrw/project-phoenix/modules/identityaccess/ports"
)

const (
	opGetCurrentStaff   = "get current staff"
	opGetCurrentTeacher = "get current teacher"
	opGetCurrentPerson  = "get current person"
	opGetCurrentUser    = "get current user"
)

// CallerContextDependencies binds the caller context to the principal of the
// request, the request memo and the owners of the caller's chain. LiveTopics
// and Overview are optional: without LiveTopics a subscription cannot be
// resolved, without Overview the school-wide scope is never granted.
type CallerContextDependencies struct {
	Caller     func(context.Context) domain.Caller
	Memo       func(context.Context) ports.CallerMemo
	Accounts   ports.CallerAccounts
	People     ports.CallerPeople
	Membership ports.CallerMembership
	Structure  ports.CallerStructure
	Activities ports.CallerActivities
	Sessions   ports.CallerSessions
	LiveTopics ports.CallerLiveTopics
	Overview   ports.CallerOverview
	Write      func(context.Context, func(context.Context) error) error
	RemoveFile func(string) error
	Logger     *slog.Logger
}

// CallerContext answers who the caller of a request is and what they reach:
// the Account → Person → Staff → Teacher chain, their groups and room
// sessions, their live-update topics and their own profile. The chain is
// memoized per request when the request carries a memo (#2099). Profile
// self-edits commit together in one write transaction of the tenant.
type CallerContext struct {
	deps CallerContextDependencies
}

func NewCallerContext(deps CallerContextDependencies) (*CallerContext, error) {
	if deps.Caller == nil || deps.Memo == nil || deps.Accounts == nil || deps.People == nil ||
		deps.Membership == nil || deps.Structure == nil || deps.Activities == nil || deps.Sessions == nil ||
		deps.Write == nil || deps.RemoveFile == nil {
		return nil, errors.New("identity access caller context: dependencies are required")
	}
	return &CallerContext{deps: deps}, nil
}

func (c *CallerContext) logger() *slog.Logger {
	if c.deps.Logger == nil {
		return slog.Default()
	}
	return c.deps.Logger
}

// principal returns the verified principal of the request.
func (c *CallerContext) principal(ctx context.Context) domain.Caller {
	return c.deps.Caller(ctx)
}

// accountID returns the authenticated account, or ErrCallerNotAuthenticated
// when the request carries no claims.
func (c *CallerContext) accountID(ctx context.Context) (int64, error) {
	caller := c.principal(ctx)
	if !caller.Authenticated {
		return 0, &domain.CallerError{Op: "get user ID from context", Err: domain.ErrCallerNotAuthenticated}
	}
	return caller.AccountID, nil
}

// Account returns the caller's account.
func (c *CallerContext) Account(ctx context.Context) (domain.AccountMetadata, error) {
	accountID, err := c.accountID(ctx)
	if err != nil {
		return domain.AccountMetadata{}, err
	}
	entry := c.entry(ctx)
	if account, ok := entry.cachedAccount(); ok {
		return account, nil
	}
	account, err := c.deps.Accounts.FindAccountMetadata(ctx, accountID)
	if err != nil {
		return domain.AccountMetadata{}, &domain.CallerError{Op: opGetCurrentUser, Err: err}
	}
	entry.storeAccount(account)
	return account, nil
}

// Person returns the person linked to the caller's account.
func (c *CallerContext) Person(ctx context.Context) (domain.CallerPerson, error) {
	accountID, err := c.accountID(ctx)
	if err != nil {
		return domain.CallerPerson{}, err
	}
	entry := c.entry(ctx)
	if person, ok := entry.cachedPerson(); ok {
		if person == nil {
			return domain.CallerPerson{}, &domain.CallerError{Op: opGetCurrentPerson, Err: domain.ErrCallerNotLinkedToPerson}
		}
		return *person, nil
	}
	person, found, err := c.deps.People.FindPersonByAccount(ctx, accountID)
	if err != nil {
		return domain.CallerPerson{}, &domain.CallerError{Op: opGetCurrentPerson, Err: err}
	}
	if !found {
		// A clean "not linked" outcome is memoized as such.
		entry.storePerson(nil)
		return domain.CallerPerson{}, &domain.CallerError{Op: opGetCurrentPerson, Err: domain.ErrCallerNotLinkedToPerson}
	}
	entry.storePerson(&person)
	return person, nil
}

// StaffID returns the staff member linked to the caller's person.
func (c *CallerContext) StaffID(ctx context.Context) (int64, error) {
	entry := c.entry(ctx)
	if staffID, ok := entry.cachedStaff(); ok {
		if staffID == 0 {
			return 0, &domain.CallerError{Op: opGetCurrentStaff, Err: domain.ErrCallerNotLinkedToStaff}
		}
		return staffID, nil
	}
	person, err := c.Person(ctx)
	if err != nil {
		return 0, err
	}
	staffID, found, err := c.deps.Membership.FindStaffByPerson(ctx, person.ID)
	if err != nil {
		return 0, &domain.CallerError{Op: opGetCurrentStaff, Err: err}
	}
	if !found {
		entry.storeStaff(0)
		return 0, &domain.CallerError{Op: opGetCurrentStaff, Err: domain.ErrCallerNotLinkedToStaff}
	}
	entry.storeStaff(staffID)
	return staffID, nil
}

// HasCurrentStaff exposes only the existence fact authorization policies
// need.
func (c *CallerContext) HasCurrentStaff(ctx context.Context) (bool, error) {
	staffID, err := c.StaffID(ctx)
	return err == nil && staffID > 0, err
}

// TeacherID returns the teacher linked to the caller's staff member.
func (c *CallerContext) TeacherID(ctx context.Context) (int64, error) {
	staffID, err := c.StaffID(ctx)
	if err != nil {
		return 0, err
	}
	return c.teacherForStaff(ctx, staffID)
}

// teacherForStaff shares the memoized teacher stage between TeacherID and
// MyGroupIDs. A staff member without a teacher profile is a clean outcome
// and is memoized as ErrCallerNotLinkedToTeacher.
func (c *CallerContext) teacherForStaff(ctx context.Context, staffID int64) (int64, error) {
	entry := c.entry(ctx)
	if teacherID, ok := entry.cachedTeacher(); ok {
		if teacherID == 0 {
			return 0, &domain.CallerError{Op: opGetCurrentTeacher, Err: domain.ErrCallerNotLinkedToTeacher}
		}
		return teacherID, nil
	}
	teacherID, found, err := c.deps.Membership.FindTeacherByStaff(ctx, staffID)
	if err != nil {
		return 0, &domain.CallerError{Op: opGetCurrentTeacher, Err: err}
	}
	if !found {
		entry.storeTeacher(0)
		return 0, &domain.CallerError{Op: opGetCurrentTeacher, Err: domain.ErrCallerNotLinkedToTeacher}
	}
	entry.storeTeacher(teacherID)
	return teacherID, nil
}

// StudentAccess resolves whether the caller sees unredacted student data:
// admins by their admin permission, everyone else by a verified staff
// record in the tenant (#2329).
func (c *CallerContext) StudentAccess(ctx context.Context) domain.StudentAccess {
	if c.principal(ctx).AdminWildcard {
		return domain.StudentAccess{Admin: true}
	}
	staff, err := c.HasCurrentStaff(ctx)
	return domain.StudentAccess{Staff: err == nil && staff}
}

// isExpectedLinkageError reports a clean "the caller is not linked" outcome.
func isExpectedLinkageError(err error) bool {
	return errors.Is(err, domain.ErrCallerNotLinkedToTeacher) ||
		errors.Is(err, domain.ErrCallerNotLinkedToStaff) ||
		errors.Is(err, domain.ErrCallerNotLinkedToPerson)
}

// isNotStaff reports the two outcomes that mean "the caller is no staff".
func isNotStaff(err error) bool {
	return errors.Is(err, domain.ErrCallerNotLinkedToStaff) || errors.Is(err, domain.ErrCallerNotLinkedToPerson)
}

type ReviewEngine struct {
	Review *ParentRequestReview
}

func (e ReviewEngine) ReviewScope(ctx context.Context, permissions []string) (bool, []int64, error) {
	schoolWide, groups, err := e.Review.Scope(ctx, permissions)
	return schoolWide, groups, mapCallerError(err)
}

func (e ReviewEngine) AbsenceReviewScope(ctx context.Context, permissions []string) (bool, []int64, error) {
	schoolWide, groups, err := e.Review.AbsenceScope(ctx, permissions)
	return schoolWide, groups, mapCallerError(err)
}

func (e ReviewEngine) ReviewAccessLevel(ctx context.Context, permissions []string) (string, error) {
	level, err := e.Review.AccessLevel(ctx, permissions)
	return level, mapCallerError(err)
}

type CallerAccounts struct {
	identityaccess.AccountProfiles
}

func (a CallerAccounts) FindAccountMetadata(ctx context.Context, accountID int64) (domain.AccountMetadata, error) {
	value, err := a.AccountProfiles.FindAccountMetadata(ctx, accountID)
	return domain.AccountMetadata(value), err
}

func (a CallerAccounts) FindAccountProfile(ctx context.Context, accountID int64) (domain.AccountProfile, bool, error) {
	bio, settings, found, err := a.AccountProfiles.FindAccountProfile(ctx, accountID)
	return domain.AccountProfile{Bio: bio, Settings: settings}, found, err
}

type CallerPeople struct{ identityPorts.CallerPeople }

func (p CallerPeople) FindPersonByAccount(ctx context.Context, accountID int64) (domain.CallerPerson, bool, error) {
	person, found, err := p.CallerPeople.FindPersonByAccount(ctx, accountID)
	return domain.CallerPerson(person), found, err
}

type CallerSessions struct{ identityPorts.CallerSessions }

func (s CallerSessions) SessionVisits(ctx context.Context, sessionID int64) ([]domain.CallerVisit, error) {
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
