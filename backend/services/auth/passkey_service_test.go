package auth

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"

	authjwt "github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/database/repositories"
	authModel "github.com/moto-nrw/project-phoenix/models/auth"
	"github.com/moto-nrw/project-phoenix/models/base"
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
	row := &authModel.PasskeyCredential{
		Model:      base.Model{ID: 44, CreatedAt: now},
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

func TestPasskeyBeginRegistrationStoresSession(t *testing.T) {
	t.Parallel()

	account := &authModel.Account{Model: base.Model{ID: 21}, Email: "teacher@example.test", Active: true}
	sessions := &passkeySessionRepoStub{}
	mfa := &MFAStub{}
	svc := &passkeyService{
		repos: &repositories.Factory{
			Account:           newStubAccountRepository(account),
			PasskeyCredential: &passkeyCredentialRepoStub{},
			PasskeySession:    sessions,
		},
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
	assert.Equal(t, authModel.PasskeySessionPurposeRegistration, sessions.created.Purpose)
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
			repos: &repositories.Factory{Account: newStubAccountRepository(account)},
			mfa:   &MFAStub{VerifyErr: wantErr},
		},
		{
			name: "unknown account",
			req: PasskeyRegistrationStartRequest{
				AccountID:      999,
				ExpectedOrigin: "http://localhost:3000",
			},
			repos:   &repositories.Factory{Account: newStubAccountRepository()},
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
			mfa:     &MFAStub{},
			wantErr: ErrAccountInactive,
		},
		{
			name: "credential lookup fails while building user",
			req: PasskeyRegistrationStartRequest{
				AccountID:      account.ID,
				ExpectedOrigin: "http://localhost:3000",
			},
			repos: &repositories.Factory{
				Account:           newStubAccountRepository(account),
				PasskeyCredential: &passkeyCredentialRepoStub{err: wantErr},
			},
			mfa: &MFAStub{},
		},
		{
			name: "session create fails",
			req: PasskeyRegistrationStartRequest{
				AccountID:      account.ID,
				ExpectedOrigin: "http://localhost:3000",
			},
			repos: &repositories.Factory{
				Account:           newStubAccountRepository(account),
				PasskeyCredential: &passkeyCredentialRepoStub{},
				PasskeySession:    &passkeySessionRepoStub{createErr: wantErr},
			},
			mfa: &MFAStub{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &passkeyService{
				repos:        tt.repos,
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
		repos:        &repositories.Factory{PasskeySession: sessions},
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
	assert.Equal(t, authModel.PasskeySessionPurposeLogin, sessions.created.Purpose)
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
				repos:        &repositories.Factory{PasskeySession: tt.session},
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
		name    string
		session *authModel.PasskeySession
		repos   *repositories.Factory
		wantErr error
	}{
		{
			name: "consume fails",
			repos: &repositories.Factory{
				PasskeySession: &passkeySessionRepoStub{consumeErr: sql.ErrNoRows},
			},
			wantErr: ErrPasskeySessionInvalid,
		},
		{
			name: "missing account id",
			session: &authModel.PasskeySession{
				SessionJSON:    json.RawMessage(`{}`),
				ExpectedOrigin: "http://localhost:3000",
			},
			repos:   &repositories.Factory{PasskeySession: &passkeySessionRepoStub{}},
			wantErr: ErrPasskeySessionInvalid,
		},
		{
			name: "wrong account id",
			session: &authModel.PasskeySession{
				AccountID:      &otherAccountID,
				SessionJSON:    json.RawMessage(`{}`),
				ExpectedOrigin: "http://localhost:3000",
			},
			repos:   &repositories.Factory{PasskeySession: &passkeySessionRepoStub{}},
			wantErr: ErrPasskeySessionInvalid,
		},
		{
			name: "invalid session json",
			session: &authModel.PasskeySession{
				AccountID:      &account.ID,
				SessionJSON:    json.RawMessage(`{`),
				ExpectedOrigin: "http://localhost:3000",
			},
			repos: &repositories.Factory{PasskeySession: &passkeySessionRepoStub{}},
		},
		{
			name: "unknown account",
			session: &authModel.PasskeySession{
				AccountID:      &account.ID,
				SessionJSON:    json.RawMessage(`{}`),
				ExpectedOrigin: "http://localhost:3000",
			},
			repos: &repositories.Factory{
				Account:        newStubAccountRepository(),
				PasskeySession: &passkeySessionRepoStub{},
			},
			wantErr: ErrAccountNotFound,
		},
		{
			name: "invalid response json",
			session: &authModel.PasskeySession{
				AccountID:      &account.ID,
				SessionJSON:    json.RawMessage(`{}`),
				ExpectedOrigin: "http://localhost:3000",
			},
			repos: &repositories.Factory{
				Account:           newStubAccountRepository(account),
				PasskeyCredential: &passkeyCredentialRepoStub{},
				PasskeySession:    &passkeySessionRepoStub{},
			},
			wantErr: ErrPasskeySessionInvalid,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sessionRepo := tt.repos.PasskeySession.(*passkeySessionRepoStub)
			sessionRepo.consumed = tt.session
			svc := &passkeyService{repos: tt.repos, rpID: "localhost", rpName: "moto"}
			_, err := svc.FinishRegistration(context.Background(), PasskeyRegistrationFinishRequest{
				AccountID:          account.ID,
				SessionID:          "session-id",
				CredentialResponse: json.RawMessage(`{`),
			})
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				return
			}
			require.Error(t, err)
		})
	}
}

func TestPasskeyFinishLoginRejectsInvalidSessionState(t *testing.T) {
	t.Parallel()

	tenantID := int64(55)

	tests := []struct {
		name    string
		session *authModel.PasskeySession
		wantErr error
	}{
		{
			name:    "consume fails",
			wantErr: ErrPasskeySessionInvalid,
		},
		{
			name: "missing tenant id",
			session: &authModel.PasskeySession{
				SessionJSON:    json.RawMessage(`{}`),
				ExpectedOrigin: "http://localhost:3000",
			},
			wantErr: ErrPasskeySessionInvalid,
		},
		{
			name: "invalid session json",
			session: &authModel.PasskeySession{
				TenantID:       &tenantID,
				SessionJSON:    json.RawMessage(`{`),
				ExpectedOrigin: "http://localhost:3000",
			},
		},
		{
			name: "invalid response json",
			session: &authModel.PasskeySession{
				TenantID:       &tenantID,
				SessionJSON:    json.RawMessage(`{}`),
				ExpectedOrigin: "http://localhost:3000",
			},
			wantErr: ErrPasskeySessionInvalid,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sessionRepo := &passkeySessionRepoStub{consumed: tt.session}
			if tt.session == nil {
				sessionRepo.consumeErr = sql.ErrNoRows
			}
			svc := &passkeyService{
				repos: &repositories.Factory{
					PasskeySession: sessionRepo,
				},
				rpID:   "localhost",
				rpName: "moto",
			}
			_, err := svc.FinishLogin(context.Background(), PasskeyLoginFinishRequest{
				SessionID:          "session-id",
				CredentialResponse: json.RawMessage(`{`),
			})
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				return
			}
			require.Error(t, err)
		})
	}
}

