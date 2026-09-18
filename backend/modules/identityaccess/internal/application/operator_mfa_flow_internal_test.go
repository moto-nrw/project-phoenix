package application

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/ports"
)

// The operator second factor moved here with the flows (#3331). These tests
// pin the four rules a store failure must not bend: an unreadable
// enrollment refuses the login instead of reading as "not enrolled", an
// unreadable send-cap count refuses the code instead of issuing one, a
// consumption that did not apply refuses the verification instead of minting
// a second session from one code, and a failed step of the disable cascade
// rolls the whole cascade back.

var errOperatorMFAStore = errors.New("operator mfa store down")

// --- doubles --------------------------------------------------------------

// fakeOperatorMFAStore serves the operator MFA rows. Only the statements the
// flows drive are modelled; each can be made to fail on its own.
type fakeOperatorMFAStore struct {
	credential      *domain.OperatorMFACredential
	challenge       *domain.OperatorMFAChallenge
	trustedDevices  []domain.OperatorTrustedDevice
	challengeCount  int
	findCredErr     error
	countErr        error
	consumeErr      error
	revokeAllErr    error
	deleteCredCalls int
	consumeCalls    int
	activateCalls   int
}

func (s *fakeOperatorMFAStore) FindOperatorMFACredential(context.Context, int64) (domain.OperatorMFACredential, bool, domain.OperationStats, error) {
	if s.findCredErr != nil {
		return domain.OperatorMFACredential{}, false, domain.OperationStats{}, s.findCredErr
	}
	if s.credential == nil {
		return domain.OperatorMFACredential{}, false, domain.OperationStats{}, nil
	}
	return *s.credential, true, domain.OperationStats{}, nil
}

func (s *fakeOperatorMFAStore) InsertOperatorMFACredential(_ context.Context, credential domain.OperatorMFACredential) (domain.OperatorMFACredential, domain.OperationStats, error) {
	credential.ID = 1
	s.credential = &credential
	return credential, domain.OperationStats{}, nil
}

func (s *fakeOperatorMFAStore) TouchOperatorMFACredential(context.Context, int64, time.Time) (domain.OperationStats, error) {
	return domain.OperationStats{}, nil
}

func (s *fakeOperatorMFAStore) DeleteOperatorMFACredentials(context.Context, int64) (domain.OperationStats, error) {
	s.deleteCredCalls++
	s.credential = nil
	return domain.OperationStats{}, nil
}

func (s *fakeOperatorMFAStore) InsertOperatorMFAChallenge(_ context.Context, challenge domain.OperatorMFAChallenge) (domain.OperatorMFAChallenge, domain.OperationStats, error) {
	challenge.ID = 7
	s.challenge = &challenge
	s.challengeCount++
	return challenge, domain.OperationStats{}, nil
}

func (s *fakeOperatorMFAStore) FindActiveOperatorMFAChallenge(context.Context, int64, time.Time) (domain.OperatorMFAChallenge, bool, domain.OperationStats, error) {
	if s.challenge == nil {
		return domain.OperatorMFAChallenge{}, false, domain.OperationStats{}, nil
	}
	return *s.challenge, true, domain.OperationStats{}, nil
}

func (s *fakeOperatorMFAStore) CountOperatorMFAChallengesSince(context.Context, int64, time.Time) (int, domain.OperationStats, error) {
	if s.countErr != nil {
		return 0, domain.OperationStats{}, s.countErr
	}
	return s.challengeCount, domain.OperationStats{}, nil
}

func (s *fakeOperatorMFAStore) ActivateOperatorMFAChallenge(context.Context, int64) (bool, domain.OperationStats, error) {
	s.activateCalls++
	return true, domain.OperationStats{}, nil
}

func (s *fakeOperatorMFAStore) ConsumeOperatorMFAChallenge(context.Context, int64, time.Time) (bool, domain.OperationStats, error) {
	s.consumeCalls++
	if s.consumeErr != nil {
		return false, domain.OperationStats{}, s.consumeErr
	}
	return true, domain.OperationStats{}, nil
}

