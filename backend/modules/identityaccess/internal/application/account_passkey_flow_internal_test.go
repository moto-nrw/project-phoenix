package application

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/ports"
)

// The school-portal ceremonies moved here with the passkey flows (#3331).
// What the tests pin is unchanged: a ceremony only starts on the school's own
// origin, a registration is gated by the emailed code, and a completion
// consumes the ceremony in one administrative transaction — committing when
// the verification refused it, rolling back when a read or write failed.

var errPasskeyStore = errors.New("passkey store down")

// --- doubles --------------------------------------------------------------

// fakeAccountDirectory answers the account facts the ceremonies read.
type fakeAccountDirectory struct {
	accounts map[int64]domain.AccountIdentity
	err      error
}

func (d *fakeAccountDirectory) FindAccountIdentity(_ context.Context, accountID int64) (domain.AccountIdentity, bool, error) {
	if d.err != nil {
		return domain.AccountIdentity{}, false, d.err
	}
	account, found := d.accounts[accountID]
	return account, found, nil
}

func directoryOf(accounts ...domain.AccountIdentity) *fakeAccountDirectory {
	byID := make(map[int64]domain.AccountIdentity, len(accounts))
	for _, account := range accounts {
		byID[account.ID] = account
	}
	return &fakeAccountDirectory{accounts: byID}
}

// fakePasskeyGate stands in for the second factor a registration is gated by
// and records what it was asked to verify.
type fakePasskeyGate struct {
	challengeToken    string
	startErr          error
	verifyErr         error
	startedAccountID  int64
	startedTenantID   int64
	startedScope      string
	verifiedAccountID int64
	verifiedCode      string
}

func (g *fakePasskeyGate) StartChallenge(_ context.Context, accountID, tenantID int64, scope string, _ net.IP) (string, error) {
	g.startedAccountID, g.startedTenantID, g.startedScope = accountID, tenantID, scope
	return g.challengeToken, g.startErr
}

func (g *fakePasskeyGate) VerifyCodeForAccount(_ context.Context, accountID, _ int64, code, _ string) error {
	g.verifiedAccountID, g.verifiedCode = accountID, code
	return g.verifyErr
}

// fakePasskeySessions mints the pair a completed login returns and answers
// the school-membership question the login turns on.
type fakePasskeySessions struct {
	access     string
	refresh    string
	issueErr   error
	hasAccess  bool
	accessErr  error
	askedForID int64
}

func (s *fakePasskeySessions) IssueTokensForAuthenticatedAccount(_ context.Context, _, _ int64, _, _ string) (string, string, error) {
	return s.access, s.refresh, s.issueErr
}

func (s *fakePasskeySessions) VerifyAccountTenantMembership(_ context.Context, accountID, _ int64) (bool, error) {
	s.askedForID = accountID
	return s.hasAccess, s.accessErr
}

// fakeAccountPasskeyStore serves the credential and ceremony halves of the
// passkey store. A missing row is found=false, as the port requires.
type fakeAccountPasskeyStore struct {
	rows        []domain.AccountPasskeyCredential
	err         error
	alreadyGone bool

	createdSession  *domain.AccountPasskeySession
	consumedSession *domain.AccountPasskeySession
	createErr       error
	consumeErr      error

	revokedAccountID    int64
	revokedCredentialID int64
}

func (s *fakeAccountPasskeyStore) InsertAccountPasskey(_ context.Context, credential domain.AccountPasskeyCredential) (domain.AccountPasskeyCredential, domain.OperationStats, error) {
	return credential, domain.OperationStats{}, s.err
}

func (s *fakeAccountPasskeyStore) ListActiveAccountPasskeys(context.Context, int64) ([]domain.AccountPasskeyCredential, domain.OperationStats, error) {
	return s.rows, domain.OperationStats{}, s.err
}

func (s *fakeAccountPasskeyStore) FindActiveAccountPasskey(context.Context, []byte, []byte) (domain.AccountPasskeyCredential, bool, domain.OperationStats, error) {
	if s.err != nil {
		return domain.AccountPasskeyCredential{}, false, domain.OperationStats{}, s.err
	}
	if len(s.rows) == 0 {
		return domain.AccountPasskeyCredential{}, false, domain.OperationStats{}, nil
	}
	return s.rows[0], true, domain.OperationStats{}, nil
}

