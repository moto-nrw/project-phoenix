package auth

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net"
	"strings"
	"testing"
	"time"

	authjwt "github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/database/repositories"
	authModel "github.com/moto-nrw/project-phoenix/models/auth"
	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func TestPasskeyTenantOriginValidation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		tenantDomain string
		origin       string
		subdomain    string
		wantErr      bool
	}{
		{
			name:         "localhost root accepted",
			tenantDomain: "localhost",
			origin:       "http://localhost:3000",
		},
		{
			name:         "localhost tenant subdomain accepted",
			tenantDomain: "localhost",
			origin:       "http://school.localhost:3000",
			subdomain:    "school",
		},
		{
			name:         "localhost wrong subdomain rejected",
			tenantDomain: "localhost",
			origin:       "http://other.localhost:3000",
			subdomain:    "school",
			wantErr:      true,
		},
		{
			name:         "production tenant subdomain accepted",
			tenantDomain: "moto-app.de",
			origin:       "https://school.moto-app.de",
			subdomain:    "school",
		},
		{
			name:         "production root rejected",
			tenantDomain: "moto-app.de",
			origin:       "https://moto-app.de",
			subdomain:    "school",
			wantErr:      true,
		},
		{
			name:         "production empty subdomain rejected",
			tenantDomain: "moto-app.de",
			origin:       "https://school.moto-app.de",
			wantErr:      true,
		},
		{
			name:         "invalid origin rejected",
			tenantDomain: "localhost",
			origin:       "not-a-url",
			wantErr:      true,
		},
		{
			name:         "unsupported origin scheme rejected",
			tenantDomain: "localhost",
			origin:       "ftp://localhost",
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &passkeyService{tenantDomain: tt.tenantDomain}

			err := svc.validateTenantOrigin(tt.origin, tt.subdomain)

			if tt.wantErr {
				require.ErrorIs(t, err, ErrPasskeyOriginInvalid)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestPasskeyHelpers(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "example.com", HostWithoutPort("https://Example.COM:443/ignored"))
	assert.Equal(t, "school.localhost", HostWithoutPort("school.localhost:3000"))

	host, err := OriginHostWithoutPort("http://school.localhost:3000")
	require.NoError(t, err)
	assert.Equal(t, "school.localhost", host)

	rpID, err := ResolvePasskeyRPIDForOrigin("localhost", "http://school.localhost:3000")
	require.NoError(t, err)
	assert.Equal(t, "school.localhost", rpID)

	rpID, err = ResolvePasskeyRPIDForOrigin("example.com", "https://school.example.com")
	require.NoError(t, err)
	assert.Equal(t, "example.com", rpID)

	assert.Equal(t, "Short name", NormalizePasskeyName("  Short name  "))
	assert.Empty(t, NormalizePasskeyName("   "))
	assert.Len(t, NormalizePasskeyName(strings.Repeat("a", 90)), 80)

	req, err := PasskeyResponseRequest(context.Background(), json.RawMessage(`{"id":"credential"}`))
	require.NoError(t, err)
	assert.Equal(t, "application/json", req.Header.Get("Content-Type"))

	_, err = PasskeyResponseRequest(context.Background(), json.RawMessage(`{`))
	require.ErrorIs(t, err, ErrPasskeySessionInvalid)

	platformReq, err := PasskeyResponseRequest(context.Background(), json.RawMessage(`{"id":"platform"}`))
	require.NoError(t, err)
	assert.Equal(t, "application/json", platformReq.Header.Get("Content-Type"))
	assert.Equal(t, NormalizePasskeyName("Example"), NormalizePasskeyName("Example"))
	assert.Equal(t, PasskeyUserHandleBytes, PasskeyUserHandleBytes)
}

func TestPasskeySummaryAndUser(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()
	lastUsedAt := now.Add(time.Minute)
	row := &PasskeyCredential{
		ID: 44, CreatedAt: now,
		Name:       "Laptop",
		LastUsedAt: &lastUsedAt,
	}
	summary := summarizePasskeyCredential(row)

	require.NotNil(t, summary)
	assert.Equal(t, "44", summary.ID)
	assert.Equal(t, "Laptop", summary.Name)
	assert.Equal(t, now, summary.CreatedAt)
	assert.Equal(t, &lastUsedAt, summary.LastUsedAt)

	user := &WebAuthnUser{
		UserHandle:  []byte("handle"),
		Name:        "teacher@example.test",
		DisplayName: "Teacher",
	}
	assert.Equal(t, []byte("handle"), user.WebAuthnID())
	assert.Equal(t, "teacher@example.test", user.WebAuthnName())
	assert.Equal(t, "Teacher", user.WebAuthnDisplayName())
	assert.Empty(t, user.WebAuthnCredentials())
}

func TestNewPasskeyServiceValidation(t *testing.T) {
	t.Parallel()

	baseCfg := PasskeyServiceConfig{
		Repos:        &repositories.Factory{},
		Records:      &passkeyRecordsStub{},
		MFAService:   &MFAStub{},
		AuthService:  &Service{},
		DB:           &bun.DB{},
		RPID:         "localhost:3000",
		RPName:       "moto tests",
		TenantDomain: "localhost:3000",
	}

	tests := []struct {
		name   string
		mutate func(*PasskeyServiceConfig)
	}{
		{name: "missing repos", mutate: func(cfg *PasskeyServiceConfig) { cfg.Repos = nil }},
		{name: "missing records", mutate: func(cfg *PasskeyServiceConfig) { cfg.Records = nil }},
		{name: "missing mfa", mutate: func(cfg *PasskeyServiceConfig) { cfg.MFAService = nil }},
		{name: "missing auth", mutate: func(cfg *PasskeyServiceConfig) { cfg.AuthService = nil }},
		{name: "missing db", mutate: func(cfg *PasskeyServiceConfig) { cfg.DB = nil }},
		{name: "missing rp id", mutate: func(cfg *PasskeyServiceConfig) { cfg.RPID = " " }},
		{name: "missing tenant domain", mutate: func(cfg *PasskeyServiceConfig) { cfg.TenantDomain = " " }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := baseCfg
			tt.mutate(&cfg)
			_, err := NewPasskeyService(cfg)
			require.Error(t, err)
		})
	}

	svc, err := NewPasskeyService(baseCfg)
	require.NoError(t, err)
	concrete := svc.(*passkeyService)
	assert.Equal(t, "localhost", concrete.rpID)
	assert.Equal(t, "localhost", concrete.tenantDomain)

	cfgWithDefaultName := baseCfg
	cfgWithDefaultName.RPName = " "
	svc, err = NewPasskeyService(cfgWithDefaultName)
	require.NoError(t, err)
	assert.Equal(t, "moto", svc.(*passkeyService).rpName)
}

func TestPasskeyEnrollmentChallenge(t *testing.T) {
	t.Parallel()

	account := &authModel.Account{Model: base.Model{ID: 11}, Email: "teacher@example.test", Active: true}
	mfa := &MFAStub{ChallengeToken: "challenge-token"}
	svc := &passkeyService{
		repos:      &repositories.Factory{Account: newStubAccountRepository(account)},
		mfaService: mfa,
	}

	challenge, err := svc.StartEnrollmentChallenge(context.Background(), account.ID, 42, net.ParseIP("203.0.113.8"))
	require.NoError(t, err)
	assert.Equal(t, "challenge-token", challenge.ChallengeToken)
	assert.Contains(t, challenge.MaskedEmail, "@")
	assert.Equal(t, account.ID, mfa.StartedAccountID)
	assert.Equal(t, int64(42), mfa.StartedTenantID)
	assert.Equal(t, authjwt.MFAChallengeScopeTenant, mfa.StartedScope)
}

func TestPasskeyEnrollmentChallengeErrors(t *testing.T) {
	t.Parallel()

	account := &authModel.Account{Model: base.Model{ID: 12}, Email: "teacher@example.test", Active: true}
	wantErr := errors.New("mfa down")

	tests := []struct {
		name  string
		repos *repositories.Factory
		mfa   *MFAStub
	}{
		{
			name:  "unknown account",
			repos: &repositories.Factory{Account: newStubAccountRepository()},
			mfa:   &MFAStub{},
		},
		{
			name:  "mfa start fails",
			repos: &repositories.Factory{Account: newStubAccountRepository(account)},
			mfa:   &MFAStub{StartErr: wantErr},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &passkeyService{repos: tt.repos, mfaService: tt.mfa}
			_, err := svc.StartEnrollmentChallenge(context.Background(), account.ID, 42, net.ParseIP("203.0.113.8"))
			require.Error(t, err)
		})
	}
}

