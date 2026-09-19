package application

import (
	"context"
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

// The operator ceremonies moved here with the passkey flows (#3331). They
// are the school-portal ones minus the school: a ceremony only starts on the
// operator portal's own host, the registration is gated by the emailed code,
// and the completion consumes the ceremony in one administrative transaction
// — committing when the verification refused it, rolling back when a read or
// write failed.

// --- doubles --------------------------------------------------------------

// fakeOperatorPasskeyGate stands in for the operator second factor.
type fakeOperatorPasskeyGate struct {
	challengeToken     string
	startErr           error
	verifyErr          error
	startedOperatorID  int64
	verifiedOperatorID int64
	verifiedCode       string
}

func (g *fakeOperatorPasskeyGate) StartChallenge(_ context.Context, operatorID int64, _ net.IP) (string, error) {
	g.startedOperatorID = operatorID
	return g.challengeToken, g.startErr
}

func (g *fakeOperatorPasskeyGate) VerifyCodeForOperator(_ context.Context, operatorID int64, code string) error {
	g.verifiedOperatorID, g.verifiedCode = operatorID, code
	return g.verifyErr
}

// fakeOperatorPasskeySessions mints the pair a completed login returns.
type fakeOperatorPasskeySessions struct {
	access   string
	refresh  string
	issueErr error
}

func (s *fakeOperatorPasskeySessions) IssueTokensForAuthenticatedOperator(_ context.Context, _ int64, _, _ string) (string, string, error) {
	return s.access, s.refresh, s.issueErr
}

// fakeOperatorPasskeyStore serves the credential and ceremony halves of the
// operator passkey store. A missing row is found=false.
type fakeOperatorPasskeyStore struct {
	rows        []domain.OperatorPasskeyCredential
	err         error
	alreadyGone bool

	createdSession  *domain.OperatorPasskeySession
	consumedSession *domain.OperatorPasskeySession
	createErr       error
	consumeErr      error

	revokedOperatorID   int64
	revokedCredentialID int64
}

func (s *fakeOperatorPasskeyStore) InsertOperatorPasskey(_ context.Context, credential domain.OperatorPasskeyCredential) (domain.OperatorPasskeyCredential, domain.OperationStats, error) {
	return credential, domain.OperationStats{}, s.err
}

func (s *fakeOperatorPasskeyStore) ListActiveOperatorPasskeys(context.Context, int64) ([]domain.OperatorPasskeyCredential, domain.OperationStats, error) {
	return s.rows, domain.OperationStats{}, s.err
}

func (s *fakeOperatorPasskeyStore) FindActiveOperatorPasskey(context.Context, []byte, []byte) (domain.OperatorPasskeyCredential, bool, domain.OperationStats, error) {
	if s.err != nil {
		return domain.OperatorPasskeyCredential{}, false, domain.OperationStats{}, s.err
	}
	if len(s.rows) == 0 {
		return domain.OperatorPasskeyCredential{}, false, domain.OperationStats{}, nil
	}
	return s.rows[0], true, domain.OperationStats{}, nil
}

func (s *fakeOperatorPasskeyStore) UpdateOperatorPasskeyAfterUse(context.Context, int64, []byte, time.Time) (bool, domain.OperationStats, error) {
	return true, domain.OperationStats{}, s.err
}

func (s *fakeOperatorPasskeyStore) RevokeOperatorPasskey(_ context.Context, operatorID, id int64, _ time.Time) (bool, domain.OperationStats, error) {
	s.revokedOperatorID, s.revokedCredentialID = operatorID, id
	if s.err != nil {
		return false, domain.OperationStats{}, s.err
	}
	return !s.alreadyGone, domain.OperationStats{}, nil
}

func (s *fakeOperatorPasskeyStore) InsertOperatorPasskeySession(_ context.Context, session domain.OperatorPasskeySession) (domain.OperatorPasskeySession, domain.OperationStats, error) {
	stored := session
	s.createdSession = &stored
	return session, domain.OperationStats{}, s.createErr
}

func (s *fakeOperatorPasskeyStore) ConsumeOperatorPasskeySession(context.Context, string, string, time.Time) (domain.OperatorPasskeySession, bool, domain.OperationStats, error) {
	if s.consumeErr != nil {
		return domain.OperatorPasskeySession{}, false, domain.OperationStats{}, s.consumeErr
	}
	if s.consumedSession == nil {
		return domain.OperatorPasskeySession{}, false, domain.OperationStats{}, nil
	}
	return *s.consumedSession, true, domain.OperationStats{}, nil
}

var _ ports.OperatorPasskeyStore = (*fakeOperatorPasskeyStore)(nil)

// newOperatorPasskeyFlows composes the ceremonies over the doubles, with the
// operators the flows resolve their user from.
func newOperatorPasskeyFlows(
	t *testing.T,
	store *fakeOperatorPasskeyStore,
	operators []domain.Operator,
	gate *fakeOperatorPasskeyGate,
	sessions *fakeOperatorPasskeySessions,
	transactions *passkeyTransactions,
) *OperatorPasskeyFlows {
	t.Helper()
	if store == nil {
		store = &fakeOperatorPasskeyStore{}
	}
	if gate == nil {
		gate = &fakeOperatorPasskeyGate{}
	}
	if sessions == nil {
		sessions = &fakeOperatorPasskeySessions{}
	}
	var runtime ports.Runtime = transactions
	if transactions == nil {
		runtime = &fakeRuntime{}
	}
	operatorStore := newFakeStore()
	for _, operator := range operators {
		operatorStore.operators[operator.ID] = operator
	}
	service := New(operatorStore, operatorStore, operatorStore, fakeTransaction{}, runtime.TenantID, func(ports.Observation) {})
	flows, err := NewOperatorPasskeyFlows(service, OperatorPasskeyFlowDependencies{
		Records: NewOperatorPasskey(service, store), MFA: gate, Sessions: sessions, Runtime: runtime,
		RPID: "localhost", RPName: "moto", OperatorFrontendURL: "http://operator.localhost:3000",
	})
	require.NoError(t, err)
	return flows
}

func passkeyOperator(id int64) domain.Operator {
	return domain.Operator{ID: id, Email: "ops@example.test", DisplayName: "Ops", Active: true}
}

// --- the enrollment challenge --------------------------------------------

func TestOperatorPasskeyEnrollmentChallenge(t *testing.T) {
	t.Parallel()

	operator := passkeyOperator(11)
	gate := &fakeOperatorPasskeyGate{challengeToken: "challenge-token"}
	flows := newOperatorPasskeyFlows(t, nil, []domain.Operator{operator}, gate, nil, nil)

	challenge, err := flows.StartEnrollmentChallenge(context.Background(), operator.ID, net.ParseIP("203.0.113.8"))
	require.NoError(t, err)
	assert.Equal(t, "challenge-token", challenge.ChallengeToken)
	assert.Contains(t, challenge.MaskedEmail, "@")
	assert.NotContains(t, challenge.MaskedEmail, "ops@", "the mailbox stays masked")
	assert.Equal(t, operator.ID, gate.startedOperatorID)
}

func TestOperatorPasskeyEnrollmentChallengeErrors(t *testing.T) {
	t.Parallel()

	operator := passkeyOperator(12)
	wantErr := errors.New("mfa down")

	tests := map[string]struct {
		operators []domain.Operator
		gate      *fakeOperatorPasskeyGate
	}{
		"unknown operator": {nil, &fakeOperatorPasskeyGate{}},
		"mfa start fails":  {[]domain.Operator{operator}, &fakeOperatorPasskeyGate{startErr: wantErr}},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			flows := newOperatorPasskeyFlows(t, nil, tt.operators, tt.gate, nil, nil)
			_, err := flows.StartEnrollmentChallenge(context.Background(), operator.ID, net.ParseIP("203.0.113.8"))
			require.Error(t, err)
		})
	}
}