func (s *fakeAccountPasskeyStore) UpdateAccountPasskeyAfterUse(context.Context, int64, []byte, time.Time) (bool, domain.OperationStats, error) {
	return true, domain.OperationStats{}, s.err
}

func (s *fakeAccountPasskeyStore) RevokeAccountPasskey(_ context.Context, accountID, id int64, _ time.Time) (bool, domain.OperationStats, error) {
	s.revokedAccountID, s.revokedCredentialID = accountID, id
	if s.err != nil {
		return false, domain.OperationStats{}, s.err
	}
	return !s.alreadyGone, domain.OperationStats{}, nil
}

func (s *fakeAccountPasskeyStore) InsertAccountPasskeySession(_ context.Context, session domain.AccountPasskeySession) (domain.AccountPasskeySession, domain.OperationStats, error) {
	stored := session
	s.createdSession = &stored
	return session, domain.OperationStats{}, s.createErr
}

func (s *fakeAccountPasskeyStore) ConsumeAccountPasskeySession(context.Context, string, string, time.Time) (domain.AccountPasskeySession, bool, domain.OperationStats, error) {
	if s.consumeErr != nil {
		return domain.AccountPasskeySession{}, false, domain.OperationStats{}, s.consumeErr
	}
	if s.consumedSession == nil {
		return domain.AccountPasskeySession{}, false, domain.OperationStats{}, nil
	}
	return *s.consumedSession, true, domain.OperationStats{}, nil
}

var _ ports.AccountPasskeyStore = (*fakeAccountPasskeyStore)(nil)

// passkeyTransactions is a runtime without a database that counts how the
// ceremony completions ended.
type passkeyTransactions struct {
	fakeRuntime
	commits   int
	rollbacks int
}

func (l *passkeyTransactions) WithAdminTx(ctx context.Context, fn func(context.Context) error) error {
	err := l.fakeRuntime.WithAdminTx(ctx, fn)
	if err != nil {
		l.rollbacks++
	} else {
		l.commits++
	}
	return err
}

// assertOutcome checks that the completion ran in exactly one transaction
// that committed (a rejected ceremony stays spent) or rolled back (a failed
// read or write leaves the ceremony open).
func (l *passkeyTransactions) assertOutcome(t *testing.T, committed bool) {
	t.Helper()
	if committed {
		assert.Equal(t, 1, l.commits, "a rejected ceremony stays spent")
		assert.Zero(t, l.rollbacks)
		return
	}
	assert.Equal(t, 1, l.rollbacks, "a failed read or write rolls the consumption back")
	assert.Zero(t, l.commits)
}

// newAccountPasskeyFlows composes the ceremonies over the doubles.
func newAccountPasskeyFlows(
	t *testing.T,
	store *fakeAccountPasskeyStore,
	directory *fakeAccountDirectory,
	gate *fakePasskeyGate,
	sessions *fakePasskeySessions,
	transactions *passkeyTransactions,
) *AccountPasskeyFlows {
	t.Helper()
	if store == nil {
		store = &fakeAccountPasskeyStore{}
	}
	if directory == nil {
		directory = directoryOf()
	}
	if gate == nil {
		gate = &fakePasskeyGate{}
	}
	if sessions == nil {
		sessions = &fakePasskeySessions{}
	}
	var runtime ports.Runtime = transactions
	if transactions == nil {
		runtime = &fakeRuntime{}
	}
	service := New(newFakeStore(), newFakeStore(), newFakeStore(), fakeTransaction{}, runtime.TenantID, func(ports.Observation) {})
	flows, err := NewAccountPasskeyFlows(AccountPasskeyFlowDependencies{
		Accounts: directory, Records: NewAccountPasskey(service, store), MFA: gate, Sessions: sessions,
		Runtime: runtime, RPID: "localhost", RPName: "moto", TenantDomain: "localhost",
	})
	require.NoError(t, err)
	return flows
}

// --- the enrollment challenge --------------------------------------------