// passkeyRecordsStub serves the records port from a credential and a
// session stub.
type passkeyRecordsStub struct {
	credentials *passkeyCredentialRepoStub
	sessions    *passkeySessionRepoStub
}

func (r *passkeyRecordsStub) CreateCredential(ctx context.Context, credential *PasskeyCredential) error {
	return r.credentials.create(ctx, credential)
}

func (r *passkeyRecordsStub) ListActiveCredentials(ctx context.Context, accountID int64) ([]*PasskeyCredential, error) {
	return r.credentials.listActive(ctx, accountID)
}

func (r *passkeyRecordsStub) FindActiveCredential(ctx context.Context, credentialID, userHandle []byte) (*PasskeyCredential, error) {
	return r.credentials.findActive(ctx, credentialID, userHandle)
}

func (r *passkeyRecordsStub) RecordCredentialUse(ctx context.Context, id int64, credentialJSON []byte, usedAt time.Time) error {
	return r.credentials.recordUse(ctx, id, credentialJSON, usedAt)
}

func (r *passkeyRecordsStub) RevokeCredential(ctx context.Context, accountID, id int64, revokedAt time.Time) (bool, error) {
	return r.credentials.revoke(ctx, accountID, id, revokedAt)
}

func (r *passkeyRecordsStub) CreateSession(ctx context.Context, session *PasskeySession) error {
	return r.sessions.create(ctx, session)
}