func (s *fakeOperatorMFAStore) InsertOperatorTrustedDevice(_ context.Context, device domain.OperatorTrustedDevice) (domain.OperatorTrustedDevice, domain.OperationStats, error) {
	device.ID = int64(len(s.trustedDevices) + 1)
	s.trustedDevices = append(s.trustedDevices, device)
	return device, domain.OperationStats{}, nil
}

func (s *fakeOperatorMFAStore) FindActiveOperatorTrustedDevice(_ context.Context, _ int64, tokenHash string, _ time.Time) (domain.OperatorTrustedDevice, bool, domain.OperationStats, error) {
	for _, device := range s.trustedDevices {
		if device.TokenHash == tokenHash {
			return device, true, domain.OperationStats{}, nil
		}
	}
	return domain.OperatorTrustedDevice{}, false, domain.OperationStats{}, nil
}

func (s *fakeOperatorMFAStore) ListActiveOperatorTrustedDevices(context.Context, int64, time.Time) ([]domain.OperatorTrustedDevice, domain.OperationStats, error) {
	return s.trustedDevices, domain.OperationStats{}, nil
}

func (s *fakeOperatorMFAStore) TouchOperatorTrustedDevice(context.Context, int64, time.Time) (domain.OperationStats, error) {
	return domain.OperationStats{}, nil
}

func (s *fakeOperatorMFAStore) RevokeOperatorTrustedDevice(context.Context, int64, time.Time) (bool, domain.OperationStats, error) {
	s.trustedDevices = nil
	return true, domain.OperationStats{}, nil
}

func (s *fakeOperatorMFAStore) RevokeOperatorTrustedDevices(context.Context, int64, time.Time) (domain.OperationStats, error) {
	if s.revokeAllErr != nil {
		return domain.OperationStats{}, s.revokeAllErr
	}
	s.trustedDevices = nil
	return domain.OperationStats{}, nil
}

var _ ports.OperatorMFAStore = (*fakeOperatorMFAStore)(nil)

// fakeMFAChallengeCodec mints and parses the challenge tokens without a JWT.
type fakeMFAChallengeCodec struct {
	issued domain.MFAChallengeClaims
	parse  map[string]domain.MFAChallengeClaims
	err    error
}

func (c *fakeMFAChallengeCodec) IssueChallengeToken(claims domain.MFAChallengeClaims, _ time.Duration) (string, error) {
	c.issued = claims
	return "challenge-token", nil
}

func (c *fakeMFAChallengeCodec) ParseChallengeToken(token string) (domain.MFAChallengeClaims, error) {
	if c.err != nil {
		return domain.MFAChallengeClaims{}, c.err
	}
	claims, ok := c.parse[token]
	if !ok {
		return domain.MFAChallengeClaims{}, errors.New("unknown token")
	}
	return claims, nil
}

// fakeShortCodes accepts one plaintext and nothing else.
type fakeShortCodes struct{ accept string }

func (fakeShortCodes) HashShortCode(plain string) (string, error) { return "hash:" + plain, nil }

func (c fakeShortCodes) VerifyShortCode(plain, encodedHash string) (bool, error) {
	if c.accept != "" {
		return plain == c.accept, nil
	}
	return encodedHash == "hash:"+plain, nil
}

// fakeMFAMail records what the flows sent and can refuse the synchronous
// delivery the code issue depends on.
type fakeMFAMail struct {
	codes   []domain.MFACodeMail
	devices []domain.TrustedDeviceMail
	err     error
}

func (m *fakeMFAMail) DeliverCode(_ context.Context, message domain.MFACodeMail) error {
	if m.err != nil {
		return m.err
	}
	m.codes = append(m.codes, message)
	return nil
}

func (m *fakeMFAMail) NotifyTrustedDeviceAdded(_ context.Context, message domain.TrustedDeviceMail) {
	m.devices = append(m.devices, message)
}