func TestAccountPasskeyEnrollmentChallenge(t *testing.T) {
	t.Parallel()

	account := domain.AccountIdentity{ID: 11, Email: "teacher@example.test", Active: true}
	gate := &fakePasskeyGate{challengeToken: "challenge-token"}
	flows := newAccountPasskeyFlows(t, nil, directoryOf(account), gate, nil, nil)

	challenge, err := flows.StartEnrollmentChallenge(context.Background(), account.ID, 42, net.ParseIP("203.0.113.8"))
	require.NoError(t, err)
	assert.Equal(t, "challenge-token", challenge.ChallengeToken)
	assert.Contains(t, challenge.MaskedEmail, "@")
	assert.NotContains(t, challenge.MaskedEmail, "teacher@", "the mailbox stays masked")
	assert.Equal(t, account.ID, gate.startedAccountID)
	assert.Equal(t, int64(42), gate.startedTenantID)
	assert.Equal(t, domain.MFAChallengeScopeTenant, gate.startedScope)
}

func TestAccountPasskeyEnrollmentChallengeErrors(t *testing.T) {
	t.Parallel()

	account := domain.AccountIdentity{ID: 12, Email: "teacher@example.test", Active: true}
	wantErr := errors.New("mfa down")

	tests := map[string]struct {
		directory *fakeAccountDirectory
		gate      *fakePasskeyGate
	}{
		"unknown account": {directoryOf(), &fakePasskeyGate{}},
		"mfa start fails": {directoryOf(account), &fakePasskeyGate{startErr: wantErr}},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			flows := newAccountPasskeyFlows(t, nil, tt.directory, tt.gate, nil, nil)
			_, err := flows.StartEnrollmentChallenge(context.Background(), account.ID, 42, net.ParseIP("203.0.113.8"))
			require.Error(t, err)
		})
	}
}

// --- beginning a ceremony -------------------------------------------------

func TestAccountPasskeyBeginRegistrationStoresSession(t *testing.T) {
	t.Parallel()

	account := domain.AccountIdentity{ID: 21, Email: "teacher@example.test", Active: true}
	store := &fakeAccountPasskeyStore{}
	gate := &fakePasskeyGate{}
	flows := newAccountPasskeyFlows(t, store, directoryOf(account), gate, nil, nil)

	creation, err := flows.BeginRegistration(context.Background(), domain.AccountPasskeyRegistrationStart{
		AccountID:       account.ID,
		TenantID:        44,
		TenantSubdomain: "school",
		ExpectedOrigin:  "http://school.localhost:3000",
		Code:            "123456",
		Name:            "Laptop",
	})
	require.NoError(t, err)
	require.NotEmpty(t, creation.SessionID)
	require.NotNil(t, creation.Options)
	require.NotNil(t, store.createdSession)
	require.NotNil(t, store.createdSession.AccountID)
	require.NotNil(t, store.createdSession.TenantID)
	assert.Equal(t, account.ID, *store.createdSession.AccountID)
	assert.Equal(t, int64(44), *store.createdSession.TenantID)
	assert.Equal(t, domain.PasskeySessionPurposeRegistration, store.createdSession.Purpose)
	assert.Equal(t, "school.localhost", store.createdSession.RPID)
	assert.Equal(t, "http://school.localhost:3000", store.createdSession.ExpectedOrigin)
	assert.False(t, store.createdSession.ExpiresAt.IsZero())
	assert.True(t, json.Valid(store.createdSession.SessionJSON))
	assert.Equal(t, account.ID, gate.verifiedAccountID)
	assert.Equal(t, "123456", gate.verifiedCode)
}