func TestPasskeyCredentialServiceMethods(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()
	account := &authModel.Account{Model: base.Model{ID: 91}, Email: "teacher@example.test"}
	row := &authModel.PasskeyCredential{
		Model:          base.Model{ID: 92, CreatedAt: now},
		AccountID:      account.ID,
		UserHandle:     []byte("user-handle"),
		CredentialJSON: json.RawMessage(`{}`),
		Name:           "Laptop",
	}
	repo := &passkeyCredentialRepoStub{rows: []*authModel.PasskeyCredential{row}}
	svc := &passkeyService{repos: &repositories.Factory{PasskeyCredential: repo}}

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

	repo.rows = []*authModel.PasskeyCredential{{CredentialJSON: json.RawMessage(`{`)}}
	_, err = svc.passkeyUserForAccount(context.Background(), account)
	require.Error(t, err)
}

func TestPasskeyCredentialServiceErrors(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("repo down")
	repo := &passkeyCredentialRepoStub{err: wantErr}
	svc := &passkeyService{repos: &repositories.Factory{PasskeyCredential: repo}}

	_, err := svc.ListCredentials(context.Background(), 1)
	require.ErrorIs(t, err, wantErr)

	err = svc.RevokeCredential(context.Background(), 1, 2)
	require.ErrorIs(t, err, ErrPasskeyNotFound)

	_, err = svc.passkeyUserForAccount(context.Background(), &authModel.Account{Email: "teacher@example.test"})
	require.ErrorIs(t, err, wantErr)
}