// fakeMFAAudit records the ledger entries the flows append.
type fakeMFAAudit struct {
	events    []domain.AuthEvent
	operators []domain.OperatorAuditEntry
}

func (a *fakeMFAAudit) RecordAuthEvent(_ context.Context, event domain.AuthEvent) error {
	a.events = append(a.events, event)
	return nil
}

func (a *fakeMFAAudit) RecordOperatorActionAsync(entry domain.OperatorAuditEntry) {
	a.operators = append(a.operators, entry)
}

func (a *fakeMFAAudit) operatorActions() []string {
	actions := make([]string, 0, len(a.operators))
	for _, entry := range a.operators {
		actions = append(actions, entry.Action)
	}
	return actions
}

// rollingBackRuntime models what an administrative transaction does to a
// failing cascade: the writes that already applied are undone. Without it a
// rollback assertion would only prove the fake kept its own state.
type rollingBackRuntime struct {
	*fakeRuntime
	store *fakeOperatorMFAStore
}

func (r *rollingBackRuntime) WithAdminTx(ctx context.Context, fn func(context.Context) error) error {
	credential := r.store.credential
	devices := append([]domain.OperatorTrustedDevice(nil), r.store.trustedDevices...)
	err := r.fakeRuntime.WithAdminTx(ctx, fn)
	if err != nil {
		r.store.credential = credential
		r.store.trustedDevices = devices
	}
	return err
}

// operatorMFAFixture is the composed operator second factor over the doubles.
type operatorMFAFixture struct {
	flows   *OperatorMFAFlows
	store   *fakeOperatorMFAStore
	codec   *fakeMFAChallengeCodec
	mail    *fakeMFAMail
	audit   *fakeMFAAudit
	runtime *fakeRuntime
}

func newOperatorMFAFixture(t *testing.T, operator domain.Operator, store *fakeOperatorMFAStore) *operatorMFAFixture {
	t.Helper()
	if store == nil {
		store = &fakeOperatorMFAStore{}
	}
	operatorStore := newFakeStore()
	if operator.ID != 0 {
		operatorStore.operators[operator.ID] = operator
	}
	runtime := &fakeRuntime{}
	service := New(operatorStore, operatorStore, operatorStore, fakeTransaction{}, runtime.TenantID, func(ports.Observation) {})
	codec := &fakeMFAChallengeCodec{parse: map[string]domain.MFAChallengeClaims{}}
	mail := &fakeMFAMail{}
	audit := &fakeMFAAudit{}
	flows, err := NewOperatorMFAFlows(service, OperatorMFAFlowDependencies{
		Records: NewOperatorMFA(service, store), Codec: codec, Codes: fakeShortCodes{},
		Mail: mail, Audit: audit, Runtime: &rollingBackRuntime{fakeRuntime: runtime, store: store},
		Secret: []byte("operator-mfa-secret"),
	})
	require.NoError(t, err)
	return &operatorMFAFixture{flows: flows, store: store, codec: codec, mail: mail, audit: audit, runtime: runtime}
}

// --- the enrollment lookup ------------------------------------------------

// An unreadable enrollment refuses the login. Reading it as "not enrolled"
// would silently downgrade an enrolled operator to the enrollment flow.
func TestOperatorMFAHasEnrollmentFailsClosedOnStoreError(t *testing.T) {
	t.Parallel()

	operator := domain.Operator{ID: 5, Email: "ops@example.test", DisplayName: "Ops", Active: true}
	fixture := newOperatorMFAFixture(t, operator, &fakeOperatorMFAStore{findCredErr: errOperatorMFAStore})

	enrolled, err := fixture.flows.HasEnrollment(context.Background(), operator.ID)
	require.ErrorIs(t, err, domain.ErrMFAStatusUnavailable)
	assert.False(t, enrolled)
}