func (r *passkeyRecordsStub) ConsumeSession(ctx context.Context, id, purpose string, consumedAt time.Time) (*PasskeySession, error) {
	return r.sessions.consume(ctx, id, purpose, consumedAt)
}

// passkeyTransactions is a unit of work without a database that counts how
// the ceremony completions ended.
type passkeyTransactions struct {
	commits   int
	rollbacks int
}

// ctx carries the counting unit of work the way the API root attaches the
// real one.
func (l *passkeyTransactions) ctx(t *testing.T) context.Context {
	t.Helper()
	runtime, err := tenant.NewUnitOfWork(
		func(context.Context, int64, func(context.Context, any) error) error {
			t.Fatal("ceremony completions never open a tenant transaction")
			return nil
		},
		func(ctx context.Context, fn func(context.Context, any) error) error {
			err := fn(ctx, bun.Tx{})
			if err != nil {
				l.rollbacks++
			} else {
				l.commits++
			}
			return err
		},
		func(context.Context, tenant.SavepointAction) error { return nil },
		func(error) bool { return false },
	)
	require.NoError(t, err)
	return tenant.WithUnitOfWork(context.Background(), runtime)
}

// assertOutcome checks that the completion ran in exactly one transaction
// that committed (a rejected ceremony stays spent) or rolled back (a failed
// read or write leaves the ceremony open).
func (l *passkeyTransactions) assertOutcome(t *testing.T, committed bool) {
	t.Helper()
	if committed {
		assert.Equal(t, passkeyTransactions{commits: 1}, *l, "a rejected ceremony stays spent")
		return
	}
	assert.Equal(t, passkeyTransactions{rollbacks: 1}, *l, "a failed read or write rolls the consumption back")
}