func TestAccountPasskeyBeginRegistrationErrors(t *testing.T) {
	t.Parallel()

	account := domain.AccountIdentity{ID: 22, Email: "teacher@example.test", Active: true}
	inactive := domain.AccountIdentity{ID: 23, Email: "inactive@example.test"}
	wantErr := errors.New("boom")

	tests := []struct {
		name      string
		request   domain.AccountPasskeyRegistrationStart
		directory *fakeAccountDirectory
		store     *fakeAccountPasskeyStore
		gate      *fakePasskeyGate
		wantErr   error
	}{
		{
			name: "invalid origin short circuits before mfa",
			request: domain.AccountPasskeyRegistrationStart{
				AccountID: account.ID, ExpectedOrigin: "http://other.localhost:3000",
			},
			directory: directoryOf(account),
			wantErr:   domain.ErrPasskeyOriginInvalid,
		},
		{
			name: "mfa verify fails",
			request: domain.AccountPasskeyRegistrationStart{
				AccountID: account.ID, ExpectedOrigin: "http://localhost:3000", Code: "000000",
			},
			directory: directoryOf(account),
			gate:      &fakePasskeyGate{verifyErr: wantErr},
		},
		{
			name: "unknown account",
			request: domain.AccountPasskeyRegistrationStart{
				AccountID: 999, ExpectedOrigin: "http://localhost:3000",
			},
			directory: directoryOf(),
			wantErr:   domain.ErrAccountNotFound,
		},
		{
			name: "inactive account",
			request: domain.AccountPasskeyRegistrationStart{
				AccountID: inactive.ID, ExpectedOrigin: "http://localhost:3000",
			},
			directory: directoryOf(inactive),
			wantErr:   domain.ErrAccountInactive,
		},
		{
			name: "credential lookup fails while building the user",
			request: domain.AccountPasskeyRegistrationStart{
				AccountID: account.ID, ExpectedOrigin: "http://localhost:3000",
			},
			directory: directoryOf(account),
			store:     &fakeAccountPasskeyStore{err: wantErr},
		},
		{
			name: "session create fails",
			request: domain.AccountPasskeyRegistrationStart{
				AccountID: account.ID, ExpectedOrigin: "http://localhost:3000",
			},
			directory: directoryOf(account),
			store:     &fakeAccountPasskeyStore{createErr: wantErr},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			flows := newAccountPasskeyFlows(t, tt.store, tt.directory, tt.gate, nil, nil)
			_, err := flows.BeginRegistration(context.Background(), tt.request)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				return
			}
			require.Error(t, err)
		})
	}
}

func TestAccountPasskeyBeginLoginStoresSession(t *testing.T) {
	t.Parallel()

	store := &fakeAccountPasskeyStore{}
	flows := newAccountPasskeyFlows(t, store, nil, nil, nil, nil)

	assertion, err := flows.BeginLogin(context.Background(), domain.AccountPasskeyLoginStart{
		TenantID: 45, TenantSubdomain: "school", ExpectedOrigin: "http://school.localhost:3000",
	})
	require.NoError(t, err)
	require.NotEmpty(t, assertion.SessionID)
	require.NotNil(t, assertion.Options)
	require.NotNil(t, store.createdSession)
	require.NotNil(t, store.createdSession.TenantID)
	assert.Equal(t, int64(45), *store.createdSession.TenantID)
	assert.Equal(t, domain.PasskeySessionPurposeLogin, store.createdSession.Purpose)
	assert.Equal(t, "school.localhost", store.createdSession.RPID)
	assert.Equal(t, "http://school.localhost:3000", store.createdSession.ExpectedOrigin)
	assert.False(t, store.createdSession.ExpiresAt.IsZero())
}

func TestAccountPasskeyBeginLoginErrors(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("session down")

	tests := []struct {
		name    string
		request domain.AccountPasskeyLoginStart
		store   *fakeAccountPasskeyStore
		wantErr error
	}{
		{
			name: "invalid origin",
			request: domain.AccountPasskeyLoginStart{
				TenantSubdomain: "school", ExpectedOrigin: "http://other.localhost:3000",
			},
			wantErr: domain.ErrPasskeyOriginInvalid,
		},
		{
			name: "session create fails",
			request: domain.AccountPasskeyLoginStart{
				TenantID: 45, ExpectedOrigin: "http://localhost:3000",
			},
			store: &fakeAccountPasskeyStore{createErr: wantErr},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			flows := newAccountPasskeyFlows(t, tt.store, nil, nil, nil, nil)
			_, err := flows.BeginLogin(context.Background(), tt.request)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				return
			}
			require.Error(t, err)
		})
	}
}

// --- completing a ceremony ------------------------------------------------

