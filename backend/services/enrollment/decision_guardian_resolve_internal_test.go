package enrollment

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	authModels "github.com/moto-nrw/project-phoenix/models/auth"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
)

// stubGuardianProfileRepo implements only the GuardianProfileRepository methods
// resolveGuardianProfile touches; the embedded interface panics on any other
// call, which keeps the unit test honest about what the resolver depends on.
type stubGuardianProfileRepo struct {
	usersModels.GuardianProfileRepository
	byAccount map[int64]*usersModels.GuardianProfile
	byEmail   map[string]*usersModels.GuardianProfile
	created   int
	updated   int
}

func (s *stubGuardianProfileRepo) FindByAccountID(_ context.Context, accountID int64) (*usersModels.GuardianProfile, error) {
	if p, ok := s.byAccount[accountID]; ok {
		return p, nil
	}
	return nil, usersModels.ErrGuardianProfileNotFound
}

func (s *stubGuardianProfileRepo) FindByEmail(_ context.Context, email string) (*usersModels.GuardianProfile, error) {
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