// A missing enrollment row is the legitimate "not enrolled" signal every
// fresh operator hits.
func TestOperatorMFAHasEnrollmentMissingRowIsNotEnrolled(t *testing.T) {
	t.Parallel()

	operator := domain.Operator{ID: 6, Email: "ops@example.test", DisplayName: "Ops", Active: true}
	fixture := newOperatorMFAFixture(t, operator, nil)

	enrolled, err := fixture.flows.HasEnrollment(context.Background(), operator.ID)
	require.NoError(t, err)
	assert.False(t, enrolled)

	require.NoError(t, fixture.flows.Enroll(context.Background(), operator.ID))
	enrolled, err = fixture.flows.HasEnrollment(context.Background(), operator.ID)
	require.NoError(t, err)
	assert.True(t, enrolled)
	assert.Contains(t, fixture.audit.operatorActions(), domain.OperatorAuditActionMFAEnrolled)
}

// --- the code issue -------------------------------------------------------

// The code is stored consumed and only activated once the transport accepted
// the message: a refused send leaves nothing redeemable behind, and the
// attempt still counts toward the abuse cap.
func TestOperatorMFAStartChallengeFailsClosedOnDeliveryFailure(t *testing.T) {
	t.Parallel()

	operator := domain.Operator{ID: 7, Email: "ops@example.test", DisplayName: "Ops", Active: true}
	fixture := newOperatorMFAFixture(t, operator, nil)
	fixture.mail.err = errors.New("smtp connection lost")

	token, err := fixture.flows.StartChallenge(context.Background(), operator.ID, net.ParseIP("203.0.113.9"))
	require.ErrorIs(t, err, domain.ErrMFAStatusUnavailable)
	assert.Empty(t, token, "a refused delivery must not produce a challenge credential")
	assert.Zero(t, fixture.store.activateCalls, "the undelivered code must stay unredeemable")
	assert.Equal(t, 1, fixture.store.challengeCount, "the refused issue still counts toward the cap")
}

// The rate-limit count is the only statement that fails below; everything
// else behaves normally, which is the situation the fail-open bug needed to
// show itself: challenge creation and mail dispatch succeed regardless of
// whether that count could be read.
func TestOperatorMFAStartChallengeFailsClosedOnRateLimitLookupFailure(t *testing.T) {
	t.Parallel()

	operator := domain.Operator{ID: 19, Email: "ops@example.test", DisplayName: "Ops", Active: true}
	fixture := newOperatorMFAFixture(t, operator, &fakeOperatorMFAStore{countErr: errOperatorMFAStore})

	token, err := fixture.flows.StartChallenge(context.Background(), operator.ID, net.ParseIP("203.0.113.9"))
	require.ErrorIs(t, err, domain.ErrMFAStatusUnavailable,
		"an unreadable rate-limit count must refuse the code, not wave it through")
	assert.Empty(t, token, "a refused challenge must not produce a challenge credential")
	assert.Empty(t, fixture.mail.codes, "a refused challenge sends nothing")
	assert.Zero(t, fixture.store.challengeCount, "a refused challenge must leave no code behind")
	assert.Zero(t, fixture.store.activateCalls)
}

// The hard cap of three codes per window is an abuse defense, not a UX knob.
func TestOperatorMFAStartChallengeRateLimits(t *testing.T) {
	t.Parallel()

	operator := domain.Operator{ID: 8, Email: "ops@example.test", DisplayName: "Ops", Active: true}
	fixture := newOperatorMFAFixture(t, operator, &fakeOperatorMFAStore{challengeCount: domain.OperatorMFARateLimitMaxSent})

	_, err := fixture.flows.StartChallenge(context.Background(), operator.ID, net.ParseIP("203.0.113.9"))
	require.ErrorIs(t, err, domain.ErrMFARateLimited)
	assert.Empty(t, fixture.mail.codes, "a rate-limited request sends nothing")
}

