package enrollment

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	capability "github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// stubGuardianProfileRepo implements only the GuardianProfileRepository methods
// resolveGuardianProfile touches; the embedded interface panics on any other
// call, which keeps the unit test honest about what the resolver depends on.
type stubGuardianProfileRepo struct {
	usersModels.GuardianProfileRepository
	byAccount map[int64]*usersModels.GuardianProfile
	byEmail   map[string]*usersModels.GuardianProfile
	// byAccountErr simulates an operational (non-not-found) failure of the
	// by-account lookup.
	byAccountErr error
	byEmailErr   error
	emailLookups int
	created      int
	updated      int
}

func (s *stubGuardianProfileRepo) FindByAccountID(_ context.Context, accountID int64) (*usersModels.GuardianProfile, error) {
	if s.byAccountErr != nil {
		return nil, s.byAccountErr
	}
	if p, ok := s.byAccount[accountID]; ok {
		return p, nil
	}
	return nil, usersModels.ErrGuardianProfileNotFound
}

func (s *stubGuardianProfileRepo) FindByEmail(_ context.Context, email string) (*usersModels.GuardianProfile, error) {
	s.emailLookups++
	if s.byEmailErr != nil {
		return nil, s.byEmailErr
	}
	if p, ok := s.byEmail[strings.ToLower(strings.TrimSpace(email))]; ok {
		return p, nil
	}
	return nil, usersModels.ErrGuardianProfileNotFound
}

func (s *stubGuardianProfileRepo) Create(_ context.Context, _ *usersModels.GuardianProfile) error {
	s.created++
	return nil
}

func (s *stubGuardianProfileRepo) Update(_ context.Context, _ *usersModels.GuardianProfile) error {
	s.updated++
	return nil
}

func int64Ptr(v int64) *int64 { return &v }

