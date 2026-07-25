package enrollment

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	authModels "github.com/moto-nrw/project-phoenix/models/auth"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
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

// #1663: an authenticated submit whose email resolves to a DIFFERENT account's
// guardian profile must be rejected, not silently linked to that other account.
func TestResolveGuardianProfile_RejectsCrossAccountEmail(t *testing.T) {
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

// stubAccountRepo answers the single FindByID lookup the email-ownership check
// makes; the embedded interface panics on anything else.
type stubAccountRepo struct {
	authModels.AccountRepository
	byID map[int64]*authModels.Account
}

func (s *stubAccountRepo) FindByID(_ context.Context, id interface{}) (*authModels.Account, error) {
	if v, ok := id.(int64); ok {
		return s.byID[v], nil
	}
	return nil, nil
}

// #1663: an UNLINKED profile is claimable only by the account that owns its
// email. A logged-in parent typing a stranger's address at a school where they
// have no profile yet must not have that family's profile (and every child on
// it) attached to their account by the by-id attach in applyApproval.
func TestResolveGuardianProfile_RejectsForeignUnclaimedEmailProfile(t *testing.T) {
	const callerAccount = int64(10)
	foreign := &usersModels.GuardianProfile{FirstName: "Vera", LastName: "Opfer"} // AccountID nil
	foreign.ID = 77
	repo := &stubGuardianProfileRepo{
		byAccount: map[int64]*usersModels.GuardianProfile{},
		byEmail:   map[string]*usersModels.GuardianProfile{"victim@example.test": foreign},
	}
	accounts := &stubAccountRepo{byID: map[int64]*authModels.Account{
		callerAccount: {Email: "caller@example.test"},
	}}
	svc := &decisionService{DecisionServiceConfig: DecisionServiceConfig{
		GuardianProfileRepo: repo,
		AccountRepo:         accounts,
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
	const callerAccount = int64(10)
	own := &usersModels.GuardianProfile{FirstName: "Anna", LastName: "Antragsteller"} // AccountID nil
	own.ID = 7
	repo := &stubGuardianProfileRepo{
		byAccount: map[int64]*usersModels.GuardianProfile{},
		byEmail:   map[string]*usersModels.GuardianProfile{"anna@example.test": own},
	}
	accounts := &stubAccountRepo{byID: map[int64]*authModels.Account{
		callerAccount: {Email: "Anna@Example.test"}, // case-insensitive match
	}}
	svc := &decisionService{DecisionServiceConfig: DecisionServiceConfig{
		GuardianProfileRepo: repo,
		AccountRepo:         accounts,
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

// An unclaimed profile carrying the email (no account yet) is NOT a mismatch:
// approval's by-id attach later links it to the caller.
func TestResolveGuardianProfile_AllowsUnclaimedEmailProfile(t *testing.T) {
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

	got, _, err := svc.resolveGuardianProfile(context.Background(), req)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, unclaimed.ID, got.ID)
}

// --- attachGuardianAccountIfPresent: already-linked profiles ----------------

// stubAccountTenantRepo records the mapping writes the attach path performs.
type stubAccountTenantRepo struct {
	authModels.AccountTenantRepository
	ensured []*authModels.AccountTenant
	created []*authModels.AccountTenant
}

func (s *stubAccountTenantRepo) EnsureActive(_ context.Context, m *authModels.AccountTenant) error {
	s.ensured = append(s.ensured, m)
	return nil
}

func (s *stubAccountTenantRepo) Create(_ context.Context, m *authModels.AccountTenant) error {
	s.created = append(s.created, m)
	return nil
}

// stubRoleRepo answers the guardian-role lookup ensureGuardianRoleForTenant makes.
type stubRoleRepo struct {
	authModels.RoleRepository
	role *authModels.Role
}

func (s *stubRoleRepo) FindByName(_ context.Context, _ string) (*authModels.Role, error) {
	return s.role, nil
}

// stubAccountRoleRepo reports the guardian role as already assigned so the
// attach path needs no write.
type stubAccountRoleRepo struct {
	authModels.AccountRoleRepository
	existing *authModels.AccountRole
}

func (s *stubAccountRoleRepo) FindByAccountAndRole(_ context.Context, _, _ int64) (*authModels.AccountRole, error) {
	return s.existing, nil
}

// #1663 review: a guardian profile that already carries an account_id proves a
// LINK, not portal ACCESS — account_id survives an offboarding that flipped
// auth.account_tenants to inactive. Since pendingGuardianInvite sends nothing
// for a linked profile, an early return here would approve the child and leave
// the parent locked out. The approval must (re)assert the active mapping.
func TestAttachGuardianAccountIfPresent_ReactivatesLinkedAccountTenant(t *testing.T) {
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

	role := &authModels.Role{Name: "guardian"}
	role.ID = 9
	assignment := &authModels.AccountRole{AccountID: accountID, RoleID: role.ID}
	mappings := &stubAccountTenantRepo{}
	svc := &decisionService{DecisionServiceConfig: DecisionServiceConfig{
		AccountTenantRepo: mappings,
		AccountRoleRepo:   &stubAccountRoleRepo{existing: assignment},
		RoleRepo:          &stubRoleRepo{role: role},
	}}

	ctx := tenant.WithTenantID(context.Background(), tenantID)
	require.NoError(t, svc.attachGuardianAccountIfPresent(ctx, &enrollmentModels.Request{}, guardian, false))

	require.Len(t, mappings.ensured, 1,
		"an already-linked guardian must still get an ACTIVE account_tenants mapping")
	assert.Equal(t, accountID, mappings.ensured[0].AccountID)
	assert.Equal(t, tenantID, mappings.ensured[0].TenantID)
	assert.Equal(t, authModels.AccountTenantStatusActive, mappings.ensured[0].Status)
	assert.Empty(t, mappings.created,
		"Create is ON CONFLICT DO NOTHING and would leave an inactive row inactive")
}