func TestPasskeyBeginRegistrationStoresSession(t *testing.T) {
	t.Parallel()

	account := &authModel.Account{Model: base.Model{ID: 21}, Email: "teacher@example.test", Active: true}
	sessions := &passkeySessionRepoStub{}
	mfa := &MFAStub{}
	svc := &passkeyService{
		repos:        &repositories.Factory{Account: newStubAccountRepository(account)},
		records:      &passkeyRecordsStub{credentials: &passkeyCredentialRepoStub{}, sessions: sessions},
		mfaService:   mfa,
		rpID:         "localhost",
		rpName:       "moto",
		tenantDomain: "localhost",
	}

	creation, err := svc.BeginRegistration(context.Background(), PasskeyRegistrationStartRequest{
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
	require.NotNil(t, sessions.created)
	require.NotNil(t, sessions.created.AccountID)
	require.NotNil(t, sessions.created.TenantID)
	assert.Equal(t, account.ID, *sessions.created.AccountID)
	assert.Equal(t, int64(44), *sessions.created.TenantID)
	assert.Equal(t, PasskeySessionPurposeRegistration, sessions.created.Purpose)
	assert.Equal(t, "school.localhost", sessions.created.RPID)
	assert.Equal(t, "http://school.localhost:3000", sessions.created.ExpectedOrigin)
	assert.False(t, sessions.created.ExpiresAt.IsZero())
	assert.True(t, json.Valid(sessions.created.SessionJSON))
	assert.Equal(t, account.ID, mfa.VerifiedAccountID)
	assert.Equal(t, "123456", mfa.VerifiedCode)
}

func TestPasskeyBeginRegistrationErrors(t *testing.T) {
	t.Parallel()

	account := &authModel.Account{Model: base.Model{ID: 22}, Email: "teacher@example.test", Active: true}
	inactive := &authModel.Account{Model: base.Model{ID: 23}, Email: "inactive@example.test", Active: false}
	wantErr := errors.New("boom")

	tests := []struct {
		name    string
		req     PasskeyRegistrationStartRequest
		repos   *repositories.Factory
		records *passkeyRecordsStub
		mfa     *MFAStub
		wantErr error
	}{
		{
			name: "invalid origin short circuits before mfa",
			req: PasskeyRegistrationStartRequest{
				AccountID:      account.ID,
				ExpectedOrigin: "http://other.localhost:3000",
			},
			repos:   &repositories.Factory{Account: newStubAccountRepository(account)},
			records: &passkeyRecordsStub{},
			mfa:     &MFAStub{},
			wantErr: ErrPasskeyOriginInvalid,
		},
		{
			name: "mfa verify fails",
			req: PasskeyRegistrationStartRequest{
				AccountID:      account.ID,
				ExpectedOrigin: "http://localhost:3000",
				Code:           "000000",
			},
			repos:   &repositories.Factory{Account: newStubAccountRepository(account)},
			records: &passkeyRecordsStub{},
			mfa:     &MFAStub{VerifyErr: wantErr},
		},
		{
			name: "unknown account",
			req: PasskeyRegistrationStartRequest{
				AccountID:      999,
				ExpectedOrigin: "http://localhost:3000",
			},
			repos:   &repositories.Factory{Account: newStubAccountRepository()},
			records: &passkeyRecordsStub{},
			mfa:     &MFAStub{},
			wantErr: ErrAccountNotFound,
		},
		{
			name: "inactive account",
			req: PasskeyRegistrationStartRequest{
				AccountID:      inactive.ID,
				ExpectedOrigin: "http://localhost:3000",
			},
			repos:   &repositories.Factory{Account: newStubAccountRepository(inactive)},
			records: &passkeyRecordsStub{},
			mfa:     &MFAStub{},
			wantErr: ErrAccountInactive,
		},
		{
			name: "credential lookup fails while building user",
			req: PasskeyRegistrationStartRequest{
				AccountID:      account.ID,
				ExpectedOrigin: "http://localhost:3000",
			},
			repos:   &repositories.Factory{Account: newStubAccountRepository(account)},
			records: &passkeyRecordsStub{credentials: &passkeyCredentialRepoStub{err: wantErr}},
			mfa:     &MFAStub{},
		},
		{
			name: "session create fails",
			req: PasskeyRegistrationStartRequest{
				AccountID:      account.ID,
				ExpectedOrigin: "http://localhost:3000",
			},
			repos: &repositories.Factory{Account: newStubAccountRepository(account)},
			records: &passkeyRecordsStub{
				credentials: &passkeyCredentialRepoStub{},
				sessions:    &passkeySessionRepoStub{createErr: wantErr},
			},
			mfa: &MFAStub{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &passkeyService{
				repos:        tt.repos,
				records:      tt.records,
				mfaService:   tt.mfa,
				rpID:         "localhost",
				rpName:       "moto",
				tenantDomain: "localhost",
			}
			_, err := svc.BeginRegistration(context.Background(), tt.req)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				return
			}
			require.Error(t, err)
		})
	}
}

func TestPasskeyBeginLoginStoresSession(t *testing.T) {
	t.Parallel()

	sessions := &passkeySessionRepoStub{}
	svc := &passkeyService{
		records:      &passkeyRecordsStub{sessions: sessions},
		rpID:         "localhost",
		rpName:       "moto",
		tenantDomain: "localhost",
	}

	assertion, err := svc.BeginLogin(context.Background(), PasskeyLoginStartRequest{
		TenantID:        45,
		TenantSubdomain: "school",
		ExpectedOrigin:  "http://school.localhost:3000",
	})
	require.NoError(t, err)
	require.NotEmpty(t, assertion.SessionID)
	require.NotNil(t, assertion.Options)
	require.NotNil(t, sessions.created)
	require.NotNil(t, sessions.created.TenantID)
	assert.Equal(t, int64(45), *sessions.created.TenantID)
	assert.Equal(t, PasskeySessionPurposeLogin, sessions.created.Purpose)
	assert.Equal(t, "school.localhost", sessions.created.RPID)
	assert.Equal(t, "http://school.localhost:3000", sessions.created.ExpectedOrigin)
	assert.False(t, sessions.created.ExpiresAt.IsZero())
}

func TestPasskeyBeginLoginErrors(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("session down")

	tests := []struct {
		name    string
		req     PasskeyLoginStartRequest
		session *passkeySessionRepoStub
		wantErr error
	}{
		{
			name: "invalid origin",
			req: PasskeyLoginStartRequest{
				TenantSubdomain: "school",
				ExpectedOrigin:  "http://other.localhost:3000",
			},
			session: &passkeySessionRepoStub{},
			wantErr: ErrPasskeyOriginInvalid,
		},
		{
			name: "session create fails",
			req: PasskeyLoginStartRequest{
				TenantID:       45,
				ExpectedOrigin: "http://localhost:3000",
			},
			session: &passkeySessionRepoStub{createErr: wantErr},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &passkeyService{
				records:      &passkeyRecordsStub{sessions: tt.session},
				rpID:         "localhost",
				rpName:       "moto",
				tenantDomain: "localhost",
			}
			_, err := svc.BeginLogin(context.Background(), tt.req)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				return
			}
			require.Error(t, err)
		})
	}
}