// An operator inside its cooldown gets no further code.
func TestOperatorMFAStartChallengeRefusesLockedOperator(t *testing.T) {
	t.Parallel()

	lockedUntil := time.Now().Add(time.Minute)
	operator := domain.Operator{ID: 9, Email: "ops@example.test", DisplayName: "Ops", Active: true, MFALockedUntil: &lockedUntil}
	fixture := newOperatorMFAFixture(t, operator, nil)

	_, err := fixture.flows.StartChallenge(context.Background(), operator.ID, net.ParseIP("203.0.113.9"))
	require.ErrorIs(t, err, domain.ErrMFALocked)
}

// The mail carries the code, the operator's display name and the fixed
// remember-device lifetime.
func TestOperatorMFAStartChallengeMailsTheCode(t *testing.T) {
	t.Parallel()

	operator := domain.Operator{ID: 10, Email: "ops@example.test", DisplayName: "Ops", Active: true}
	fixture := newOperatorMFAFixture(t, operator, nil)

	token, err := fixture.flows.StartChallenge(context.Background(), operator.ID, net.ParseIP("203.0.113.9"))
	require.NoError(t, err)
	assert.Equal(t, "challenge-token", token)
	assert.Equal(t, domain.MFAChallengeScopePlatform, fixture.codec.issued.Scope,
		"the operator id reuses the account slot, so the scope is what distinguishes it")
	assert.Equal(t, operator.ID, fixture.codec.issued.AccountID)

	require.Len(t, fixture.mail.codes, 1)
	sent := fixture.mail.codes[0]
	assert.Equal(t, operator.Email, sent.Recipient)
	assert.Equal(t, operator.DisplayName, sent.RecipientName)
	assert.True(t, sent.Operator)
	assert.True(t, sent.TrustedDeviceEnabled, "operators always have remember-device on")
	assert.Equal(t, fixture.flows.TrustedDeviceDays(), sent.TrustedDeviceDays)
	assert.Equal(t, 1, fixture.store.activateCalls, "a delivered code becomes redeemable")
}

// --- verification ---------------------------------------------------------

// The loser of two concurrent verifications is refused: the consumption is
// one conditional statement, and without proof of single use the flow must
// not mint a second session from one code.
func TestOperatorMFAVerifyChallengeRefusesTheConsumeRaceLoser(t *testing.T) {
	t.Parallel()

	operator := domain.Operator{ID: 11, Email: "ops@example.test", DisplayName: "Ops", Active: true}
	store := &fakeOperatorMFAStore{
		challenge:  &domain.OperatorMFAChallenge{ID: 7, OperatorID: 11, CodeHash: "hash:123456"},
		consumeErr: errors.New("operator mfa challenge was already consumed or activated"),
	}
	fixture := newOperatorMFAFixture(t, operator, store)
	fixture.codec.parse["token"] = domain.MFAChallengeClaims{AccountID: operator.ID, Scope: domain.MFAChallengeScopePlatform}

	verified, err := fixture.flows.VerifyChallenge(context.Background(), "token", "123456")
	require.ErrorIs(t, err, domain.ErrMFACodeInvalid,
		"a consumption that did not apply must read as the generic invalid code, leaking nothing")
	assert.Zero(t, verified.OperatorID)
	assert.Equal(t, 1, store.consumeCalls, "the flow consumes exactly once before refusing")
	assert.Contains(t, fixture.audit.operatorActions(), domain.OperatorAuditActionMFAFailed)
}

// The JWT-less path the enrollment confirm uses applies the same rule.
func TestOperatorMFAVerifyCodeRefusesTheConsumeRaceLoser(t *testing.T) {
	t.Parallel()

	operator := domain.Operator{ID: 12, Email: "ops@example.test", DisplayName: "Ops", Active: true}
	store := &fakeOperatorMFAStore{
		challenge:  &domain.OperatorMFAChallenge{ID: 7, OperatorID: 12, CodeHash: "hash:654321"},
		consumeErr: errors.New("operator mfa challenge was already consumed or activated"),
	}
	fixture := newOperatorMFAFixture(t, operator, store)

	err := fixture.flows.VerifyCodeForOperator(context.Background(), operator.ID, "654321")
	require.ErrorIs(t, err, domain.ErrMFACodeInvalid)
	assert.Equal(t, 1, store.consumeCalls)
}