func TestAccountPasskeyFinishRegistrationRejectsInvalidSessionState(t *testing.T) {
	t.Parallel()

	account := domain.AccountIdentity{ID: 31, Email: "teacher@example.test", Active: true}
	accountID := account.ID
	otherAccountID := account.ID + 1

	tests := []struct {
		name      string
		session   *domain.AccountPasskeySession
		directory *fakeAccountDirectory
		store     *fakeAccountPasskeyStore
		wantErr   error
		committed bool
	}{
		{
			name:      "no pending ceremony",
			wantErr:   domain.ErrPasskeySessionInvalid,
			committed: true,
		},
		{
			name:    "consume store failure is not an invalid session",
			store:   &fakeAccountPasskeyStore{consumeErr: errPasskeyStore},
			wantErr: errPasskeyStore,
		},
		{
			name: "missing account id",
			session: &domain.AccountPasskeySession{
				SessionJSON: json.RawMessage(`{}`), ExpectedOrigin: "http://localhost:3000",
			},
			wantErr:   domain.ErrPasskeySessionInvalid,
			committed: true,
		},
		{
			name: "wrong account id",
			session: &domain.AccountPasskeySession{
				AccountID: &otherAccountID, SessionJSON: json.RawMessage(`{}`),
				ExpectedOrigin: "http://localhost:3000",
			},
			wantErr:   domain.ErrPasskeySessionInvalid,
			committed: true,
		},
		{
			name: "invalid session json",
			session: &domain.AccountPasskeySession{
				AccountID: &accountID, SessionJSON: json.RawMessage(`{`),
				ExpectedOrigin: "http://localhost:3000",
			},
			committed: true,
		},
		{
			name: "unknown account",
			session: &domain.AccountPasskeySession{
				AccountID: &accountID, SessionJSON: json.RawMessage(`{}`),
				ExpectedOrigin: "http://localhost:3000",
			},
			directory: directoryOf(),
			wantErr:   domain.ErrAccountNotFound,
			committed: true,
		},
		{
			name: "account read failure is not a missing account",
			session: &domain.AccountPasskeySession{
				AccountID: &accountID, SessionJSON: json.RawMessage(`{}`),
				ExpectedOrigin: "http://localhost:3000",
			},
			directory: &fakeAccountDirectory{err: errPasskeyStore},
			wantErr:   errPasskeyStore,
		},
		{
			name: "credential lookup failure",
			session: &domain.AccountPasskeySession{
				AccountID: &accountID, SessionJSON: json.RawMessage(`{}`),
				ExpectedOrigin: "http://localhost:3000",
			},
			directory: directoryOf(account),
			store:     &fakeAccountPasskeyStore{err: errPasskeyStore},
			wantErr:   errPasskeyStore,
		},
		{
			name: "invalid response json",
			session: &domain.AccountPasskeySession{
				AccountID: &accountID, SessionJSON: json.RawMessage(`{}`),
				ExpectedOrigin: "http://localhost:3000",
			},
			directory: directoryOf(account),
			wantErr:   domain.ErrPasskeySessionInvalid,
			committed: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			store := tt.store
			if store == nil {
				store = &fakeAccountPasskeyStore{}
			}
			store.consumedSession = tt.session
			transactions := &passkeyTransactions{}
			flows := newAccountPasskeyFlows(t, store, tt.directory, nil, nil, transactions)

			_, err := flows.FinishRegistration(context.Background(), domain.AccountPasskeyRegistrationFinish{
				AccountID:          account.ID,
				SessionID:          "session-id",
				CredentialResponse: json.RawMessage(`{`),
			})
			transactions.assertOutcome(t, tt.committed)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				return
			}
			require.Error(t, err)
		})
	}
}