func TestPasskeyFinishRegistrationRejectsInvalidSessionState(t *testing.T) {
	t.Parallel()

	account := &authModel.Account{Model: base.Model{ID: 31}, Email: "teacher@example.test", Active: true}
	otherAccountID := account.ID + 1

	tests := []struct {
		name      string
		session   *PasskeySession
		repos     *repositories.Factory
		records   *passkeyRecordsStub
		wantErr   error
		committed bool
	}{
		{
			name:      "no pending ceremony",
			records:   &passkeyRecordsStub{sessions: &passkeySessionRepoStub{}},
			wantErr:   ErrPasskeySessionInvalid,
			committed: true,
		},
		{
			name:    "consume store failure is not an invalid session",
			records: &passkeyRecordsStub{sessions: &passkeySessionRepoStub{consumeErr: errPasskeyStore}},
			wantErr: errPasskeyStore,
		},
		{
			name: "missing account id",
			session: &PasskeySession{
				SessionJSON:    json.RawMessage(`{}`),
				ExpectedOrigin: "http://localhost:3000",
			},
			records:   &passkeyRecordsStub{sessions: &passkeySessionRepoStub{}},
			wantErr:   ErrPasskeySessionInvalid,
			committed: true,
		},
		{
			name: "wrong account id",
			session: &PasskeySession{
				AccountID:      &otherAccountID,
				SessionJSON:    json.RawMessage(`{}`),
				ExpectedOrigin: "http://localhost:3000",
			},
			records:   &passkeyRecordsStub{sessions: &passkeySessionRepoStub{}},
			wantErr:   ErrPasskeySessionInvalid,
			committed: true,
		},
		{
			name: "invalid session json",
			session: &PasskeySession{
				AccountID:      &account.ID,
				SessionJSON:    json.RawMessage(`{`),
				ExpectedOrigin: "http://localhost:3000",
			},
			records:   &passkeyRecordsStub{sessions: &passkeySessionRepoStub{}},
			committed: true,
		},
		{
			name: "unknown account",
			session: &PasskeySession{
				AccountID:      &account.ID,
				SessionJSON:    json.RawMessage(`{}`),
				ExpectedOrigin: "http://localhost:3000",
			},
			repos:     &repositories.Factory{Account: newStubAccountRepository()},
			records:   &passkeyRecordsStub{sessions: &passkeySessionRepoStub{}},
			wantErr:   ErrAccountNotFound,
			committed: true,
		},
		{
			name: "account read failure is not a missing account",
			session: &PasskeySession{
				AccountID:      &account.ID,
				SessionJSON:    json.RawMessage(`{}`),
				ExpectedOrigin: "http://localhost:3000",
			},
			repos:   &repositories.Factory{Account: failingAccountRepository{err: errPasskeyStore}},
			records: &passkeyRecordsStub{sessions: &passkeySessionRepoStub{}},
			wantErr: errPasskeyStore,
		},
		{
			name: "credential lookup failure",
			session: &PasskeySession{
				AccountID:      &account.ID,
				SessionJSON:    json.RawMessage(`{}`),
				ExpectedOrigin: "http://localhost:3000",
			},
			repos: &repositories.Factory{Account: newStubAccountRepository(account)},
			records: &passkeyRecordsStub{
				credentials: &passkeyCredentialRepoStub{err: errPasskeyStore},
				sessions:    &passkeySessionRepoStub{},
			},
			wantErr: errPasskeyStore,
		},
		{
			name: "invalid response json",
			session: &PasskeySession{
				AccountID:      &account.ID,
				SessionJSON:    json.RawMessage(`{}`),
				ExpectedOrigin: "http://localhost:3000",
			},
			repos: &repositories.Factory{Account: newStubAccountRepository(account)},
			records: &passkeyRecordsStub{
				credentials: &passkeyCredentialRepoStub{},
				sessions:    &passkeySessionRepoStub{},
			},
			wantErr:   ErrPasskeySessionInvalid,
			committed: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.records.sessions.consumed = tt.session
			var transactions passkeyTransactions
			svc := &passkeyService{repos: tt.repos, records: tt.records, rpID: "localhost", rpName: "moto"}
			_, err := svc.FinishRegistration(transactions.ctx(t), PasskeyRegistrationFinishRequest{
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

// TestPasskeyCompletionRequiresTenantRuntime pins the fail-closed wiring:
// without a unit of work nothing is consumed.
func TestPasskeyCompletionRequiresTenantRuntime(t *testing.T) {
	t.Parallel()

	sessions := &passkeySessionRepoStub{consumeErr: errPasskeyStore}
	svc := &passkeyService{records: &passkeyRecordsStub{sessions: sessions}}

	_, err := svc.FinishLogin(context.Background(), PasskeyLoginFinishRequest{SessionID: "session-id"})
	require.ErrorIs(t, err, tenant.ErrRuntimeRequired)
	_, err = svc.FinishRegistration(context.Background(), PasskeyRegistrationFinishRequest{SessionID: "session-id"})
	require.ErrorIs(t, err, tenant.ErrRuntimeRequired)
}

func TestPasskeyFinishLoginRejectsInvalidSessionState(t *testing.T) {
	t.Parallel()

	tenantID := testpkg.UniqueTestTenantID(t)

	tests := []struct {
		name       string
		session    *PasskeySession
		consumeErr error
		wantErr    error
		committed  bool
	}{
		{
			name:      "no pending ceremony",
			wantErr:   ErrPasskeySessionInvalid,
			committed: true,
		},
		{
			name:       "consume store failure is not an invalid session",
			consumeErr: errPasskeyStore,
			wantErr:    errPasskeyStore,
		},
		{
			name: "missing tenant id",
			session: &PasskeySession{
				SessionJSON:    json.RawMessage(`{}`),
				ExpectedOrigin: "http://localhost:3000",
			},
			wantErr:   ErrPasskeySessionInvalid,
			committed: true,
		},
		{
			name: "invalid session json",
			session: &PasskeySession{
				TenantID:       &tenantID,
				SessionJSON:    json.RawMessage(`{`),
				ExpectedOrigin: "http://localhost:3000",
			},
			committed: true,
		},
		{
			name: "invalid response json",
			session: &PasskeySession{
				TenantID:       &tenantID,
				SessionJSON:    json.RawMessage(`{}`),
				ExpectedOrigin: "http://localhost:3000",
			},
			wantErr:   ErrPasskeySessionInvalid,
			committed: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sessionRepo := &passkeySessionRepoStub{consumed: tt.session, consumeErr: tt.consumeErr}
			var transactions passkeyTransactions
			svc := &passkeyService{
				records: &passkeyRecordsStub{sessions: sessionRepo},
				rpID:    "localhost",
				rpName:  "moto",
			}
			_, err := svc.FinishLogin(transactions.ctx(t), PasskeyLoginFinishRequest{
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

// NewPasskeyAssertionForTests returns a parseable discoverable-login
// response for credentialID and userHandle. The WebAuthn library resolves
// the credential from it before checking the signature, so it drives
// FinishLogin to the credential lookup and fails verification afterwards.
func NewPasskeyAssertionForTests(t *testing.T, credentialID, userHandle []byte) json.RawMessage {
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
func TestPasskeyFinishLoginCredentialLookup(t *testing.T) {
	t.Parallel()

	tenantID := testpkg.UniqueTestTenantID(t)
	account := &authModel.Account{Model: base.Model{ID: 41}, Email: "teacher@example.test", Active: true}
	registered := &PasskeyCredential{
		ID: 42, AccountID: account.ID, UserHandle: []byte("user-handle"), CredentialJSON: json.RawMessage(`{}`),
	}
	tests := []struct {
		name        string
		credentials *passkeyCredentialRepoStub
		repos       *repositories.Factory
		wantErr     error
		committed   bool
	}{
		{
			name:        "credential lookup failure",
			credentials: &passkeyCredentialRepoStub{err: errPasskeyStore},
			wantErr:     errPasskeyStore,
		},
		{
			name:        "unknown credential",
			credentials: &passkeyCredentialRepoStub{},
			wantErr:     ErrInvalidCredentials,
			committed:   true,
		},
		{
			name:        "account read failure",
			credentials: &passkeyCredentialRepoStub{rows: []*PasskeyCredential{registered}},
			repos:       &repositories.Factory{Account: failingAccountRepository{err: errPasskeyStore}},
			wantErr:     errPasskeyStore,
		},
		{
			name:        "account gone",
			credentials: &passkeyCredentialRepoStub{rows: []*PasskeyCredential{registered}},
			repos:       &repositories.Factory{Account: newStubAccountRepository()},
			wantErr:     ErrInvalidCredentials,
			committed:   true,
		},
		{
			name:        "signature does not verify",
			credentials: &passkeyCredentialRepoStub{rows: []*PasskeyCredential{registered}},
			repos:       &repositories.Factory{Account: newStubAccountRepository(account)},
			wantErr:     ErrInvalidCredentials,
			committed:   true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sessions := &passkeySessionRepoStub{consumed: &PasskeySession{
				TenantID:       &tenantID,
				SessionJSON:    json.RawMessage(`{"challenge":"Y2hhbGxlbmdl"}`),
				ExpectedOrigin: "http://school.localhost:3000",
			}}
			var transactions passkeyTransactions
			svc := &passkeyService{
				repos:        tt.repos,
				records:      &passkeyRecordsStub{credentials: tt.credentials, sessions: sessions},
				rpID:         "localhost",
				rpName:       "moto",
				tenantDomain: "localhost",
			}
			_, err := svc.FinishLogin(transactions.ctx(t), PasskeyLoginFinishRequest{
				SessionID:          "session-id",
				CredentialResponse: NewPasskeyAssertionForTests(t, []byte("credential-id"), registered.UserHandle),
			})
			require.ErrorIs(t, err, tt.wantErr)
			transactions.assertOutcome(t, tt.committed)
		})
	}
}

func TestPasskeyCredentialServiceMethods(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()
	account := &authModel.Account{Model: base.Model{ID: 91}, Email: "teacher@example.test"}
	row := &PasskeyCredential{
		ID:             92,
		CreatedAt:      now,
		AccountID:      account.ID,
		UserHandle:     []byte("user-handle"),
		CredentialJSON: json.RawMessage(`{}`),
		Name:           "Laptop",
	}
	repo := &passkeyCredentialRepoStub{rows: []*PasskeyCredential{row}}
	svc := &passkeyService{records: &passkeyRecordsStub{credentials: repo}}

	list, err := svc.ListCredentials(context.Background(), account.ID)
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, "92", list[0].ID)

	require.NoError(t, svc.RevokeCredential(context.Background(), account.ID, row.ID))
	assert.Equal(t, account.ID, repo.revokedAccountID)
	assert.Equal(t, row.ID, repo.revokedCredentialID)

	user, err := svc.passkeyUserForAccount(context.Background(), account)
	require.NoError(t, err)
	assert.Equal(t, []byte("user-handle"), user.WebAuthnID())
	assert.Equal(t, account.Email, user.WebAuthnName())
	assert.Len(t, user.WebAuthnCredentials(), 1)

	repo.rows = nil
	user, err = svc.passkeyUserForAccount(context.Background(), account)
	require.NoError(t, err)
	assert.Len(t, user.WebAuthnID(), PasskeyUserHandleBytes)

	user, err = svc.passkeyUserForAccount(context.Background(), account, []byte("session-handle"))
	require.NoError(t, err)
	assert.Equal(t, []byte("session-handle"), user.WebAuthnID())

	repo.rows = []*PasskeyCredential{{CredentialJSON: json.RawMessage(`{`)}}
	_, err = svc.passkeyUserForAccount(context.Background(), account)
	require.Error(t, err)
}

func TestPasskeyCredentialServiceErrors(t *testing.T) {
	t.Parallel()

	repo := &passkeyCredentialRepoStub{err: errPasskeyStore}
	svc := &passkeyService{records: &passkeyRecordsStub{credentials: repo}}

	_, err := svc.ListCredentials(context.Background(), 1)
	require.ErrorIs(t, err, errPasskeyStore)

	err = svc.RevokeCredential(context.Background(), 1, 2)
	require.ErrorIs(t, err, errPasskeyStore, "a store failure is not a missing passkey")
	require.NotErrorIs(t, err, ErrPasskeyNotFound)

	_, err = svc.passkeyUserForAccount(context.Background(), &authModel.Account{Email: "teacher@example.test"})
	require.ErrorIs(t, err, errPasskeyStore)

	missing := &passkeyService{records: &passkeyRecordsStub{credentials: &passkeyCredentialRepoStub{alreadyGone: true}}}
	err = missing.RevokeCredential(context.Background(), 1, 2)
	require.ErrorIs(t, err, ErrPasskeyNotFound, "a passkey that is gone, revoked or foreign is not found")
}

var errPasskeyStore = errors.New("passkey store down")

// failingAccountRepository fails the account lookup like an unavailable
// store; a missing account is sql.ErrNoRows, which this is not.
type failingAccountRepository struct {
	noopAccountRepository
	err error
}

func (r failingAccountRepository) FindByID(context.Context, interface{}) (*authModel.Account, error) {
	return nil, r.err
}

// passkeyCredentialRepoStub serves the credential half of the records port.
type passkeyCredentialRepoStub struct {
	rows                []*PasskeyCredential
	err                 error
	alreadyGone         bool
	revokedAccountID    int64
	revokedCredentialID int64
}

func (r *passkeyCredentialRepoStub) create(context.Context, *PasskeyCredential) error {
	return r.err
}

func (r *passkeyCredentialRepoStub) listActive(context.Context, int64) ([]*PasskeyCredential, error) {
	return r.rows, r.err
}

// findActive follows the port contract: a missing row is (nil, nil).
func (r *passkeyCredentialRepoStub) findActive(context.Context, []byte, []byte) (*PasskeyCredential, error) {
	if r.err != nil {
		return nil, r.err
	}
	if len(r.rows) == 0 {
		return nil, nil
	}
	return r.rows[0], nil
}

func (r *passkeyCredentialRepoStub) recordUse(context.Context, int64, []byte, time.Time) error {
	return r.err
}

func (r *passkeyCredentialRepoStub) revoke(_ context.Context, accountID, id int64, _ time.Time) (bool, error) {
	r.revokedAccountID = accountID
	r.revokedCredentialID = id
	if r.err != nil {
		return false, r.err
	}
	return !r.alreadyGone, nil
}

// passkeySessionRepoStub serves the ceremony half of the records port.
type passkeySessionRepoStub struct {
	created    *PasskeySession
	consumed   *PasskeySession
	createErr  error
	consumeErr error
}

func (r *passkeySessionRepoStub) create(_ context.Context, session *PasskeySession) error {
	r.created = session
	return r.createErr
}

// consume follows the port contract: no pending ceremony is (nil, nil).
func (r *passkeySessionRepoStub) consume(context.Context, string, string, time.Time) (*PasskeySession, error) {
	if r.consumeErr != nil {
		return nil, r.consumeErr
	}
	return r.consumed, nil
}

// MFAStub is defined in mfa_stub_test.go (shared with auth_login_mfa_infra_test.go).
