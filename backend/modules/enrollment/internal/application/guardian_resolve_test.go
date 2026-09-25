package application

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
)

// stubGuardianProfiles implements only the guardian profile reads and writes
// resolveGuardianProfile touches; the embedded interface panics on any other
// call, which keeps the unit test honest about what the resolver depends on.
type stubGuardianProfiles struct {
	GuardianProfiles
	byAccount map[int64]*GuardianProfile
	byEmail   map[string]*GuardianProfile
	// byAccountErr simulates an operational (non-not-found) failure of the
	// by-account lookup.
	byAccountErr error
	byEmailErr   error
	emailLookups int
	created      int
	updated      int
}

func (s *stubGuardianProfiles) GuardianProfileByAccount(_ context.Context, accountID int64) (*GuardianProfile, error) {
	if s.byAccountErr != nil {
		return nil, s.byAccountErr
	}
	return s.byAccount[accountID], nil
}

func (s *stubGuardianProfiles) GuardianProfileByEmail(_ context.Context, email string) (*GuardianProfile, error) {
	s.emailLookups++
	if s.byEmailErr != nil {
		return nil, s.byEmailErr
	}
	return s.byEmail[strings.ToLower(strings.TrimSpace(email))], nil
}

func (s *stubGuardianProfiles) CreateGuardianProfile(context.Context, *GuardianProfile) error {
	s.created++
	return nil
}

func (s *stubGuardianProfiles) UpdateGuardianProfile(context.Context, *GuardianProfile) error {
	s.updated++
	return nil
}

func int64Ptr(v int64) *int64 { return &v }

// testTenantKey carries the tenant the stubbed runtime reports, so a grant
// can prove it ran in the approval's own context.
type testTenantKey struct{}

func withTestTenant(ctx context.Context, tenantID int64) context.Context {
	return context.WithValue(ctx, testTenantKey{}, tenantID)
}

func testTenantID(ctx context.Context) int64 {
	tenantID, _ := ctx.Value(testTenantKey{}).(int64)
	return tenantID
}

func guardianDecisions(deps DecisionDependencies) *Decisions {
	deps.Runtime.TenantID = testTenantID
	if deps.Logger == nil {
		deps.Logger = slog.Default()
	}
	return &Decisions{deps: deps}
}

func withGuardianProfiles(profiles GuardianProfiles) DecisionDependencies {
	return DecisionDependencies{People: PeopleDirectory{GuardianProfiles: profiles}}
}

func TestResolveGuardianProfile_PreservesEmailReadFailure(t *testing.T) {
	t.Parallel()
	failure := errors.New("guardian storage unavailable")
	repo := &stubGuardianProfiles{byEmailErr: failure}
	svc := guardianDecisions(withGuardianProfiles(repo))
	profile, created, err := svc.resolveGuardianProfile(context.Background(), &enrollmentModels.Request{
		GuardianEmail: "guardian@example.test", GuardianFirstName: "Anna", GuardianLastName: "Test",
	})
	require.ErrorIs(t, err, failure)
	require.Nil(t, profile)
	require.False(t, created)
	require.Zero(t, repo.created)
}

func TestResolveAdditionalGuardianProfile_PreservesEmailReadFailure(t *testing.T) {
	t.Parallel()
	failure := errors.New("guardian storage unavailable")
	repo := &stubGuardianProfiles{byEmailErr: failure}
	svc := guardianDecisions(withGuardianProfiles(repo))
	email := "guardian@example.test"
	id, err := svc.resolveAdditionalGuardianProfile(context.Background(), &enrollment.RequestGuardian{
		Email: &email, FirstName: "Anna", LastName: "Test",
	})
	require.ErrorIs(t, err, failure)
	require.Zero(t, id)
	require.Zero(t, repo.created)
}

type failingDecisionRequestReader struct {
	DecisionRequests
	err error
}

func (r failingDecisionRequestReader) RequestByID(context.Context, int64, bool) (*enrollment.Request, error) {
	return nil, r.err
}