func TestAccountPasskeyFinishLoginRejectsInvalidSessionState(t *testing.T) {
	t.Parallel()

	tenantID := int64(70010001)

	tests := []struct {
		name       string
		session    *domain.AccountPasskeySession
		consumeErr error
		wantErr    error
		committed  bool
	}{
		{
			name:      "no pending ceremony",
			wantErr:   domain.ErrPasskeySessionInvalid,
			committed: true,
		},
		{
			name:       "consume store failure is not an invalid session",
			consumeErr: errPasskeyStore,
			wantErr:    errPasskeyStore,
		},
		{
			name: "missing tenant id",
			session: &domain.AccountPasskeySession{
				SessionJSON: json.RawMessage(`{}`), ExpectedOrigin: "http://localhost:3000",
			},
			wantErr:   domain.ErrPasskeySessionInvalid,
			committed: true,
		},
		{
			name: "invalid session json",
			session: &domain.AccountPasskeySession{
				TenantID: &tenantID, SessionJSON: json.RawMessage(`{`),
				ExpectedOrigin: "http://localhost:3000",
			},
			committed: true,
		},
		{
			name: "invalid response json",
			session: &domain.AccountPasskeySession{
				TenantID: &tenantID, SessionJSON: json.RawMessage(`{}`),
				ExpectedOrigin: "http://localhost:3000",
			},
			wantErr:   domain.ErrPasskeySessionInvalid,
			committed: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			store := &fakeAccountPasskeyStore{consumedSession: tt.session, consumeErr: tt.consumeErr}
			transactions := &passkeyTransactions{}
			flows := newAccountPasskeyFlows(t, store, nil, nil, nil, transactions)

			_, err := flows.FinishLogin(context.Background(), domain.AccountPasskeyLoginFinish{
				SessionID:          "session-id",
				CredentialResponse: json.RawMessage(`{`),
			})
			transactions.assertOutcome(t, tt.committed)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				return
			}
			require.Error(t, err)
		})
	}
}

// newPasskeyAssertionForTests returns a parseable discoverable-login response
// for credentialID and userHandle. The WebAuthn library resolves the
// credential from it before checking the signature, so it drives FinishLogin
// to the credential lookup and fails verification afterwards.
func newPasskeyAssertionForTests(t *testing.T, credentialID, userHandle []byte) json.RawMessage {
	t.Helper()
	encode := base64.RawURLEncoding.EncodeToString
	clientData := `{"type":"webauthn.get","challenge":"Y2hhbGxlbmdl","origin":"http://school.localhost:3000"}`
	raw, err := json.Marshal(map[string]any{
		"id": encode(credentialID), "rawId": encode(credentialID), "type": "public-key",
		"response": map[string]string{
			"clientDataJSON":    encode([]byte(clientData)),
			"authenticatorData": encode(make([]byte, 37)),
			"signature":         encode([]byte("signature")),
			"userHandle":        encode(userHandle),
		},
	})
	require.NoError(t, err)
	return raw
}

// Every read failure behind the credential resolution must surface as the
// store error and roll the consumption back; an unknown credential, an
// account without access to the school and a failed signature read as a
// client error and spend the ceremony.
func TestAccountPasskeyFinishLoginCredentialLookup(t *testing.T) {
	t.Parallel()

	tenantID := int64(70010002)
	account := domain.AccountIdentity{ID: 41, Email: "teacher@example.test", Active: true}
	registered := domain.AccountPasskeyCredential{
		ID: 42, AccountID: account.ID, UserHandle: []byte("user-handle"), CredentialJSON: json.RawMessage(`{}`),
	}

	tests := []struct {
		name      string
		store     *fakeAccountPasskeyStore
		directory *fakeAccountDirectory
		sessions  *fakePasskeySessions
		wantErr   error
		committed bool
	}{
		{
			name:    "credential lookup failure",
			store:   &fakeAccountPasskeyStore{err: errPasskeyStore},
			wantErr: errPasskeyStore,
		},
		{
			name:      "unknown credential",
			store:     &fakeAccountPasskeyStore{},
			wantErr:   domain.ErrInvalidCredentials,
			committed: true,
		},
		{
			name:      "account read failure",
			store:     &fakeAccountPasskeyStore{rows: []domain.AccountPasskeyCredential{registered}},
			directory: &fakeAccountDirectory{err: errPasskeyStore},
			wantErr:   errPasskeyStore,
		},
		{
			name:      "account gone",
			store:     &fakeAccountPasskeyStore{rows: []domain.AccountPasskeyCredential{registered}},
			directory: directoryOf(),
			wantErr:   domain.ErrInvalidCredentials,
			committed: true,
		},
		{
			name:      "signature does not verify",
			store:     &fakeAccountPasskeyStore{rows: []domain.AccountPasskeyCredential{registered}},
			directory: directoryOf(account),
			wantErr:   domain.ErrInvalidCredentials,
			committed: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tt.store.consumedSession = &domain.AccountPasskeySession{
				TenantID:       &tenantID,
				SessionJSON:    json.RawMessage(`{"challenge":"Y2hhbGxlbmdl"}`),
				ExpectedOrigin: "http://school.localhost:3000",
			}
			transactions := &passkeyTransactions{}
			flows := newAccountPasskeyFlows(t, tt.store, tt.directory, nil, tt.sessions, transactions)

			_, err := flows.FinishLogin(context.Background(), domain.AccountPasskeyLoginFinish{
				SessionID:          "session-id",
				CredentialResponse: newPasskeyAssertionForTests(t, []byte("credential-id"), registered.UserHandle),
			})
			require.ErrorIs(t, err, tt.wantErr)
			transactions.assertOutcome(t, tt.committed)
		})
	}
}