// A challenge token of another portal is refused before any code is read.
func TestOperatorMFAVerifyChallengeRefusesForeignScope(t *testing.T) {
	t.Parallel()

	operator := domain.Operator{ID: 13, Email: "ops@example.test", DisplayName: "Ops", Active: true}
	fixture := newOperatorMFAFixture(t, operator, nil)
	fixture.codec.parse["tenant-token"] = domain.MFAChallengeClaims{
		AccountID: operator.ID, Scope: domain.MFAChallengeScopeTenant,
	}

	_, err := fixture.flows.VerifyChallenge(context.Background(), "tenant-token", "123456")
	require.ErrorIs(t, err, domain.ErrMFAChallengeTokenInvalid)
	assert.Zero(t, fixture.store.consumeCalls, "a foreign token never reaches the code")
}

// A wrong code counts toward the cooldown and is refused generically.
func TestOperatorMFAVerifyChallengeWrongCodeCountsTheAttempt(t *testing.T) {
	t.Parallel()

	operator := domain.Operator{ID: 14, Email: "ops@example.test", DisplayName: "Ops", Active: true}
	store := &fakeOperatorMFAStore{challenge: &domain.OperatorMFAChallenge{ID: 7, OperatorID: 14, CodeHash: "hash:123456"}}
	fixture := newOperatorMFAFixture(t, operator, store)
	fixture.codec.parse["token"] = domain.MFAChallengeClaims{AccountID: operator.ID, Scope: domain.MFAChallengeScopePlatform}

	_, err := fixture.flows.VerifyChallenge(context.Background(), "token", "000000")
	require.ErrorIs(t, err, domain.ErrMFACodeInvalid)
	assert.Zero(t, store.consumeCalls, "a wrong code is never consumed")
	assert.Contains(t, fixture.audit.operatorActions(), domain.OperatorAuditActionMFAFailed)
}

// --- the disable cascade --------------------------------------------------

// The cascade is one transaction: a failed revoke must roll the credential
// delete back, or the operator would be left with no credential while its
// remember-device cookies still verify.
func TestOperatorMFADisableRollsBackOnPartialFailure(t *testing.T) {
	t.Parallel()

	operator := domain.Operator{ID: 15, Email: "ops@example.test", DisplayName: "Ops", Active: true}
	revokeErr := errors.New("simulated hiccup during trusted-device revoke")
	fixture := newOperatorMFAFixture(t, operator, &fakeOperatorMFAStore{revokeAllErr: revokeErr})

	ctx := context.Background()
	require.NoError(t, fixture.flows.Enroll(ctx, operator.ID))
	cookie, _, err := fixture.flows.IssueTrustedDevice(ctx, operator.ID, "Mozilla/5.0 (Test)", net.ParseIP("203.0.113.20"))
	require.NoError(t, err)
	require.NotEmpty(t, cookie)

	require.ErrorIs(t, fixture.flows.Disable(ctx, operator.ID), revokeErr,
		"the disable must surface the failing step")
	assert.Equal(t, 1, fixture.store.deleteCredCalls, "the delete ran before the failing revoke")
	assert.Equal(t, 1, fixture.runtime.adminTxCount, "the cascade runs in exactly one transaction")
	assert.NotContains(t, fixture.audit.operatorActions(), domain.OperatorAuditActionMFADisabled,
		"a rolled-back cascade records no disable")

	// Both must survive: the credential delete that already applied is
	// undone with the failing revoke, so the operator is never left without
	// an enrollment while its cookies still verify.
	enrolled, err := fixture.flows.HasEnrollment(ctx, operator.ID)
	require.NoError(t, err)
	assert.True(t, enrolled, "the credential delete must roll back with the failing revoke")
	verified, err := fixture.flows.VerifyTrustedDevice(ctx, operator.ID, cookie)
	require.NoError(t, err)
	assert.True(t, verified, "the trusted device must still verify after the failed cascade")
}

