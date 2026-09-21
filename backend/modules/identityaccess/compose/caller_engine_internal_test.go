package compose

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
)

// The caller-context errors reach consumers through mapCallerError as the
// public types: the sentinels consumers match, the failed operation and the
// /api/me wire text. These tests drive the composed capability over fake
// seams and assert the public side.

type engineTestAccounts struct{ identityaccess.AccountProfiles }

type engineTestPeople struct {
	found bool
	err   error
}

func (p engineTestPeople) FindPersonByAccount(context.Context, int64) (identityaccess.CallerPerson, bool, error) {
	return identityaccess.CallerPerson{ID: 70}, p.found, p.err
}
func (engineTestPeople) CreatePerson(context.Context, int64, string, string) error   { return nil }
func (engineTestPeople) RenamePerson(context.Context, int64, *string, *string) error { return nil }

type engineTestMembership struct{ staff, teacher bool }

func (m engineTestMembership) FindStaffByPerson(context.Context, int64) (int64, bool, error) {
	return 42, m.staff, nil
}

func (m engineTestMembership) FindTeacherByStaff(context.Context, int64) (int64, bool, error) {
	return 91, m.teacher, nil
}

type engineTestStructure struct {
	teacherGroups []int64
	subsErr       error
}

func (s engineTestStructure) TeacherGroupIDs(context.Context, int64) ([]int64, error) {
	return s.teacherGroups, nil
}

func (s engineTestStructure) SubstitutedGroups(context.Context, int64) (map[int64]bool, error) {
	return nil, s.subsErr
}

func (engineTestStructure) SchoolClasses(context.Context, int64) ([]string, error) { return nil, nil }

type engineTestActivities struct{}

func (engineTestActivities) SupervisedActivityGroupIDs(context.Context, int64) ([]int64, error) {
	return nil, nil
}

type engineTestSessions struct{ exists bool }

func (engineTestSessions) OpenSessionIDsForActivities(context.Context, []int64) ([]int64, error) {
	return nil, nil
}
func (engineTestSessions) SupervisedSessionIDs(context.Context, int64) ([]int64, error) {
	return nil, nil
}
func (s engineTestSessions) SessionExists(context.Context, int64) (bool, error) { return s.exists, nil }
func (engineTestSessions) SessionVisits(context.Context, int64) ([]identityaccess.CallerVisit, error) {
	return nil, nil
}

type engineTestLiveTopics struct{}

func (engineTestLiveTopics) OpenSessionIDs(context.Context) ([]int64, error) { return nil, nil }
func (engineTestLiveTopics) StaffSessionIDs(context.Context, int64) ([]int64, error) {
	return nil, nil
}

type engineSeams struct {
	authenticated bool
	people        engineTestPeople
	membership    engineTestMembership
	structure     engineTestStructure
	sessions      engineTestSessions
}

func (s engineSeams) compose(t *testing.T) identityaccess.CallerContext {
	t.Helper()
	caller, err := NewCallerContext(CallerContextDependencies{
		Caller: func(context.Context) identityaccess.Caller {
			if !s.authenticated {
				return identityaccess.Caller{}
			}
			return identityaccess.Caller{Authenticated: true, AccountID: 42, ClaimsTenantID: 10}
		},
		Accounts:   engineTestAccounts{},
		People:     s.people,
		Membership: s.membership,
		Structure:  s.structure,
		Activities: engineTestActivities{},
		Sessions:   s.sessions,
		LiveTopics: engineTestLiveTopics{},
	})
	require.NoError(t, err)
	return caller
}

// requireCallerError asserts the public CallerError with its operation, the
// public sentinel it wraps and its wire text.
func requireCallerError(t *testing.T, err error, op string, sentinel error) {
	t.Helper()
	var callerErr *identityaccess.CallerError
	require.ErrorAs(t, err, &callerErr)
	assert.Equal(t, op, callerErr.Op)
	assert.Same(t, sentinel, callerErr.Err, "the public sentinel, not the internal twin")
	require.ErrorIs(t, err, sentinel)
	assert.Equal(t, "usercontext."+op+": "+sentinel.Error(), err.Error())
}

func TestCallerEngineMapsTheChainSentinels(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	_, err := engineSeams{}.compose(t).Person(ctx)
	requireCallerError(t, err, "get user ID from context", identityaccess.ErrCallerNotAuthenticated)

	_, err = engineSeams{authenticated: true}.compose(t).Person(ctx)
	requireCallerError(t, err, "get current person", identityaccess.ErrCallerNotLinkedToPerson)

	_, err = engineSeams{authenticated: true, people: engineTestPeople{found: true}}.compose(t).StaffID(ctx)
	requireCallerError(t, err, "get current staff", identityaccess.ErrCallerNotLinkedToStaff)

	_, err = engineSeams{
		authenticated: true, people: engineTestPeople{found: true}, membership: engineTestMembership{staff: true},
	}.compose(t).TeacherID(ctx)
	requireCallerError(t, err, "get current teacher", identityaccess.ErrCallerNotLinkedToTeacher)
}