// --- beginning a ceremony -------------------------------------------------

func TestOperatorPasskeyBeginRegistrationStoresSession(t *testing.T) {
	t.Parallel()

	operator := passkeyOperator(21)
	store := &fakeOperatorPasskeyStore{}
	gate := &fakeOperatorPasskeyGate{}
	flows := newOperatorPasskeyFlows(t, store, []domain.Operator{operator}, gate, nil, nil)

	creation, err := flows.BeginRegistration(context.Background(), domain.OperatorPasskeyRegistrationStart{
		OperatorID: operator.ID, ExpectedOrigin: "http://operator.localhost:3000",
		Code: "123456", Name: "YubiKey",
	})
	require.NoError(t, err)
	require.NotEmpty(t, creation.SessionID)
	require.NotNil(t, creation.Options)
	require.NotNil(t, store.createdSession)
	require.NotNil(t, store.createdSession.OperatorID)
	assert.Equal(t, operator.ID, *store.createdSession.OperatorID)
	assert.Equal(t, domain.PasskeySessionPurposeRegistration, store.createdSession.Purpose)
	assert.Equal(t, "operator.localhost", store.createdSession.RPID)
	assert.Equal(t, "http://operator.localhost:3000", store.createdSession.ExpectedOrigin)
	assert.False(t, store.createdSession.ExpiresAt.IsZero())
	assert.True(t, json.Valid(store.createdSession.SessionJSON))
	assert.Equal(t, operator.ID, gate.verifiedOperatorID)
	assert.Equal(t, "123456", gate.verifiedCode)
}