// The successful cascade clears the enrollment and every trusted device.
func TestOperatorMFADisableClearsEverything(t *testing.T) {
	t.Parallel()

	operator := domain.Operator{ID: 16, Email: "ops@example.test", DisplayName: "Ops", Active: true}
	fixture := newOperatorMFAFixture(t, operator, nil)

	ctx := context.Background()
	require.NoError(t, fixture.flows.Enroll(ctx, operator.ID))
	cookie, _, err := fixture.flows.IssueTrustedDevice(ctx, operator.ID, "Mozilla/5.0 (Test)", net.ParseIP("203.0.113.21"))
	require.NoError(t, err)

	require.NoError(t, fixture.flows.Disable(ctx, operator.ID))

	enrolled, err := fixture.flows.HasEnrollment(ctx, operator.ID)
	require.NoError(t, err)
	assert.False(t, enrolled)
	verified, err := fixture.flows.VerifyTrustedDevice(ctx, operator.ID, cookie)
	require.NoError(t, err)
	assert.False(t, verified, "a revoked device must stop verifying")
	assert.Contains(t, fixture.audit.operatorActions(), domain.OperatorAuditActionMFADisabled)
}

// --- trusted devices ------------------------------------------------------

// A device can only be revoked by its own operator, so an id guess cannot
// revoke someone else's.
func TestOperatorMFARevokeTrustedDeviceChecksOwnership(t *testing.T) {
	t.Parallel()

	operator := domain.Operator{ID: 17, Email: "ops@example.test", DisplayName: "Ops", Active: true}
	fixture := newOperatorMFAFixture(t, operator, nil)

	ctx := context.Background()
	_, _, err := fixture.flows.IssueTrustedDevice(ctx, operator.ID, "Mozilla/5.0 (Test)", net.ParseIP("203.0.113.22"))
	require.NoError(t, err)
	devices, err := fixture.flows.ListTrustedDevices(ctx, operator.ID)
	require.NoError(t, err)
	require.Len(t, devices, 1)

	require.ErrorIs(t, fixture.flows.RevokeTrustedDevice(ctx, operator.ID, devices[0].ID+99),
		domain.ErrMFAPermissionDenied, "a device that is not this operator's is refused")
	require.NoError(t, fixture.flows.RevokeTrustedDevice(ctx, operator.ID, devices[0].ID))
}

// Issuing a cookie tells the operator by mail, so trusting a device never
// happens silently.
func TestOperatorMFAIssueTrustedDeviceNotifiesTheOperator(t *testing.T) {
	t.Parallel()

	operator := domain.Operator{ID: 18, Email: "ops@example.test", DisplayName: "Ops", Active: true}
	fixture := newOperatorMFAFixture(t, operator, nil)

	cookie, expiresAt, err := fixture.flows.IssueTrustedDevice(
		context.Background(), operator.ID, "Mozilla/5.0 (Macintosh) Chrome/120.0", net.ParseIP("203.0.113.23"))
	require.NoError(t, err)
	assert.NotEmpty(t, cookie)
	assert.True(t, expiresAt.After(time.Now()))

	require.Len(t, fixture.mail.devices, 1)
	sent := fixture.mail.devices[0]
	assert.Equal(t, operator.Email, sent.Recipient)
	assert.True(t, sent.Operator)
	assert.Equal(t, fixture.flows.TrustedDeviceDays(), sent.TrustedDays)
	assert.NotEmpty(t, sent.DeviceLabel)
	assert.Contains(t, fixture.audit.operatorActions(), domain.OperatorAuditActionMFATrustedDeviceAdded)
}
