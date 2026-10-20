package behavior_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/api"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
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

// mfaRows arranges and inspects the account MFA rows of a behaviour test
// through SQL, so the composed flows stay the only production code a test
// drives. Each statement matches the module's own persistence.
type mfaRows struct{ db *bun.DB }

func mfaRecordRows(t *testing.T, db *bun.DB) mfaRows {
	t.Helper()
	return mfaRows{db: db}
}

const mfaChallengeColumns = "id, account_id, scope, tenant_id, code_hash, expires_at, consumed_at, ip_address, created_at, updated_at"

func (r mfaRows) CreateCredential(ctx context.Context, credential identityaccess.AccountMFACredential) error {
	_, err := r.db.NewRaw(`INSERT INTO auth.mfa_credentials (account_id, method, enrolled_at)
		VALUES (?, ?, CURRENT_TIMESTAMP)`, credential.AccountID, credential.Method).Exec(ctx)
	return err
}

func (r mfaRows) FindCredential(ctx context.Context, accountID int64) (identityaccess.AccountMFACredential, bool, error) {
	var row identityaccess.AccountMFACredential
	err := r.db.NewRaw(`SELECT id, account_id, method, enrolled_at, last_used_at, created_at, updated_at
		FROM auth.mfa_credentials WHERE account_id = ? LIMIT 1`, accountID).Scan(ctx, &row)
	if errors.Is(err, sql.ErrNoRows) {
		return identityaccess.AccountMFACredential{}, false, nil
	}
	return row, err == nil, err
}

func (r mfaRows) CreateChallenge(ctx context.Context, challenge identityaccess.AccountMFAChallenge) (identityaccess.AccountMFAChallenge, error) {
	var ipAddress *string
	if len(challenge.IPAddress) > 0 {
		value := challenge.IPAddress.String()
		ipAddress = &value
	}
	var row identityaccess.AccountMFAChallenge
	err := r.db.NewRaw(`INSERT INTO auth.mfa_email_challenges (account_id, scope, tenant_id, code_hash, expires_at, consumed_at, ip_address)
		VALUES (?, ?, NULLIF(?, 0), ?, ?, ?, ?) RETURNING `+mfaChallengeColumns,
		challenge.AccountID, challenge.Scope, challenge.TenantID, challenge.CodeHash, challenge.ExpiresAt, challenge.ConsumedAt, ipAddress).Scan(ctx, &row)
	return row, err
}

func (r mfaRows) ConsumeChallenge(ctx context.Context, id int64, consumedAt time.Time) error {
	result, err := r.db.NewRaw(`UPDATE auth.mfa_email_challenges SET consumed_at = ? WHERE id = ? AND consumed_at IS NULL`, consumedAt, id).Exec(ctx)
	if err != nil {
		return err
	}
	if affected, _ := result.RowsAffected(); affected != 1 {
		return fmt.Errorf("consume mfa challenge %d: %d rows affected", id, affected)
	}
	return nil
}

func (r mfaRows) CountChallengesSince(ctx context.Context, accountID int64, since time.Time) (int, error) {
	var count int
	err := r.db.NewRaw(`SELECT COUNT(*) FROM auth.mfa_email_challenges WHERE account_id = ? AND created_at >= ?`, accountID, since).Scan(ctx, &count)
	return count, err
}

func (r mfaRows) FindActiveChallengeForAccount(ctx context.Context, id, accountID int64) (identityaccess.AccountMFAChallenge, bool, error) {
	var row identityaccess.AccountMFAChallenge
	err := r.db.NewRaw(`SELECT `+mfaChallengeColumns+` FROM auth.mfa_email_challenges
		WHERE id = ? AND account_id = ? AND consumed_at IS NULL AND expires_at > ? LIMIT 1`, id, accountID, time.Now()).Scan(ctx, &row)
	return row, err == nil, err
}

func (r mfaRows) FindActiveChallengeInScope(ctx context.Context, accountID, tenantID int64, scope string) (identityaccess.AccountMFAChallenge, bool, error) {
	var row identityaccess.AccountMFAChallenge
	err := r.db.NewRaw(`SELECT `+mfaChallengeColumns+` FROM auth.mfa_email_challenges
		WHERE account_id = ? AND scope = ? AND (? <= 0 OR tenant_id = ?) AND consumed_at IS NULL AND expires_at > ?
		ORDER BY expires_at DESC LIMIT 1`, accountID, scope, tenantID, tenantID, time.Now()).Scan(ctx, &row)
	return row, err == nil, err
}

func (r mfaRows) FindGlobalOverride(ctx context.Context, accountID int64) (identityaccess.AccountMFAOverride, bool, error) {
	return r.findOverride(ctx, `account_id = ? AND tenant_id IS NULL`, accountID)
}

func (r mfaRows) FindTenantOverride(ctx context.Context, accountID, tenantID int64) (identityaccess.AccountMFAOverride, bool, error) {
	return r.findOverride(ctx, `account_id = ? AND tenant_id = ?`, accountID, tenantID)
}

func (r mfaRows) findOverride(ctx context.Context, where string, args ...any) (identityaccess.AccountMFAOverride, bool, error) {
	var row identityaccess.AccountMFAOverride
	err := r.db.NewRaw(`SELECT id, account_id, tenant_id, override, set_by, set_by_type, reason
		FROM auth.mfa_overrides WHERE `+where+` LIMIT 1`, args...).Scan(ctx, &row)
	if errors.Is(err, sql.ErrNoRows) {
		return identityaccess.AccountMFAOverride{}, false, nil
	}
	return row, err == nil, err
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