func TestOperatorPasskeyBeginRegistrationErrors(t *testing.T) {
	t.Parallel()

	operator := passkeyOperator(22)
	inactive := domain.Operator{ID: 23, Email: "inactive@example.test", DisplayName: "Ops"}
	wantErr := errors.New("boom")

	tests := []struct {
		name      string
		request   domain.OperatorPasskeyRegistrationStart
		operators []domain.Operator
		store     *fakeOperatorPasskeyStore
		gate      *fakeOperatorPasskeyGate
		wantErr   error
	}{
		{
			name: "invalid origin short circuits before mfa",
			request: domain.OperatorPasskeyRegistrationStart{
				OperatorID: operator.ID, ExpectedOrigin: "http://school.localhost:3000",
			},
			operators: []domain.Operator{operator},
			wantErr:   domain.ErrPasskeyOriginInvalid,
		},
		{
			name: "mfa verify fails",
			request: domain.OperatorPasskeyRegistrationStart{
				OperatorID: operator.ID, ExpectedOrigin: "http://operator.localhost:3000", Code: "000000",
			},
			operators: []domain.Operator{operator},
			gate:      &fakeOperatorPasskeyGate{verifyErr: wantErr},
		},
		{
			name: "unknown operator",
			request: domain.OperatorPasskeyRegistrationStart{
				OperatorID: 999, ExpectedOrigin: "http://operator.localhost:3000",
			},
			wantErr: domain.ErrOperatorNotFound,
		},
		{
			name: "inactive operator",
			request: domain.OperatorPasskeyRegistrationStart{
				OperatorID: inactive.ID, ExpectedOrigin: "http://operator.localhost:3000",
			},
			operators: []domain.Operator{inactive},
			wantErr:   domain.ErrOperatorInactive,
		},
		{
			name: "credential lookup fails while building the user",
			request: domain.OperatorPasskeyRegistrationStart{
				OperatorID: operator.ID, ExpectedOrigin: "http://operator.localhost:3000",
			},
			operators: []domain.Operator{operator},
			store:     &fakeOperatorPasskeyStore{err: wantErr},
		},
		{
			name: "session create fails",
			request: domain.OperatorPasskeyRegistrationStart{
				OperatorID: operator.ID, ExpectedOrigin: "http://operator.localhost:3000",
			},
			operators: []domain.Operator{operator},
			store:     &fakeOperatorPasskeyStore{createErr: wantErr},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			flows := newOperatorPasskeyFlows(t, tt.store, tt.operators, tt.gate, nil, nil)
			_, err := flows.BeginRegistration(context.Background(), tt.request)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				return
			}
			require.Error(t, err)
		})
	}
}

func TestOperatorPasskeyBeginLoginStoresSession(t *testing.T) {
	t.Parallel()

	store := &fakeOperatorPasskeyStore{}
	flows := newOperatorPasskeyFlows(t, store, nil, nil, nil, nil)

	assertion, err := flows.BeginLogin(context.Background(), "http://operator.localhost:3000")
	require.NoError(t, err)
	require.NotEmpty(t, assertion.SessionID)
	require.NotNil(t, assertion.Options)
	require.NotNil(t, store.createdSession)
	assert.Nil(t, store.createdSession.OperatorID, "a discoverable login learns the operator from the assertion")
	assert.Equal(t, domain.PasskeySessionPurposeLogin, store.createdSession.Purpose)
	assert.Equal(t, "operator.localhost", store.createdSession.RPID)
	assert.False(t, store.createdSession.ExpiresAt.IsZero())
}

func TestOperatorPasskeyBeginLoginErrors(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("session down")

	tests := []struct {
		name    string
		origin  string
		store   *fakeOperatorPasskeyStore
		wantErr error
	}{
		{name: "invalid origin", origin: "http://school.localhost:3000", wantErr: domain.ErrPasskeyOriginInvalid},
		{name: "session create fails", origin: "http://operator.localhost:3000", store: &fakeOperatorPasskeyStore{createErr: wantErr}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			flows := newOperatorPasskeyFlows(t, tt.store, nil, nil, nil, nil)
			_, err := flows.BeginLogin(context.Background(), tt.origin)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				return
			}
			require.Error(t, err)
		})
	}
}

// --- completing a ceremony ------------------------------------------------