func TestDecide_PreservesRequestReadFailure(t *testing.T) {
	t.Parallel()
	failure := errors.New("request storage unavailable")
	svc := guardianDecisions(DecisionDependencies{
		Requests: failingDecisionRequestReader{err: failure},
		Runtime:  Runtime{NotFound: func(err error) bool { return errors.Is(err, errNoDecisionRow) }},
	})
	outcome, err := svc.Decide(context.Background(), enrollment.DecideInput{RequestID: 10, ChildID: 20, Status: enrollment.DecisionApproved})
	require.ErrorIs(t, err, failure)
	require.NotErrorIs(t, err, careplan.ErrBookingRequestNotFound)
	require.Nil(t, outcome)
}

var errNoDecisionRow = errors.New("no rows")

type stubLateInvites struct {
	invite  *enrollment.LateInvite
	err     error
	lookups int
}

func (s *stubLateInvites) LateInviteByUsedRequestID(context.Context, int64) (*enrollment.LateInvite, error) {
	s.lookups++
	return s.invite, s.err
}

func TestGuardianIdentityRequest_UsesLateInviteRecipient(t *testing.T) {
	t.Parallel()

	repo := &stubLateInvites{invite: &enrollment.LateInvite{GuardianEmail: "invited@example.test"}}
	svc := guardianDecisions(DecisionDependencies{LateInvites: repo})
	request := &enrollmentModels.Request{
		SubmissionSource: enrollmentModels.RequestSourceLateInvite,
		GuardianEmail:    "corrected@example.test",
	}
	request.ID = 42

	identity, err := svc.guardianIdentityRequest(context.Background(), request)

	require.NoError(t, err)
	assert.Equal(t, "invited@example.test", identity.GuardianEmail)
	assert.Equal(t, "corrected@example.test", request.GuardianEmail, "the editable contact address must remain unchanged")
	assert.Equal(t, 1, repo.lookups)
}

func TestGuardianIdentityRequest_AuthenticatedSubmitKeepsAccountIdentity(t *testing.T) {
	t.Parallel()

	repo := &stubLateInvites{invite: &enrollment.LateInvite{GuardianEmail: "invited@example.test"}}
	svc := guardianDecisions(DecisionDependencies{LateInvites: repo})
	request := &enrollmentModels.Request{
		SubmissionSource:  enrollmentModels.RequestSourceLateInvite,
		GuardianAccountID: int64Ptr(23),
		GuardianEmail:     "corrected@example.test",
	}

	identity, err := svc.guardianIdentityRequest(context.Background(), request)

	require.NoError(t, err)
	assert.Same(t, request, identity)
	assert.Zero(t, repo.lookups)
}

func TestGuardianIdentityRequest_MissingLateInviteFailsClosed(t *testing.T) {
	t.Parallel()

	svc := guardianDecisions(DecisionDependencies{})
	request := &enrollmentModels.Request{
		SubmissionSource: enrollmentModels.RequestSourceLateInvite,
		GuardianEmail:    "corrected@example.test",
	}

	identity, err := svc.guardianIdentityRequest(context.Background(), request)

	require.Error(t, err)
	assert.Nil(t, identity)
}

// #1663: an authenticated submit whose email resolves to a DIFFERENT account's
// guardian profile must be rejected, not silently linked to that other account.
func TestResolveGuardianProfile_RejectsCrossAccountEmail(t *testing.T) {
	t.Parallel()

	const (
		callerAccount = int64(10)
		victimAccount = int64(20)
	)
	victim := &GuardianProfile{ID: 99, FirstName: "Vera", LastName: "Opfer", AccountID: int64Ptr(victimAccount)}
	repo := &stubGuardianProfiles{
		byAccount: map[int64]*GuardianProfile{}, // caller has no profile here yet
		byEmail:   map[string]*GuardianProfile{"victim@example.test": victim},
	}
	svc := guardianDecisions(withGuardianProfiles(repo))

	req := &enrollmentModels.Request{
		GuardianAccountID: int64Ptr(callerAccount),
		GuardianEmail:     "victim@example.test",
		GuardianFirstName: "Anna",
		GuardianLastName:  "Antragsteller",
	}

	got, wasNew, err := svc.resolveGuardianProfile(context.Background(), req)
	require.ErrorIs(t, err, enrollment.ErrGuardianAccountMismatch)
	assert.Nil(t, got)
	assert.False(t, wasNew)
	assert.Zero(t, repo.created, "must not create a profile on a rejected mismatch")
}