type passkeyCredentialRepoStub struct {
	rows                []*authModel.PasskeyCredential
	err                 error
	revokedAccountID    int64
	revokedCredentialID int64
}

func (r *passkeyCredentialRepoStub) Create(context.Context, *authModel.PasskeyCredential) error {
	return r.err
}

func (r *passkeyCredentialRepoStub) FindActiveByAccountID(context.Context, int64) ([]*authModel.PasskeyCredential, error) {
	return r.rows, r.err
}

func (r *passkeyCredentialRepoStub) FindActiveByCredentialIDAndUserHandle(context.Context, []byte, []byte) (*authModel.PasskeyCredential, error) {
	if len(r.rows) == 0 {
		return nil, r.err
	}
	return r.rows[0], r.err
}

func (r *passkeyCredentialRepoStub) UpdateAfterUse(context.Context, int64, []byte, time.Time) error {
	return r.err
}

func (r *passkeyCredentialRepoStub) Revoke(_ context.Context, accountID, id int64, _ time.Time) error {
	r.revokedAccountID = accountID
	r.revokedCredentialID = id
	return r.err
}

type passkeySessionRepoStub struct {
	created    *authModel.PasskeySession
	consumed   *authModel.PasskeySession
	createErr  error
	consumeErr error
}

func (r *passkeySessionRepoStub) Create(_ context.Context, session *authModel.PasskeySession) error {
	r.created = session
	return r.createErr
}

func (r *passkeySessionRepoStub) Consume(_ context.Context, _, _ string, _ time.Time) (*authModel.PasskeySession, error) {
	if r.consumeErr != nil {
		return nil, r.consumeErr
	}
	if r.consumed == nil {
		return nil, sql.ErrNoRows
	}
	return r.consumed, nil
}

func (r *passkeySessionRepoStub) DeleteExpired(context.Context, time.Time) (int, error) {
	return 0, nil
}

// MFAStub is defined in mfa_stub_test.go (shared with auth_login_mfa_infra_test.go).