func TestOperatorPasskeyFinishRegistrationRejectsInvalidSessionState(t *testing.T) {
	t.Parallel()

	operator := passkeyOperator(31)
	operatorID := operator.ID
	otherOperatorID := operator.ID + 1

	tests := []struct {
		name      string
		session   *domain.OperatorPasskeySession
		operators []domain.Operator
		store     *fakeOperatorPasskeyStore
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
			store:   &fakeOperatorPasskeyStore{consumeErr: errPasskeyStore},
			wantErr: errPasskeyStore,
		},
		{
			name: "missing operator id",
			session: &domain.OperatorPasskeySession{
				SessionJSON: json.RawMessage(`{}`), ExpectedOrigin: "http://operator.localhost:3000",
			},
			wantErr:   domain.ErrPasskeySessionInvalid,
			committed: true,
		},
		{
			name: "wrong operator id",
			session: &domain.OperatorPasskeySession{
				OperatorID: &otherOperatorID, SessionJSON: json.RawMessage(`{}`),
				ExpectedOrigin: "http://operator.localhost:3000",
			},
			wantErr:   domain.ErrPasskeySessionInvalid,
			committed: true,
		},
		{
			name: "invalid session json",
			session: &domain.OperatorPasskeySession{
				OperatorID: &operatorID, SessionJSON: json.RawMessage(`{`),
				ExpectedOrigin: "http://operator.localhost:3000",
			},
			committed: true,
		},
		{
			name: "unknown operator",
			session: &domain.OperatorPasskeySession{
				OperatorID: &operatorID, SessionJSON: json.RawMessage(`{}`),
				ExpectedOrigin: "http://operator.localhost:3000",
			},
			wantErr: domain.ErrOperatorNotFound,
		},
		{
			name: "credential lookup failure",
			session: &domain.OperatorPasskeySession{
				OperatorID: &operatorID, SessionJSON: json.RawMessage(`{}`),
				ExpectedOrigin: "http://operator.localhost:3000",
			},
			operators: []domain.Operator{operator},
			store:     &fakeOperatorPasskeyStore{err: errPasskeyStore},
			wantErr:   errPasskeyStore,
		},
		{
			name: "invalid response json",
			session: &domain.OperatorPasskeySession{
				OperatorID: &operatorID, SessionJSON: json.RawMessage(`{}`),
				ExpectedOrigin: "http://operator.localhost:3000",
			},
			operators: []domain.Operator{operator},
			wantErr:   domain.ErrPasskeySessionInvalid,
			committed: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			store := tt.store
			if store == nil {
				store = &fakeOperatorPasskeyStore{}
			}
			store.consumedSession = tt.session
			transactions := &passkeyTransactions{}
			flows := newOperatorPasskeyFlows(t, store, tt.operators, nil, nil, transactions)

			_, err := flows.FinishRegistration(context.Background(), domain.OperatorPasskeyRegistrationFinish{
				OperatorID:         operator.ID,
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

func TestOperatorPasskeyFinishLoginRejectsInvalidSessionState(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		session    *domain.OperatorPasskeySession
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
			name: "invalid session json",
			session: &domain.OperatorPasskeySession{
				SessionJSON: json.RawMessage(`{`), ExpectedOrigin: "http://operator.localhost:3000",
			},
			committed: true,
		},
		{
			name: "invalid response json",
			session: &domain.OperatorPasskeySession{
				SessionJSON: json.RawMessage(`{}`), ExpectedOrigin: "http://operator.localhost:3000",
			},
			wantErr:   domain.ErrPasskeySessionInvalid,
			committed: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			store := &fakeOperatorPasskeyStore{consumedSession: tt.session, consumeErr: tt.consumeErr}
			transactions := &passkeyTransactions{}
			flows := newOperatorPasskeyFlows(t, store, nil, nil, nil, transactions)

			_, err := flows.FinishLogin(context.Background(), domain.OperatorPasskeyLoginFinish{
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

// Every read failure behind the credential resolution must surface as the
// store error and roll the consumption back; an unknown credential, a gone
// operator and a failed signature read as a client error and spend the
// ceremony.
func TestOperatorPasskeyFinishLoginCredentialLookup(t *testing.T) {
	t.Parallel()

	operator := passkeyOperator(41)
	registered := domain.OperatorPasskeyCredential{
		ID: 42, OperatorID: operator.ID, UserHandle: []byte("user-handle"), CredentialJSON: json.RawMessage(`{}`),
	}

	tests := []struct {
		name      string
		store     *fakeOperatorPasskeyStore
		operators []domain.Operator
		wantErr   error
		committed bool
	}{
		{
			name:    "credential lookup failure",
			store:   &fakeOperatorPasskeyStore{err: errPasskeyStore},
			wantErr: errPasskeyStore,
		},
		{
			name:      "unknown credential",
			store:     &fakeOperatorPasskeyStore{},
			wantErr:   domain.ErrInvalidCredentials,
			committed: true,
		},
		{
			name:      "operator gone",
			store:     &fakeOperatorPasskeyStore{rows: []domain.OperatorPasskeyCredential{registered}},
			wantErr:   domain.ErrInvalidCredentials,
			committed: true,
		},
		{
			name:      "signature does not verify",
			store:     &fakeOperatorPasskeyStore{rows: []domain.OperatorPasskeyCredential{registered}},
			operators: []domain.Operator{operator},
			wantErr:   domain.ErrInvalidCredentials,
			committed: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tt.store.consumedSession = &domain.OperatorPasskeySession{
				SessionJSON:    json.RawMessage(`{"challenge":"Y2hhbGxlbmdl"}`),
				ExpectedOrigin: "http://school.localhost:3000",
			}
			transactions := &passkeyTransactions{}
			flows := newOperatorPasskeyFlows(t, tt.store, tt.operators, nil, nil, transactions)

			_, err := flows.FinishLogin(context.Background(), domain.OperatorPasskeyLoginFinish{
				SessionID:          "session-id",
				CredentialResponse: newPasskeyAssertionForTests(t, []byte("credential-id"), registered.UserHandle),
			})
			require.ErrorIs(t, err, tt.wantErr)
			transactions.assertOutcome(t, tt.committed)
		})
	}
}

// --- listing and revoking -------------------------------------------------

func TestOperatorPasskeyCredentialMethods(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()
	operator := passkeyOperator(91)
	row := domain.OperatorPasskeyCredential{
		ID: 92, CreatedAt: now, OperatorID: operator.ID,
		UserHandle: []byte("user-handle"), CredentialJSON: json.RawMessage(`{}`), Name: "YubiKey",
	}
	store := &fakeOperatorPasskeyStore{rows: []domain.OperatorPasskeyCredential{row}}
	flows := newOperatorPasskeyFlows(t, store, []domain.Operator{operator}, nil, nil, nil)

	list, err := flows.ListCredentials(context.Background(), operator.ID)
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, "92", list[0].ID)

	require.NoError(t, flows.RevokeCredential(context.Background(), operator.ID, row.ID))
	assert.Equal(t, operator.ID, store.revokedOperatorID)
	assert.Equal(t, row.ID, store.revokedCredentialID)

	user, err := flows.passkeyUser(context.Background(), operator)
	require.NoError(t, err)
	assert.Equal(t, []byte("user-handle"), user.WebAuthnID())
	assert.Equal(t, operator.Email, user.WebAuthnName())
	assert.Equal(t, operator.DisplayName, user.WebAuthnDisplayName())
	assert.Len(t, user.WebAuthnCredentials(), 1)

	store.rows = nil
	user, err = flows.passkeyUser(context.Background(), operator)
	require.NoError(t, err)
	assert.Len(t, user.WebAuthnID(), domain.PasskeyUserHandleBytes)

	user, err = flows.passkeyUser(context.Background(), operator, []byte("session-handle"))
	require.NoError(t, err)
	assert.Equal(t, []byte("session-handle"), user.WebAuthnID())

	store.rows = []domain.OperatorPasskeyCredential{{CredentialJSON: json.RawMessage(`{`)}}
	_, err = flows.passkeyUser(context.Background(), operator)
	require.Error(t, err)
}

func TestOperatorPasskeyCredentialErrors(t *testing.T) {
	t.Parallel()

	operator := passkeyOperator(1)
	failing := newOperatorPasskeyFlows(t, &fakeOperatorPasskeyStore{err: errPasskeyStore}, []domain.Operator{operator}, nil, nil, nil)

	_, err := failing.ListCredentials(context.Background(), operator.ID)
	require.ErrorIs(t, err, errPasskeyStore)

	err = failing.RevokeCredential(context.Background(), operator.ID, 2)
	require.ErrorIs(t, err, errPasskeyStore, "a store failure is not a missing passkey")
	require.NotErrorIs(t, err, domain.ErrPasskeyNotFound)

	_, err = failing.passkeyUser(context.Background(), operator)
	require.ErrorIs(t, err, errPasskeyStore)

	gone := newOperatorPasskeyFlows(t, &fakeOperatorPasskeyStore{alreadyGone: true}, []domain.Operator{operator}, nil, nil, nil)
	err = gone.RevokeCredential(context.Background(), operator.ID, 2)
	require.ErrorIs(t, err, domain.ErrPasskeyNotFound, "a passkey that is gone, revoked or foreign is not found")
}
