package behavior_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/api"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	identityPorts "github.com/moto-nrw/project-phoenix/modules/identityaccess/ports"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken"
	owners "github.com/moto-nrw/project-phoenix/roots/owners"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// The second factor and both portals' passkey ceremonies live in Identity &
// Access since #3331. The behaviour tests in this directory still run against
// a real test database and the same repositories the service root composes,
// so what they pin — the lockout, the single-use code, the trusted-device
// scope and the admin overrides — is what production runs.

// newMFATestModule composes the module the way the service root does.
func newMFATestModule(t *testing.T, db *bun.DB, options ...api.AuthTestOption) api.AuthTestModule {
	t.Helper()
	module, err := api.NewAuthTestModule(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db), options...)
	require.NoError(t, err)
	return module
}

// newTestMFAService wires a test-DB-backed second factor with a successful
// mailer, so StartMFAChallenge exercises the delivered-code path.
func newTestMFAService(t *testing.T, options ...api.AuthTestOption) (identityaccess.AccountMFA, owners.MembershipTestRepositories, *bun.DB) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	module := newMFATestModule(t, db, withDeliveringMailer(options)...)
	return module.MFA, owners.MustNewMembershipTestRepositories(testpkg.NewDatabase(t, db)), db
}

// withDeliveringMailer prepends a mailer that accepts, because the composed
// default only logs and then reports the transport unavailable — which the
// fail-closed code send correctly refuses on. A caller that supplies its own
// mailer still wins, options being applied in order.
func withDeliveringMailer(options []api.AuthTestOption) []api.AuthTestOption {
	return append([]api.AuthTestOption{
		api.WithAuthTestMailer(testpkg.NewCapturingMailer()),
		api.WithAuthTestMFABackoff(time.Millisecond),
	}, options...)
}

// challengeTokenAuth is Identity's session token adapter over the signing
// configuration the composed module mints and parses the challenge JWTs with,
// so a test can read the claims it carries.
func challengeTokenAuth(t *testing.T, module api.AuthTestModule) *sessiontoken.TokenAuth {
	t.Helper()
	signing := module.SessionSigning
	require.NotEmpty(t, signing.Secret)
	tokenAuth, err := sessiontoken.NewTokenAuthWithDurations(signing.Secret, signing.AccessExpiry, signing.RefreshExpiry)
	require.NoError(t, err)
	return tokenAuth
}

// newMFATestScenario composes the second factor together with the material a
// behaviour test drives it with: the repositories it wrote through, the
// signer it mints challenge tokens with and an account to gate.
type mfaTestScenario struct {
	MFA       identityaccess.AccountMFA
	Repos     owners.MembershipTestRepositories
	TokenAuth *sessiontoken.TokenAuth
	DB        *bun.DB
	AccountID int64
}

func newMFATestScenario(t *testing.T, options ...api.AuthTestOption) mfaTestScenario {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	module := newMFATestModule(t, db, withDeliveringMailer(options)...)
	account := testpkg.CreateTestAccount(t, db, "mfa-extra")
	return mfaTestScenario{
		MFA: module.MFA, Repos: owners.MustNewMembershipTestRepositories(testpkg.NewDatabase(t, db)), TokenAuth: challengeTokenAuth(t, module),
		DB: db, AccountID: account.ID,
	}
}

// newGatedAuthService composes the auth service over a supplied second
// factor. The gate moved into the module with #3331, so a test that drives
// the branches the login decides between names the capability when it builds
// the service instead of setting it afterwards.
func newGatedAuthService(t *testing.T, db *bun.DB, mfa identityaccess.AccountMFA) gatedAuthService {
	t.Helper()
	module := newMFATestModule(t, db, withDeliveringMailer([]api.AuthTestOption{
		api.WithAuthTestMFACapability(mfa),
	})...)
	return gatedAuthService{
		testAuthService: newFixtureAuthService(t, db, module.Auth),
		TokenAuth:       challengeTokenAuth(t, module),
	}
}

// gatedAuthService is the composed service together with the signer the
// module minted its tokens with, so a test can decode what the login
// returned instead of re-deriving a secret. It carries the fixture's account
// creation too, so the gated tests build their accounts the way every other
// test in this package does.
type gatedAuthService struct {
	testAuthService
	TokenAuth *sessiontoken.TokenAuth
}

// failingMFARecords replaces exactly one statement of the composed record
// port and leaves the rest untouched, which is the situation a fail-open bug
// needs to show itself: everything around the failing read still works.
type failingMFARecords struct {
	identityPorts.AccountMFARecords
	countChallengesSince func(ctx context.Context, accountID int64, since time.Time) (int, error)
	consumeChallenge     func(ctx context.Context, id int64, consumedAt time.Time) error
	findCredential       func(ctx context.Context, accountID int64) (identityaccess.AccountMFACredential, bool, error)
	findGlobalOverride   func(ctx context.Context, accountID int64) (identityaccess.AccountMFAOverride, bool, error)
}

func (r failingMFARecords) CountChallengesSince(ctx context.Context, accountID int64, since time.Time) (int, error) {
	if r.countChallengesSince != nil {
		return r.countChallengesSince(ctx, accountID, since)
	}
	return r.AccountMFARecords.CountChallengesSince(ctx, accountID, since)
}

func (r failingMFARecords) ConsumeChallenge(ctx context.Context, id int64, consumedAt time.Time) error {
	if r.consumeChallenge != nil {
		return r.consumeChallenge(ctx, id, consumedAt)
	}
	return r.AccountMFARecords.ConsumeChallenge(ctx, id, consumedAt)
}

func (r failingMFARecords) FindCredential(ctx context.Context, accountID int64) (identityaccess.AccountMFACredential, bool, error) {
	if r.findCredential != nil {
		return r.findCredential(ctx, accountID)
	}
	return r.AccountMFARecords.FindCredential(ctx, accountID)
}

func (r failingMFARecords) FindGlobalOverride(ctx context.Context, accountID int64) (identityaccess.AccountMFAOverride, bool, error) {
	if r.findGlobalOverride != nil {
		return r.findGlobalOverride(ctx, accountID)
	}
	return r.AccountMFARecords.FindGlobalOverride(ctx, accountID)
}

// withFailingMFARecords composes the module with one statement replaced.
func withFailingMFARecords(replace failingMFARecords) api.AuthTestOption {
	return api.WithAuthTestMFARecords(func(records identityPorts.AccountMFARecords) identityPorts.AccountMFARecords {
		replace.AccountMFARecords = records
		return replace
	})
}

// requiredAdminsPolicy is the verdict a school with security.mfa_mode
// "required_admins" produces: the second factor is required of the admin
// role and of nobody else. The gate holds the policy as an interface, so a
// test that drives a branch supplies the predicate directly.
type mfaPolicyFunc func(roleNames []string) bool

func (f mfaPolicyFunc) RequiredFor(roleNames []string) bool { return f(roleNames) }

func requiredAdminsPolicy() identityaccess.MFAPolicy {
	return mfaPolicyFunc(func(roleNames []string) bool {
		for _, name := range roleNames {
			if strings.EqualFold(name, "admin") {
				return true
			}
		}
		return false
	})
}