// The authenticated account is authoritative: when the caller already owns a
// profile at the tenant, it wins even if the (parent-editable) email was
// changed — the resolver must not fall back to an email lookup.
func TestResolveGuardianProfile_PrefersAuthenticatedAccountProfile(t *testing.T) {
	t.Parallel()

	const callerAccount = int64(10)
	own := &GuardianProfile{ID: 5, FirstName: "Anna", LastName: "Antragsteller", AccountID: int64Ptr(callerAccount)}
	// A colliding email owned by someone else exists too; the account-first
	// resolution must never consult it.
	other := &GuardianProfile{ID: 99, FirstName: "Vera", LastName: "Opfer", AccountID: int64Ptr(int64(20))}
	repo := &stubGuardianProfiles{
		byAccount: map[int64]*GuardianProfile{callerAccount: own},
		byEmail:   map[string]*GuardianProfile{"victim@example.test": other},
	}
	svc := guardianDecisions(withGuardianProfiles(repo))

	req := &enrollmentModels.Request{
		GuardianAccountID: int64Ptr(callerAccount),
		GuardianEmail:     "victim@example.test",
	}

	got, wasNew, err := svc.resolveGuardianProfile(context.Background(), req)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, own.ID, got.ID)
	assert.False(t, wasNew)
	assert.Zero(t, repo.created)
}

// stubGuardianAccess answers the account lookups the ownership check and the
// attach paths make and records every guardian-access grant with the tenant
// it was requested for.
type stubGuardianAccess struct {
	byID   map[int64]identityaccess.Account
	grants []identityaccess.GuardianTenantAccess
}

func (s *stubGuardianAccess) FindAccount(_ context.Context, id int64) (identityaccess.Account, error) {
	account, ok := s.byID[id]
	if !ok {
		return identityaccess.Account{}, identityaccess.ErrAccountNotFound
	}
	return account, nil
}

func (s *stubGuardianAccess) FindAccountByEmail(_ context.Context, email string) (identityaccess.Account, error) {
	for _, account := range s.byID {
		if strings.EqualFold(account.Email, email) {
			return account, nil
		}
	}
	return identityaccess.Account{}, identityaccess.ErrAccountNotFound
}

func (s *stubGuardianAccess) GrantGuardianTenantAccess(ctx context.Context, accountID int64) (identityaccess.GuardianTenantAccess, error) {
	granted := identityaccess.GuardianTenantAccess{AccountID: accountID, TenantID: testTenantID(ctx), RoleAssigned: true}
	s.grants = append(s.grants, granted)
	return granted, nil
}

// #1663: an UNLINKED profile is claimable only by the account that owns its
// email. A logged-in parent typing a stranger's address at a school where they
// have no profile yet must not have that family's profile (and every child on
// it) attached to their account by the by-id attach in applyApproval.
func TestResolveGuardianProfile_RejectsForeignUnclaimedEmailProfile(t *testing.T) {
	t.Parallel()

	const callerAccount = int64(10)
	foreign := &GuardianProfile{ID: 77, FirstName: "Vera", LastName: "Opfer"} // AccountID nil
	repo := &stubGuardianProfiles{
		byAccount: map[int64]*GuardianProfile{},
		byEmail:   map[string]*GuardianProfile{"victim@example.test": foreign},
	}
	accounts := &stubGuardianAccess{byID: map[int64]identityaccess.Account{
		callerAccount: {ID: callerAccount, Email: "caller@example.test"},
	}}
	deps := withGuardianProfiles(repo)
	deps.GuardianAccess = accounts
	svc := guardianDecisions(deps)

	req := &enrollmentModels.Request{
		GuardianAccountID: int64Ptr(callerAccount),
		GuardianEmail:     "victim@example.test",
		GuardianFirstName: "Anna",
		GuardianLastName:  "Antragsteller",
	}

	got, wasNew, err := svc.resolveGuardianProfile(context.Background(), req)
	require.ErrorIs(t, err, enrollment.ErrGuardianAccountMismatch)
	assert.Nil(t, got)
	assert.False(t, wasNew)
	assert.Zero(t, repo.created, "must not create a profile on a rejected claim")
}