func TestCallerEngineMapsTheSessionSentinels(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	seams := engineSeams{authenticated: true, people: engineTestPeople{found: true}, membership: engineTestMembership{staff: true}}

	_, err := seams.compose(t).GroupVisits(ctx, 55)
	requireCallerError(t, err, "get group visits", identityaccess.ErrCallerGroupNotFound)

	seams.sessions.exists = true
	_, err = seams.compose(t).GroupStudentIDs(ctx, 55)
	requireCallerError(t, err, "get group students", identityaccess.ErrCallerNotAuthorized)
}

func TestCallerEngineKeepsTheCauseOfAFailedRead(t *testing.T) {
	t.Parallel()

	cause := errors.New("database connection lost")
	_, err := engineSeams{authenticated: true, people: engineTestPeople{err: cause}}.compose(t).Person(context.Background())

	var callerErr *identityaccess.CallerError
	require.ErrorAs(t, err, &callerErr)
	assert.Equal(t, "get current person", callerErr.Op)
	require.ErrorIs(t, err, cause)
	assert.NotErrorIs(t, err, identityaccess.ErrCallerNotLinkedToPerson)
	assert.Equal(t, "usercontext.get current person: database connection lost", err.Error())
}

func TestCallerEngineKeepsThePartialGroupsError(t *testing.T) {
	t.Parallel()

	cause := errors.New("substitutions unavailable")
	groups, err := engineSeams{
		authenticated: true, people: engineTestPeople{found: true},
		membership: engineTestMembership{staff: true, teacher: true},
		structure:  engineTestStructure{teacherGroups: []int64{11}, subsErr: cause},
	}.compose(t).MyGroupIDs(context.Background())

	assert.Equal(t, []int64{11}, groups, "the groups that did load are returned")
	var partial *identityaccess.CallerGroupsPartialError
	require.ErrorAs(t, err, &partial)
	assert.Equal(t, "get my groups (substitutions)", partial.Op)
	assert.Equal(t, 1, partial.FailureCount)
	require.ErrorIs(t, err, cause)
	assert.Equal(t,
		"usercontext.get my groups (substitutions): partial failure - 0 succeeded, 1 failed (last error: substitutions unavailable)",
		err.Error())
}

func TestCallerEngineKeepsTheSSESetupError(t *testing.T) {
	t.Parallel()

	_, err := engineSeams{authenticated: true}.compose(t).SSESubscription(context.Background())

	var setupErr *identityaccess.SSESetupError
	require.ErrorAs(t, err, &setupErr)
	assert.Equal(t, "Account not found", setupErr.SetupMessage())
	assert.Equal(t, http.StatusUnauthorized, setupErr.SetupStatus())
	assert.Equal(t, "SSE setup: Account not found", err.Error())

	_, err = engineSeams{authenticated: true, people: engineTestPeople{found: true}}.compose(t).SSESubscription(context.Background())
	require.ErrorAs(t, err, &setupErr)
	assert.Equal(t, http.StatusForbidden, setupErr.SetupStatus())
}

// Every public sentinel has the internal twin's text; the domain tests pin
// the internal texts, this pins that every public sentinel is mapped.
func TestMapCallerErrorCoversEveryPublicSentinel(t *testing.T) {
	t.Parallel()

	public := []error{
		identityaccess.ErrCallerNotAuthenticated, identityaccess.ErrCallerNotFound, identityaccess.ErrCallerNotAuthorized,
		identityaccess.ErrCallerNotLinkedToPerson, identityaccess.ErrCallerNotLinkedToStaff,
		identityaccess.ErrCallerNotLinkedToTeacher, identityaccess.ErrCallerGroupNotFound,
	}
	require.Len(t, callerSentinels, len(public))
	for i, pair := range callerSentinels {
		assert.Same(t, public[i], pair[1])
		assert.Equal(t, pair[0].Error(), pair[1].Error(), "texts of %v", pair[1])
		assert.Same(t, pair[1], mapCallerError(pair[0]))
	}
	require.NoError(t, mapCallerError(nil))
	other := errors.New("SSE active service is not configured")
	assert.Same(t, other, mapCallerError(other))
}
