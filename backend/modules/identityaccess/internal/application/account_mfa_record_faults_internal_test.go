package application

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/ports"
)

// The second factor must never fail open. These tests break one statement of
// the account MFA record port and drive the composed capability and the real
// login gate over it: everything around the failing statement still works,
// which is the situation a fail-open bug needs to show itself.
//
// Before the fix a failure was silently read as "MFA off" and a full session
// token pair was issued: an attacker who could DoS the settings table could
// downgrade MFA-required schools to no MFA at all.

// memoryMFARecords keeps the account MFA rows the scenarios below touch.
type memoryMFARecords struct {
	ports.AccountMFARecords
	accounts    map[int64]domain.AccountIdentity
	credentials map[int64]domain.AccountMFACredential
	overrides   map[int64]domain.AccountMFAOverride
	challenges  []domain.AccountMFAChallenge
}

func newMemoryMFARecords(accounts ...domain.AccountIdentity) *memoryMFARecords {
	records := &memoryMFARecords{
		accounts:    map[int64]domain.AccountIdentity{},
		credentials: map[int64]domain.AccountMFACredential{},
		overrides:   map[int64]domain.AccountMFAOverride{},
	}
	for _, account := range accounts {
		records.accounts[account.ID] = account
	}
	return records
}

func (r *memoryMFARecords) FindAccountIdentity(_ context.Context, accountID int64) (domain.AccountIdentity, bool, error) {
	account, found := r.accounts[accountID]
	return account, found, nil
}

func (r *memoryMFARecords) ResetMFAAttempts(_ context.Context, accountID int64) error {
	account := r.accounts[accountID]
	account.MFAAttempts, account.MFALockedUntil = 0, nil
	r.accounts[accountID] = account
	return nil
}

func (r *memoryMFARecords) FindCredential(_ context.Context, accountID int64) (domain.AccountMFACredential, bool, error) {
	credential, found := r.credentials[accountID]
	return credential, found, nil
}

func (r *memoryMFARecords) CreateCredential(_ context.Context, credential domain.AccountMFACredential) error {
	credential.ID = int64(len(r.credentials) + 1)
	r.credentials[credential.AccountID] = credential
	return nil
}

func (r *memoryMFARecords) TouchCredential(context.Context, int64, time.Time) error { return nil }

func (r *memoryMFARecords) CreateChallenge(_ context.Context, challenge domain.AccountMFAChallenge) (domain.AccountMFAChallenge, error) {
	challenge.ID = int64(len(r.challenges) + 1)
	if challenge.CreatedAt.IsZero() {
		challenge.CreatedAt = time.Now()
	}
	r.challenges = append(r.challenges, challenge)
	return challenge, nil
}

func (r *memoryMFARecords) ConsumeChallenge(_ context.Context, id int64, consumedAt time.Time) error {
	for index := range r.challenges {
		if r.challenges[index].ID == id && r.challenges[index].ConsumedAt == nil {
			r.challenges[index].ConsumedAt = &consumedAt
			return nil
		}
	}
	return errors.New("mark mfa email challenge consumed: expected 1 rows affected, got 0")
}

func (r *memoryMFARecords) CountChallengesSince(_ context.Context, accountID int64, since time.Time) (int, error) {
	count := 0
	for _, challenge := range r.challenges {
		if challenge.AccountID == accountID && !challenge.CreatedAt.Before(since) {
			count++
		}
	}
	return count, nil
}

func (r *memoryMFARecords) FindActiveChallengeForAccount(_ context.Context, id, accountID int64) (domain.AccountMFAChallenge, bool, error) {
	for _, challenge := range r.challenges {
		if challenge.ID == id && challenge.AccountID == accountID && challenge.ConsumedAt == nil && challenge.ExpiresAt.After(time.Now()) {
			return challenge, true, nil
		}
	}
	return domain.AccountMFAChallenge{}, false, nil
}

func (r *memoryMFARecords) FindActiveChallengeInScope(_ context.Context, accountID, tenantID int64, scope string) (domain.AccountMFAChallenge, bool, error) {
	for _, challenge := range r.challenges {
		if challenge.AccountID == accountID && challenge.Scope == scope && (tenantID <= 0 || challenge.TenantID == tenantID) &&
			challenge.ConsumedAt == nil && challenge.ExpiresAt.After(time.Now()) {
			return challenge, true, nil
		}
	}
	return domain.AccountMFAChallenge{}, false, nil
}