// The same shape with the caller's OWN address is the legitimate claim: an
// admin-created profile for this parent at a new school.
func TestResolveGuardianProfile_AllowsOwnUnclaimedEmailProfile(t *testing.T) {
	t.Parallel()

	const callerAccount = int64(10)
	own := &GuardianProfile{ID: 7, FirstName: "Anna", LastName: "Antragsteller"} // AccountID nil
	repo := &stubGuardianProfiles{
		byAccount: map[int64]*GuardianProfile{},
		byEmail:   map[string]*GuardianProfile{"anna@example.test": own},
	}
	accounts := &stubGuardianAccess{byID: map[int64]identityaccess.Account{
		callerAccount: {ID: callerAccount, Email: "Anna@Example.test"}, // case-insensitive match
	}}
	deps := withGuardianProfiles(repo)
	deps.GuardianAccess = accounts
	svc := guardianDecisions(deps)

	req := &enrollmentModels.Request{
		GuardianAccountID: int64Ptr(callerAccount),
		GuardianEmail:     "anna@example.test",
		GuardianFirstName: "Anna",
		GuardianLastName:  "Antragsteller",
	}

	got, _, err := svc.resolveGuardianProfile(context.Background(), req)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, own.ID, got.ID)
}

// A database failure on the by-account lookup must NOT degrade into the email
// path: doing so hands the linkage decision to the parent-editable email field
// (or creates a duplicate profile) on a transient outage. Only the explicit
// not-found answer may fall through.
func TestResolveGuardianProfile_PropagatesAccountLookupFailure(t *testing.T) {
	t.Parallel()

	const callerAccount = int64(10)
	other := &GuardianProfile{ID: 99, FirstName: "Vera", LastName: "Opfer"}
	repo := &stubGuardianProfiles{
		byAccount:    map[int64]*GuardianProfile{},
		byEmail:      map[string]*GuardianProfile{"anna@example.test": other},
		byAccountErr: errors.New("connection reset by peer"),
	}
	svc := guardianDecisions(withGuardianProfiles(repo))

	req := &enrollmentModels.Request{
		GuardianAccountID: int64Ptr(callerAccount),
		GuardianEmail:     "anna@example.test",
		GuardianFirstName: "Anna",
		GuardianLastName:  "Antragsteller",
	}

	got, wasNew, err := svc.resolveGuardianProfile(context.Background(), req)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "connection reset by peer")
	assert.Nil(t, got)
	assert.False(t, wasNew)
	assert.Zero(t, repo.emailLookups, "a lookup failure must not fall through to the email path")
	assert.Zero(t, repo.created, "a lookup failure must not create a duplicate profile")
}

// An unclaimed profile carrying the email (no account yet) can only be
// claimed after the ownership check ran. Without the Identity & Access
// capability the check cannot run, and the approval must refuse instead of
// assuming ownership: the old optional repositories answered "owned" when
// unwired, which handed the linkage decision to the parent-editable email
// field (#2563: no nil no-op collaborators).
func TestResolveGuardianProfile_RequiresGuardianAccessForUnclaimedEmailProfile(t *testing.T) {
	t.Parallel()

	const callerAccount = int64(10)
	unclaimed := &GuardianProfile{ID: 7, FirstName: "Anna", LastName: "Antragsteller"} // AccountID nil
	repo := &stubGuardianProfiles{
		byAccount: map[int64]*GuardianProfile{},
		byEmail:   map[string]*GuardianProfile{"anna@example.test": unclaimed},
	}
	svc := guardianDecisions(withGuardianProfiles(repo))

	req := &enrollmentModels.Request{
		GuardianAccountID: int64Ptr(callerAccount),
		GuardianEmail:     "anna@example.test",
		GuardianFirstName: "Anna",
		GuardianLastName:  "Antragsteller",
	}

	got, wasNew, err := svc.resolveGuardianProfile(context.Background(), req)
	require.ErrorIs(t, err, errDecisionGuardianAccessRequired)
	assert.Nil(t, got)
	assert.False(t, wasNew)
	assert.Zero(t, repo.created, "a refused ownership check must not create a profile")
}