func TestPasskeyRepositories(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	requirePasskeyTables(t, db, "auth.passkey_credentials", "auth.passkey_sessions")
	scope := testpkg.NewTenantScope(t, db)

	ctx := context.Background()
	repos := repositories.NewFactory(db)
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())

	account := &authModel.Account{
		Email:  fmt.Sprintf("passkey-%s@example.test", suffix),
		Active: true,
	}
	require.NoError(t, repos.Account.Create(ctx, account))
	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM auth.passkey_sessions WHERE account_id = ?`, account.ID)
		_, _ = db.ExecContext(ctx, `DELETE FROM auth.passkey_credentials WHERE account_id = ?`, account.ID)
		_, _ = db.ExecContext(ctx, `DELETE FROM auth.accounts WHERE id = ?`, account.ID)
	})

	credentialID := []byte("credential-" + suffix)
	userHandle := []byte("user-handle-" + suffix)
	credential := &authModel.PasskeyCredential{
		AccountID:      account.ID,
		UserHandle:     userHandle,
		CredentialID:   credentialID,
		CredentialJSON: json.RawMessage(`{"id":"credential"}`),
		Name:           "Laptop",
	}
	require.NoError(t, repos.PasskeyCredential.Create(ctx, credential))
	require.NotZero(t, credential.ID)

	active, err := repos.PasskeyCredential.FindActiveByAccountID(ctx, account.ID)
	require.NoError(t, err)
	require.Len(t, active, 1)
	assert.Equal(t, "Laptop", active[0].Name)

	byHandle, err := repos.PasskeyCredential.FindActiveByCredentialIDAndUserHandle(ctx, credentialID, userHandle)
	require.NoError(t, err)
	assert.Equal(t, credential.ID, byHandle.ID)

	usedAt := time.Now().UTC().Truncate(time.Microsecond)
	require.NoError(t, repos.PasskeyCredential.UpdateAfterUse(ctx, credential.ID, json.RawMessage(`{"id":"updated"}`), usedAt))
	updated, err := repos.PasskeyCredential.FindActiveByCredentialIDAndUserHandle(ctx, credentialID, userHandle)
	require.NoError(t, err)
	require.NotNil(t, updated.LastUsedAt)
	assert.JSONEq(t, `{"id":"updated"}`, string(updated.CredentialJSON))

	require.NoError(t, repos.PasskeyCredential.Revoke(ctx, account.ID, credential.ID, time.Now()))
	active, err = repos.PasskeyCredential.FindActiveByAccountID(ctx, account.ID)
	require.NoError(t, err)
	assert.Empty(t, active)
	require.Error(t, repos.PasskeyCredential.UpdateAfterUse(ctx, credential.ID, json.RawMessage(`{"id":"revoked"}`), time.Now()))

	sessionID := "test-passkey-session-" + suffix
	session := &authModel.PasskeySession{
		ID:             sessionID,
		AccountID:      &account.ID,
		TenantID:       &scope.TenantID,
		Purpose:        authModel.PasskeySessionPurposeRegistration,
		RPID:           "localhost",
		ExpectedOrigin: "http://localhost:3000",
		SessionJSON:    json.RawMessage(`{"challenge":"abc"}`),
		ExpiresAt:      time.Now().Add(time.Hour),
	}
	require.NoError(t, repos.PasskeySession.Create(ctx, session))

	consumed, err := repos.PasskeySession.Consume(ctx, sessionID, authModel.PasskeySessionPurposeRegistration, time.Now())
	require.NoError(t, err)
	require.NotNil(t, consumed.ConsumedAt)
	_, err = repos.PasskeySession.Consume(ctx, sessionID, authModel.PasskeySessionPurposeRegistration, time.Now())
	require.Error(t, err)

	expiredID := "test-passkey-expired-" + suffix
	require.NoError(t, repos.PasskeySession.Create(ctx, &authModel.PasskeySession{
		ID:             expiredID,
		AccountID:      &account.ID,
		TenantID:       &scope.TenantID,
		Purpose:        authModel.PasskeySessionPurposeLogin,
		RPID:           "localhost",
		ExpectedOrigin: "http://localhost:3000",
		SessionJSON:    json.RawMessage(`{"challenge":"expired"}`),
		ExpiresAt:      time.Now().Add(-time.Hour),
	}))
	deleted, err := repos.PasskeySession.DeleteExpired(ctx, time.Now())
	require.NoError(t, err)
	assert.Positive(t, deleted)
}

func requirePasskeyTables(t *testing.T, db *bun.DB, tables ...string) {
	t.Helper()

	ctx := context.Background()
	for _, table := range tables {
		var regclass sql.NullString
		err := db.QueryRowContext(ctx, `SELECT to_regclass(?)`, table).Scan(&regclass)
		require.NoError(t, err)
		if !regclass.Valid {
			t.Skipf("%s is missing; run backend migrations before repository passkey tests", table)
		}
	}
}