func (r *memoryMFARecords) FindGlobalOverride(_ context.Context, accountID int64) (domain.AccountMFAOverride, bool, error) {
	override, found := r.overrides[accountID]
	return override, found, nil
}

func (r *memoryMFARecords) FindTenantOverride(context.Context, int64, int64) (domain.AccountMFAOverride, bool, error) {
	return domain.AccountMFAOverride{}, false, nil
}

// failingMFARecords replaces exactly one statement of the record port and
// leaves the rest untouched.
type failingMFARecords struct {
	ports.AccountMFARecords
	countChallengesSince func(ctx context.Context, accountID int64, since time.Time) (int, error)
	consumeChallenge     func(ctx context.Context, id int64, consumedAt time.Time) error
	findCredential       func(ctx context.Context, accountID int64) (domain.AccountMFACredential, bool, error)
	findGlobalOverride   func(ctx context.Context, accountID int64) (domain.AccountMFAOverride, bool, error)
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

func (r failingMFARecords) FindCredential(ctx context.Context, accountID int64) (domain.AccountMFACredential, bool, error) {
	if r.findCredential != nil {
		return r.findCredential(ctx, accountID)
	}
	return r.AccountMFARecords.FindCredential(ctx, accountID)
}

func (r failingMFARecords) FindGlobalOverride(ctx context.Context, accountID int64) (domain.AccountMFAOverride, bool, error) {
	if r.findGlobalOverride != nil {
		return r.findGlobalOverride(ctx, accountID)
	}
	return r.AccountMFARecords.FindGlobalOverride(ctx, accountID)
}

// optionalMFASettings is a school with the default MFA settings.
type optionalMFASettings struct{}

func (optionalMFASettings) MFAMode(context.Context, int64) (string, error) { return "optional", nil }
func (optionalMFASettings) MFAModeInTx(context.Context, int64) (string, error) {
	return "optional", nil
}
func (optionalMFASettings) TrustedDeviceEnabled(context.Context, int64) (bool, error) {
	return true, nil
}
func (optionalMFASettings) TrustedDeviceDays(context.Context, int64) (int, error) { return 30, nil }
func (optionalMFASettings) LockoutThreshold(context.Context, int64) (int, error)  { return 5, nil }
func (optionalMFASettings) LockoutDuration(context.Context, int64) (time.Duration, error) {
	return 15 * time.Minute, nil
}

// accountMFAFixture is the composed account second factor over the given
// record port, with the challenge codec it parses tokens with.
type accountMFAFixture struct {
	capability identityaccess.AccountMFA
	codec      *fakeMFAChallengeCodec
}

func newAccountMFAFixture(t *testing.T, records ports.AccountMFARecords) accountMFAFixture {
	t.Helper()
	codec := &fakeMFAChallengeCodec{parse: map[string]domain.MFAChallengeClaims{}}
	flows, err := NewAccountMFAFlows(AccountMFAFlowDependencies{
		Records: records, Settings: optionalMFASettings{}, Codec: codec, Codes: fakeShortCodes{},
		Mail: &fakeMFAMail{}, Audit: &fakeMFAAudit{}, Runtime: &fakeRuntime{}, Secret: []byte("trusted-device-secret"),
	})
	require.NoError(t, err)
	return accountMFAFixture{capability: ComposedAccountMFA{Flows: flows}, codec: codec}
}

// The rate-limit count is the only statement that fails below; everything
// else behaves normally, which is the situation the fail-open bug needed to
// show itself: challenge creation and mail dispatch succeed regardless of
// whether that count could be read.
func TestAccountMFA_StartChallenge_RateLimitLookupFails_IssuesNoCode(t *testing.T) {
	t.Parallel()

	native := newMemoryMFARecords(domain.AccountIdentity{ID: 1, Email: "mfa-ratelimit-blind@example.com", Active: true})
	svc := newAccountMFAFixture(t, failingMFARecords{AccountMFARecords: native,
		countChallengesSince: func(context.Context, int64, time.Time) (int, error) {
			return 0, errors.New("rate-limit count unavailable")
		},
	}).capability

	challengeToken, err := svc.StartMFAChallenge(
		context.Background(), 1, 0, identityaccess.MFAChallengeScopeTenant, net.ParseIP("127.0.0.1"),
	)

	require.Error(t, err)
	assert.ErrorIs(t, err, identityaccess.ErrMFAStatusUnavailable,
		"an unreadable rate-limit count must refuse the code, not wave it through")
	assert.Empty(t, challengeToken)

	issued, err := native.CountChallengesSince(context.Background(), 1, time.Now().Add(-time.Hour))
	require.NoError(t, err)
	assert.Zero(t, issued, "a refused challenge must leave no code behind")
}

// raceLosingConsume forces the consumption to report the "0 rows affected"
// outcome a concurrent race-loser observes. Every other statement runs
// against the records, so the rest of the verify pipeline sees the genuine
// row the test created up front.
type raceLosingConsume struct{ markCalls int }

func (r *raceLosingConsume) consume(context.Context, int64, time.Time) error {
	r.markCalls++
	return errors.New("mark mfa email challenge consumed: expected 1 rows affected, got 0")
}

// TestAccountMFA_VerifyChallenge_RaceLoserRejected exercises Item #4 from the
// PR #1430 review: the previous code logged-and-continued when MarkConsumed
// returned "0 rows affected" (the deterministic signal that a concurrent
// verify already consumed the same single-use code) and proceeded to mint a
// VerifiedChallenge. That meant two racing requests both completed login on
// one code, defeating single-use. The fix refuses the loser with
// ErrMFACodeInvalid.
func TestAccountMFA_VerifyChallenge_RaceLoserRejected(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	native := newMemoryMFARecords(domain.AccountIdentity{ID: 1, Email: "mfa-race-loser@example.com", Active: true})
	stub := &raceLosingConsume{}
	fixture := newAccountMFAFixture(t, failingMFARecords{AccountMFARecords: native, consumeChallenge: stub.consume})
	svc := fixture.capability

	require.NoError(t, svc.EnrollMFA(ctx, 1))

	// The test writes its own challenge with a known hash so VerifyShortCode
	// succeeds and the flow reaches MarkConsumed.
	plaintext := "654321"
	hash, err := fakeShortCodes{}.HashShortCode(plaintext)
	require.NoError(t, err)
	challenge, err := native.CreateChallenge(ctx, domain.AccountMFAChallenge{
		AccountID: 1,
		Scope:     domain.MFAChallengeScopeTenant,
		CodeHash:  hash,
		ExpiresAt: time.Now().Add(domain.MFAChallengeTTL),
		IPAddress: net.ParseIP("203.0.113.99"),
	})
	require.NoError(t, err)

	// ChallengeID mirrors what StartChallenge stamps: the verify path resolves
	// the exact row the token names, so a hand-built token must name it too.
	fixture.codec.parse["challenge-jwt"] = domain.MFAChallengeClaims{
		AccountID: 1, Scope: domain.MFAChallengeScopeTenant, ChallengeID: challenge.ID,
	}

	verified, err := svc.VerifyMFAChallenge(ctx, "challenge-jwt", plaintext)
	assert.Zero(t, verified, "race-loser must not get a verified challenge")
	assert.ErrorIs(t, err, identityaccess.ErrMFACodeInvalid,
		"MarkConsumed-failure must surface as the same generic invalid-code error "+
			"(no info leak distinguishing race-loser from wrong-code attacker)")
	assert.Equal(t, 1, stub.markCalls,
		"VerifyChallenge must call MarkConsumed exactly once before refusing")
}

// TestAccountMFA_VerifyCodeForAccount_RaceLoserRejected mirrors the previous
// test for the JWT-less code path used by enrollment confirmation. The same
// fail-closed-on-consume-failure rule applies.
func TestAccountMFA_VerifyCodeForAccount_RaceLoserRejected(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	native := newMemoryMFARecords(domain.AccountIdentity{ID: 1, Email: "mfa-race-loser-jwt-less@example.com", Active: true})
	stub := &raceLosingConsume{}
	svc := newAccountMFAFixture(t, failingMFARecords{AccountMFARecords: native, consumeChallenge: stub.consume}).capability

	plaintext := "987654"
	hash, err := fakeShortCodes{}.HashShortCode(plaintext)
	require.NoError(t, err)
	_, err = native.CreateChallenge(ctx, domain.AccountMFAChallenge{
		AccountID: 1,
		Scope:     domain.MFAChallengeScopeTenant,
		CodeHash:  hash,
		ExpiresAt: time.Now().Add(domain.MFAChallengeTTL),
	})
	require.NoError(t, err)

	err = svc.VerifyMFACodeForAccount(ctx, 1, 0, plaintext, identityaccess.MFAChallengeScopeTenant)
	assert.ErrorIs(t, err, identityaccess.ErrMFACodeInvalid,
		"VerifyCodeForAccount must refuse the race-loser with the generic invalid-code error")
	assert.Equal(t, 1, stub.markCalls)
}

// newGatedLogin is the account login whose gate is the composed second
// factor over the given records, as the composition wires it.
func newGatedLogin(t *testing.T, records ports.AccountMFARecords) *authFixture {
	t.Helper()
	f := newAuthFixture(t)
	f.seedStaff(1, 10, "user")
	f.auth.mfa = CapabilityGate{Capability: newAccountMFAFixture(t, records).capability}
	return f
}

// A failing override lookup is the settings-side half of the gate: it cannot
// be decided, so the login is refused with ErrMFAStatusUnavailable rather
// than downgraded.
func TestLoginWithMFAGate_PolicyInfraError_ReturnsMFAStatusUnavailable(t *testing.T) {
	t.Parallel()

	native := newMemoryMFARecords(domain.AccountIdentity{ID: 1, Email: "user1@example.com", Active: true})
	f := newGatedLogin(t, failingMFARecords{AccountMFARecords: native,
		findGlobalOverride: func(context.Context, int64) (domain.AccountMFAOverride, bool, error) {
			return domain.AccountMFAOverride{}, false, errors.New("settings DB timed out")
		},
	})

	result, err := f.auth.LoginWithMFAGate(context.Background(), "user1@example.com", "secret", "127.0.0.1", "ua-test", "", "")
	assert.Nil(t, result, "the infra-error path must not issue tokens")
	require.Error(t, err)

	var authErr *identityaccess.AuthenticationError
	require.True(t, errors.As(authenticationError(err), &authErr), "must wrap as AuthError so handleLoginError can switch on it")
	assert.True(t, errors.Is(authErr.Err, identityaccess.ErrMFAStatusUnavailable),
		"an undecidable policy must surface as ErrMFAStatusUnavailable")
}

// The enrollment lookup is the other half. Only a missing row means "not
// enrolled"; anything else refuses this login.
func TestLoginWithMFAGate_EnrollmentInfraError_ReturnsMFAStatusUnavailable(t *testing.T) {
	t.Parallel()

	native := newMemoryMFARecords(domain.AccountIdentity{ID: 1, Email: "user1@example.com", Active: true})
	native.overrides[1] = domain.AccountMFAOverride{AccountID: 1, Override: domain.MFAAdminOverrideForceOn}
	f := newGatedLogin(t, failingMFARecords{AccountMFARecords: native,
		findCredential: func(context.Context, int64) (domain.AccountMFACredential, bool, error) {
			return domain.AccountMFACredential{}, false, errors.New("mfa_credentials lookup failed: conn reset")
		},
	})

	result, err := f.auth.LoginWithMFAGate(context.Background(), "user1@example.com", "secret", "127.0.0.1", "ua-test", "", "")
	assert.Nil(t, result, "the infra-error path must not issue tokens")
	require.Error(t, err)

	var authErr *identityaccess.AuthenticationError
	require.True(t, errors.As(authenticationError(err), &authErr), "must wrap as AuthError")
	assert.True(t, errors.Is(authErr.Err, identityaccess.ErrMFAStatusUnavailable),
		"an unreadable enrollment must surface as ErrMFAStatusUnavailable")
}