// A deleted submitter account is not an outage: the by-id attach falls back
// to the email owner, so the ownership check lets the unclaimed profile
// through exactly like before.
func TestResolveGuardianProfile_AllowsUnclaimedEmailProfileWhenSubmitterAccountIsGone(t *testing.T) {
	t.Parallel()

	const callerAccount = int64(10)
	unclaimed := &GuardianProfile{ID: 7, FirstName: "Anna", LastName: "Antragsteller"} // AccountID nil
	repo := &stubGuardianProfiles{
		byAccount: map[int64]*GuardianProfile{},
		byEmail:   map[string]*GuardianProfile{"anna@example.test": unclaimed},
	}
	deps := withGuardianProfiles(repo)
	deps.GuardianAccess = &stubGuardianAccess{byID: map[int64]identityaccess.Account{}}
	svc := guardianDecisions(deps)

	req := &enrollmentModels.Request{
		GuardianAccountID: int64Ptr(callerAccount),
		GuardianEmail:     "anna@example.test",
		GuardianFirstName: "Anna",
		GuardianLastName:  "Antragsteller",
	}

	got, _, err := svc.resolveGuardianProfile(context.Background(), req)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, unclaimed.ID, got.ID)
}

// --- attachGuardianAccountIfPresent: already-linked profiles ----------------

// #1663 review: a guardian profile that already carries an account_id proves a
// LINK, not portal ACCESS — account_id survives an offboarding that flipped
// auth.account_tenants to inactive. Since pendingGuardianInvite sends nothing
// for a linked profile, an early return here would approve the child and leave
// the parent locked out. The approval must (re)assert the active mapping.
func TestAttachGuardianAccountIfPresent_ReactivatesLinkedAccountTenant(t *testing.T) {
	t.Parallel()

	const (
		accountID = int64(4242)
		tenantID  = int64(77)
	)
	guardian := &GuardianProfile{
		ID:         5,
		FirstName:  "Anna",
		LastName:   "Antragsteller",
		AccountID:  int64Ptr(accountID),
		HasAccount: true,
	}

	access := &stubGuardianAccess{byID: map[int64]identityaccess.Account{}}
	svc := guardianDecisions(DecisionDependencies{GuardianAccess: access})

	ctx := withTestTenant(context.Background(), tenantID)
	require.NoError(t, svc.attachGuardianAccountIfPresent(ctx, &enrollmentModels.Request{}, guardian, false))

	require.Len(t, access.grants, 1,
		"an already-linked guardian must still be (re)granted ACTIVE access to this school")
	assert.Equal(t, accountID, access.grants[0].AccountID)
	assert.Equal(t, tenantID, access.grants[0].TenantID)
}

// Without the Identity & Access capability the already-linked path must
// refuse rather than approve a child the parent cannot see.
func TestAttachGuardianAccountIfPresent_RequiresGuardianAccess(t *testing.T) {
	t.Parallel()

	guardian := &GuardianProfile{ID: 5, AccountID: int64Ptr(4242), HasAccount: true}
	svc := guardianDecisions(DecisionDependencies{})
	ctx := withTestTenant(context.Background(), 77)
	err := svc.attachGuardianAccountIfPresent(ctx, &enrollmentModels.Request{}, guardian, false)
	require.ErrorIs(t, err, errDecisionGuardianAccessRequired)
}