func TestResolveGuardianProfile_PreservesEmailReadFailure(t *testing.T) {
	t.Parallel()
	failure := errors.New("guardian storage unavailable")
	repo := &stubGuardianProfileRepo{byEmailErr: failure}
	svc := &decisionService{DecisionServiceConfig: DecisionServiceConfig{GuardianProfileRepo: repo}}
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
	repo := &stubGuardianProfileRepo{byEmailErr: failure}
	svc := &decisionService{DecisionServiceConfig: DecisionServiceConfig{GuardianProfileRepo: repo}}
	email := "guardian@example.test"
	id, err := svc.resolveAdditionalGuardianProfile(context.Background(), &capability.RequestGuardian{
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

func (r failingDecisionRequestReader) RequestByID(context.Context, int64, bool) (*capability.Request, error) {
	return nil, r.err
}

func TestDecide_PreservesRequestReadFailure(t *testing.T) {
	t.Parallel()
	failure := errors.New("request storage unavailable")
	svc := &decisionService{DecisionServiceConfig: DecisionServiceConfig{Requests: failingDecisionRequestReader{err: failure}}}
	outcome, err := svc.Decide(context.Background(), DecideInput{RequestID: 10, ChildID: 20, Status: DecisionApproved})
	require.ErrorIs(t, err, failure)
	require.NotErrorIs(t, err, ErrDecisionRequestNotFound)
	require.Nil(t, outcome)
}

type stubLateInviteRepo struct {
	invite  *capability.LateInvite
	err     error
	lookups int
}

func (s *stubLateInviteRepo) LateInviteByUsedRequestID(_ context.Context, _ int64) (*capability.LateInvite, error) {
	s.lookups++
	return s.invite, s.err
}

func TestGuardianIdentityRequest_UsesLateInviteRecipient(t *testing.T) {
	t.Parallel()

	repo := &stubLateInviteRepo{invite: &capability.LateInvite{GuardianEmail: "invited@example.test"}}
	svc := &decisionService{DecisionServiceConfig: DecisionServiceConfig{LateInviteRepo: repo}}
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

	repo := &stubLateInviteRepo{invite: &capability.LateInvite{GuardianEmail: "invited@example.test"}}
	svc := &decisionService{DecisionServiceConfig: DecisionServiceConfig{LateInviteRepo: repo}}
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

	svc := &decisionService{}
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
	victim := &usersModels.GuardianProfile{FirstName: "Vera", LastName: "Opfer", AccountID: int64Ptr(victimAccount)}
	victim.ID = 99
	repo := &stubGuardianProfileRepo{
		byAccount: map[int64]*usersModels.GuardianProfile{}, // caller has no profile here yet
		byEmail:   map[string]*usersModels.GuardianProfile{"victim@example.test": victim},
	}
	svc := &decisionService{DecisionServiceConfig: DecisionServiceConfig{GuardianProfileRepo: repo}}

	req := &enrollmentModels.Request{
		GuardianAccountID: int64Ptr(callerAccount),
		GuardianEmail:     "victim@example.test",
		GuardianFirstName: "Anna",
		GuardianLastName:  "Antragsteller",
	}

	got, wasNew, err := svc.resolveGuardianProfile(context.Background(), req)
	require.ErrorIs(t, err, ErrGuardianAccountMismatch)
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
	own := &usersModels.GuardianProfile{FirstName: "Anna", LastName: "Antragsteller", AccountID: int64Ptr(callerAccount)}
	own.ID = 5
	// A colliding email owned by someone else exists too; the account-first
	// resolution must never consult it.
	other := &usersModels.GuardianProfile{FirstName: "Vera", LastName: "Opfer", AccountID: int64Ptr(int64(20))}
	other.ID = 99
	repo := &stubGuardianProfileRepo{
		byAccount: map[int64]*usersModels.GuardianProfile{callerAccount: own},
		byEmail:   map[string]*usersModels.GuardianProfile{"victim@example.test": other},
	}
	svc := &decisionService{DecisionServiceConfig: DecisionServiceConfig{GuardianProfileRepo: repo}}

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
	granted := identityaccess.GuardianTenantAccess{AccountID: accountID, TenantID: tenant.FromContext(ctx), RoleAssigned: true}
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
	foreign := &usersModels.GuardianProfile{FirstName: "Vera", LastName: "Opfer"} // AccountID nil
	foreign.ID = 77
	repo := &stubGuardianProfileRepo{
		byAccount: map[int64]*usersModels.GuardianProfile{},
		byEmail:   map[string]*usersModels.GuardianProfile{"victim@example.test": foreign},
	}
	accounts := &stubGuardianAccess{byID: map[int64]identityaccess.Account{
		callerAccount: {ID: callerAccount, Email: "caller@example.test"},
	}}
	svc := &decisionService{DecisionServiceConfig: DecisionServiceConfig{
		GuardianProfileRepo: repo,
		GuardianAccess:      accounts,
	}}

	req := &enrollmentModels.Request{
		GuardianAccountID: int64Ptr(callerAccount),
		GuardianEmail:     "victim@example.test",
		GuardianFirstName: "Anna",
		GuardianLastName:  "Antragsteller",
	}

	got, wasNew, err := svc.resolveGuardianProfile(context.Background(), req)
	require.ErrorIs(t, err, ErrGuardianAccountMismatch)
	assert.Nil(t, got)
	assert.False(t, wasNew)
	assert.Zero(t, repo.created, "must not create a profile on a rejected claim")
}

// The same shape with the caller's OWN address is the legitimate claim: an
// admin-created profile for this parent at a new school.
func TestResolveGuardianProfile_AllowsOwnUnclaimedEmailProfile(t *testing.T) {
	t.Parallel()

	const callerAccount = int64(10)
	own := &usersModels.GuardianProfile{FirstName: "Anna", LastName: "Antragsteller"} // AccountID nil
	own.ID = 7
	repo := &stubGuardianProfileRepo{
		byAccount: map[int64]*usersModels.GuardianProfile{},
		byEmail:   map[string]*usersModels.GuardianProfile{"anna@example.test": own},
	}
	accounts := &stubGuardianAccess{byID: map[int64]identityaccess.Account{
		callerAccount: {ID: callerAccount, Email: "Anna@Example.test"}, // case-insensitive match
	}}
	svc := &decisionService{DecisionServiceConfig: DecisionServiceConfig{
		GuardianProfileRepo: repo,
		GuardianAccess:      accounts,
	}}

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
// not-found sentinel may fall through.
func TestResolveGuardianProfile_PropagatesAccountLookupFailure(t *testing.T) {
	t.Parallel()

	const callerAccount = int64(10)
	other := &usersModels.GuardianProfile{FirstName: "Vera", LastName: "Opfer"}
	other.ID = 99
	repo := &stubGuardianProfileRepo{
		byAccount:    map[int64]*usersModels.GuardianProfile{},
		byEmail:      map[string]*usersModels.GuardianProfile{"anna@example.test": other},
		byAccountErr: errors.New("connection reset by peer"),
	}
	svc := &decisionService{DecisionServiceConfig: DecisionServiceConfig{GuardianProfileRepo: repo}}

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
	unclaimed := &usersModels.GuardianProfile{FirstName: "Anna", LastName: "Antragsteller"} // AccountID nil
	unclaimed.ID = 7
	repo := &stubGuardianProfileRepo{
		byAccount: map[int64]*usersModels.GuardianProfile{},
		byEmail:   map[string]*usersModels.GuardianProfile{"anna@example.test": unclaimed},
	}
	svc := &decisionService{DecisionServiceConfig: DecisionServiceConfig{GuardianProfileRepo: repo}}

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
	unclaimed := &usersModels.GuardianProfile{FirstName: "Anna", LastName: "Antragsteller"} // AccountID nil
	unclaimed.ID = 7
	repo := &stubGuardianProfileRepo{
		byAccount: map[int64]*usersModels.GuardianProfile{},
		byEmail:   map[string]*usersModels.GuardianProfile{"anna@example.test": unclaimed},
	}
	svc := &decisionService{DecisionServiceConfig: DecisionServiceConfig{
		GuardianProfileRepo: repo,
		GuardianAccess:      &stubGuardianAccess{byID: map[int64]identityaccess.Account{}},
		Logger:              slog.Default(),
	}}

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
	guardian := &usersModels.GuardianProfile{
		FirstName:  "Anna",
		LastName:   "Antragsteller",
		AccountID:  int64Ptr(accountID),
		HasAccount: true,
	}
	guardian.ID = 5

	access := &stubGuardianAccess{byID: map[int64]identityaccess.Account{}}
	svc := &decisionService{DecisionServiceConfig: DecisionServiceConfig{
		GuardianAccess: access,
		Logger:         slog.Default(),
	}}

	ctx := tenant.WithTenantID(context.Background(), tenantID)
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

	guardian := &usersModels.GuardianProfile{AccountID: int64Ptr(4242), HasAccount: true}
	guardian.ID = 5
	svc := &decisionService{DecisionServiceConfig: DecisionServiceConfig{}}
	ctx := tenant.WithTenantID(context.Background(), 77)
	err := svc.attachGuardianAccountIfPresent(ctx, &enrollmentModels.Request{}, guardian, false)
	require.ErrorIs(t, err, errDecisionGuardianAccessRequired)
}