// --- listing and revoking -------------------------------------------------

func TestAccountPasskeyCredentialMethods(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()
	account := domain.AccountIdentity{ID: 91, Email: "teacher@example.test", Active: true}
	row := domain.AccountPasskeyCredential{
		ID: 92, CreatedAt: now, AccountID: account.ID,
		UserHandle: []byte("user-handle"), CredentialJSON: json.RawMessage(`{}`), Name: "Laptop",
	}
	store := &fakeAccountPasskeyStore{rows: []domain.AccountPasskeyCredential{row}}
	flows := newAccountPasskeyFlows(t, store, directoryOf(account), nil, nil, nil)

	list, err := flows.ListCredentials(context.Background(), account.ID)
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, "92", list[0].ID)

	require.NoError(t, flows.RevokeCredential(context.Background(), account.ID, row.ID))
	assert.Equal(t, account.ID, store.revokedAccountID)
	assert.Equal(t, row.ID, store.revokedCredentialID)

	user, err := flows.passkeyUser(context.Background(), account)
	require.NoError(t, err)
	assert.Equal(t, []byte("user-handle"), user.WebAuthnID())
	assert.Equal(t, account.Email, user.WebAuthnName())
	assert.Len(t, user.WebAuthnCredentials(), 1)

	// Without a stored credential the handle comes from the ceremony, and a
	// ceremony without one gets a fresh handle.
	store.rows = nil
	user, err = flows.passkeyUser(context.Background(), account)
	require.NoError(t, err)
	assert.Len(t, user.WebAuthnID(), domain.PasskeyUserHandleBytes)

	user, err = flows.passkeyUser(context.Background(), account, []byte("session-handle"))
	require.NoError(t, err)
	assert.Equal(t, []byte("session-handle"), user.WebAuthnID())

	store.rows = []domain.AccountPasskeyCredential{{CredentialJSON: json.RawMessage(`{`)}}
	_, err = flows.passkeyUser(context.Background(), account)
	require.Error(t, err)
}

func TestAccountPasskeyCredentialErrors(t *testing.T) {
	t.Parallel()

	account := domain.AccountIdentity{ID: 1, Email: "teacher@example.test", Active: true}
	failing := newAccountPasskeyFlows(t, &fakeAccountPasskeyStore{err: errPasskeyStore}, directoryOf(account), nil, nil, nil)

	_, err := failing.ListCredentials(context.Background(), account.ID)
	require.ErrorIs(t, err, errPasskeyStore)

	err = failing.RevokeCredential(context.Background(), account.ID, 2)
	require.ErrorIs(t, err, errPasskeyStore, "a store failure is not a missing passkey")
	require.NotErrorIs(t, err, domain.ErrPasskeyNotFound)

	_, err = failing.passkeyUser(context.Background(), account)
	require.ErrorIs(t, err, errPasskeyStore)

	gone := newAccountPasskeyFlows(t, &fakeAccountPasskeyStore{alreadyGone: true}, directoryOf(account), nil, nil, nil)
	err = gone.RevokeCredential(context.Background(), account.ID, 2)
	require.ErrorIs(t, err, domain.ErrPasskeyNotFound, "a passkey that is gone, revoked or foreign is not found")
}
