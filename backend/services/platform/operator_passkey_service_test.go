package platform

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/models/base"
	platformModel "github.com/moto-nrw/project-phoenix/models/platform"
	authService "github.com/moto-nrw/project-phoenix/services/auth"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func TestOperatorPasskeyOriginValidation(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		origin  string
		wantErr bool
	}{
		{
			name:   "operator localhost with port accepted",
			origin: "http://operator.localhost:3000",
		},
		{
			name:   "operator https origin accepted",
			origin: "https://operator.localhost",
		},
		{
			name:    "wrong host rejected",
			origin:  "http://school.localhost:3000",
			wantErr: true,
		},
		{
			name:    "invalid origin rejected",
			origin:  "not-a-url",
			wantErr: true,
		},
		{
			name:    "unsupported scheme rejected",
			origin:  "ftp://operator.localhost",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &operatorPasskeyService{operatorOriginHost: "operator.localhost"}

			err := svc.validateOperatorOrigin(tt.origin)

			if tt.wantErr {
				require.ErrorIs(t, err, authService.ErrPasskeyOriginInvalid)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestOperatorPasskeySummaryAndUser(t *testing.T) {
	t.Parallel()
	now := time.Now().UTC()
	lastUsedAt := now.Add(time.Minute)
	row := &platformModel.OperatorPasskeyCredential{
		Model:      base.Model{ID: 55, CreatedAt: now},
		Name:       "Admin laptop",
		LastUsedAt: &lastUsedAt,
	}
	summary := summarizeOperatorPasskeyCredential(row)

	require.NotNil(t, summary)
	assert.Equal(t, "55", summary.ID)
	assert.Equal(t, "Admin laptop", summary.Name)
	assert.Equal(t, now, summary.CreatedAt)
	assert.Equal(t, &lastUsedAt, summary.LastUsedAt)

	user := &authService.WebAuthnUser{
		UserHandle:  []byte("handle"),
		Name:        "operator@example.test",
		DisplayName: "Operator",
	}
	assert.Equal(t, []byte("handle"), user.WebAuthnID())
	assert.Equal(t, "operator@example.test", user.WebAuthnName())
	assert.Equal(t, "Operator", user.WebAuthnDisplayName())
	assert.Empty(t, user.WebAuthnCredentials())
}

func TestNewOperatorPasskeyServiceValidation(t *testing.T) {
	t.Parallel()
	baseCfg := OperatorPasskeyServiceConfig{
		Repos:               &repositories.Factory{},
		MFAService:          &operatorPasskeyMFAServiceStub{},
		AuthService:         &operatorAuthService{},
		DB:                  &bun.DB{},
		RPID:                "operator.localhost:3000",
		RPName:              "moto tests",
		OperatorFrontendURL: "http://operator.localhost:3000",
	}

	tests := []struct {
		name   string
		mutate func(*OperatorPasskeyServiceConfig)
	}{
		{name: "missing repos", mutate: func(cfg *OperatorPasskeyServiceConfig) { cfg.Repos = nil }},
		{name: "missing mfa", mutate: func(cfg *OperatorPasskeyServiceConfig) { cfg.MFAService = nil }},
		{name: "missing auth", mutate: func(cfg *OperatorPasskeyServiceConfig) { cfg.AuthService = nil }},
		{name: "missing db", mutate: func(cfg *OperatorPasskeyServiceConfig) { cfg.DB = nil }},
		{name: "missing rp id", mutate: func(cfg *OperatorPasskeyServiceConfig) { cfg.RPID = " " }},
		{name: "invalid operator frontend url", mutate: func(cfg *OperatorPasskeyServiceConfig) { cfg.OperatorFrontendURL = "not-a-url" }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := baseCfg
			tt.mutate(&cfg)
			_, err := NewOperatorPasskeyService(cfg)
			require.Error(t, err)
		})
	}

	svc, err := NewOperatorPasskeyService(baseCfg)
	require.NoError(t, err)
	concrete := svc.(*operatorPasskeyService)
	assert.Equal(t, "operator.localhost", concrete.rpID)
	assert.Equal(t, "operator.localhost", concrete.operatorOriginHost)

	cfgWithDefaultName := baseCfg
	cfgWithDefaultName.RPName = " "
	svc, err = NewOperatorPasskeyService(cfgWithDefaultName)
	require.NoError(t, err)
	assert.Equal(t, "moto", svc.(*operatorPasskeyService).rpName)
}

func TestOperatorPasskeyEnrollmentChallenge(t *testing.T) {
	t.Parallel()
	operator := &platformModel.Operator{Model: base.Model{ID: 101}, Email: "operator@example.test", Active: true}
	mfa := &operatorPasskeyMFAServiceStub{challengeToken: "operator-challenge"}
	svc := &operatorPasskeyService{
		repos:      &repositories.Factory{Operator: &operatorPasskeyOperatorRepoStub{operator: operator}},
		mfaService: mfa,
	}

	challenge, err := svc.StartEnrollmentChallenge(context.Background(), operator.ID, net.ParseIP("203.0.113.9"))
	require.NoError(t, err)
	assert.Equal(t, "operator-challenge", challenge.ChallengeToken)
	assert.Contains(t, challenge.MaskedEmail, "@")
	assert.Equal(t, operator.ID, mfa.startedOperatorID)
}

func TestOperatorPasskeyEnrollmentChallengeErrors(t *testing.T) {
	t.Parallel()
	operator := &platformModel.Operator{Model: base.Model{ID: 102}, Email: "operator@example.test", Active: true}
	wantErr := errors.New("mfa down")

	tests := []struct {
		name  string
		repos *repositories.Factory
		mfa   *operatorPasskeyMFAServiceStub
	}{
		{
			name:  "unknown operator",
			repos: &repositories.Factory{Operator: &operatorPasskeyOperatorRepoStub{err: sql.ErrNoRows}},
			mfa:   &operatorPasskeyMFAServiceStub{},
		},
		{
			name:  "mfa start fails",
			repos: &repositories.Factory{Operator: &operatorPasskeyOperatorRepoStub{operator: operator}},
			mfa:   &operatorPasskeyMFAServiceStub{startErr: wantErr},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &operatorPasskeyService{repos: tt.repos, mfaService: tt.mfa}
			_, err := svc.StartEnrollmentChallenge(context.Background(), operator.ID, net.ParseIP("203.0.113.9"))
			require.Error(t, err)
		})
	}
}

func TestOperatorPasskeyBeginRegistrationStoresSession(t *testing.T) {
	t.Parallel()
	operator := &platformModel.Operator{Model: base.Model{ID: 111}, Email: "operator@example.test", DisplayName: "Operator", Active: true}
	sessions := &operatorPasskeySessionRepoStub{}
	mfa := &operatorPasskeyMFAServiceStub{}
	svc := &operatorPasskeyService{
		repos: &repositories.Factory{
			Operator:                  &operatorPasskeyOperatorRepoStub{operator: operator},
			OperatorPasskeyCredential: &operatorPasskeyCredentialRepoStub{},
			OperatorPasskeySession:    sessions,
		},
		mfaService:         mfa,
		rpID:               "localhost",
		rpName:             "moto",
		operatorOriginHost: "operator.localhost",
	}

	creation, err := svc.BeginRegistration(context.Background(), OperatorPasskeyRegistrationStartRequest{
		OperatorID:     operator.ID,
		ExpectedOrigin: "http://operator.localhost:3000",
		Code:           "123456",
		Name:           "Admin laptop",
	})
	require.NoError(t, err)
	require.NotEmpty(t, creation.SessionID)
	require.NotNil(t, creation.Options)
	require.NotNil(t, sessions.created)
	require.NotNil(t, sessions.created.OperatorID)
	assert.Equal(t, operator.ID, *sessions.created.OperatorID)
	assert.Equal(t, platformModel.OperatorPasskeySessionPurposeRegistration, sessions.created.Purpose)
	assert.Equal(t, "operator.localhost", sessions.created.RPID)
	assert.Equal(t, "http://operator.localhost:3000", sessions.created.ExpectedOrigin)
	assert.False(t, sessions.created.ExpiresAt.IsZero())
	assert.True(t, json.Valid(sessions.created.SessionJSON))
	assert.Equal(t, operator.ID, mfa.verifiedOperatorID)
	assert.Equal(t, "123456", mfa.verifiedCode)
}

func TestOperatorPasskeyBeginRegistrationErrors(t *testing.T) {
	t.Parallel()
	operator := &platformModel.Operator{Model: base.Model{ID: 112}, Email: "operator@example.test", DisplayName: "Operator", Active: true}
	inactive := &platformModel.Operator{Model: base.Model{ID: 113}, Email: "inactive@example.test", DisplayName: "Inactive", Active: false}
	wantErr := errors.New("boom")

	tests := []struct {
		name    string
		req     OperatorPasskeyRegistrationStartRequest
		repos   *repositories.Factory
		mfa     *operatorPasskeyMFAServiceStub
		wantErr error
	}{
		{
			name: "invalid origin short circuits before mfa",
			req: OperatorPasskeyRegistrationStartRequest{
				OperatorID:     operator.ID,
				ExpectedOrigin: "http://school.localhost:3000",
			},
			repos:   &repositories.Factory{Operator: &operatorPasskeyOperatorRepoStub{operator: operator}},
			mfa:     &operatorPasskeyMFAServiceStub{},
			wantErr: authService.ErrPasskeyOriginInvalid,
		},
		{
			name: "mfa verify fails",
			req: OperatorPasskeyRegistrationStartRequest{
				OperatorID:     operator.ID,
				ExpectedOrigin: "http://operator.localhost:3000",
				Code:           "000000",
			},
			repos: &repositories.Factory{Operator: &operatorPasskeyOperatorRepoStub{operator: operator}},
			mfa:   &operatorPasskeyMFAServiceStub{verifyErr: wantErr},
		},
		{
			name: "unknown operator",
			req: OperatorPasskeyRegistrationStartRequest{
				OperatorID:     999,
				ExpectedOrigin: "http://operator.localhost:3000",
			},
			repos: &repositories.Factory{Operator: &operatorPasskeyOperatorRepoStub{err: sql.ErrNoRows}},
			mfa:   &operatorPasskeyMFAServiceStub{},
		},
		{
			name: "inactive operator",
			req: OperatorPasskeyRegistrationStartRequest{
				OperatorID:     inactive.ID,
				ExpectedOrigin: "http://operator.localhost:3000",
			},
			repos: &repositories.Factory{Operator: &operatorPasskeyOperatorRepoStub{operator: inactive}},
			mfa:   &operatorPasskeyMFAServiceStub{},
		},
		{
			name: "credential lookup fails while building user",
			req: OperatorPasskeyRegistrationStartRequest{
				OperatorID:     operator.ID,
				ExpectedOrigin: "http://operator.localhost:3000",
			},
			repos: &repositories.Factory{
				Operator:                  &operatorPasskeyOperatorRepoStub{operator: operator},
				OperatorPasskeyCredential: &operatorPasskeyCredentialRepoStub{err: wantErr},
			},
			mfa: &operatorPasskeyMFAServiceStub{},
		},
		{
			name: "session create fails",
			req: OperatorPasskeyRegistrationStartRequest{
				OperatorID:     operator.ID,
				ExpectedOrigin: "http://operator.localhost:3000",
			},
			repos: &repositories.Factory{
				Operator:                  &operatorPasskeyOperatorRepoStub{operator: operator},
				OperatorPasskeyCredential: &operatorPasskeyCredentialRepoStub{},
				OperatorPasskeySession:    &operatorPasskeySessionRepoStub{createErr: wantErr},
			},
			mfa: &operatorPasskeyMFAServiceStub{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &operatorPasskeyService{
				repos:              tt.repos,
				mfaService:         tt.mfa,
				rpID:               "operator.localhost",
				rpName:             "moto",
				operatorOriginHost: "operator.localhost",
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

func TestOperatorPasskeyBeginLoginStoresSession(t *testing.T) {
	t.Parallel()
	sessions := &operatorPasskeySessionRepoStub{}
	svc := &operatorPasskeyService{
		repos:              &repositories.Factory{OperatorPasskeySession: sessions},
		rpID:               "localhost",
		rpName:             "moto",
		operatorOriginHost: "operator.localhost",
	}

	assertion, err := svc.BeginLogin(context.Background(), "http://operator.localhost:3000")
	require.NoError(t, err)
	require.NotEmpty(t, assertion.SessionID)
	require.NotNil(t, assertion.Options)
	require.NotNil(t, sessions.created)
	assert.Equal(t, platformModel.OperatorPasskeySessionPurposeLogin, sessions.created.Purpose)
	assert.Equal(t, "operator.localhost", sessions.created.RPID)
	assert.Equal(t, "http://operator.localhost:3000", sessions.created.ExpectedOrigin)
	assert.False(t, sessions.created.ExpiresAt.IsZero())
}

func TestOperatorPasskeyBeginLoginErrors(t *testing.T) {
	t.Parallel()
	wantErr := errors.New("session down")

	tests := []struct {
		name    string
		origin  string
		session *operatorPasskeySessionRepoStub
		wantErr error
	}{
		{
			name:    "invalid origin",
			origin:  "http://school.localhost:3000",
			session: &operatorPasskeySessionRepoStub{},
			wantErr: authService.ErrPasskeyOriginInvalid,
		},
		{
			name:    "session create fails",
			origin:  "http://operator.localhost:3000",
			session: &operatorPasskeySessionRepoStub{createErr: wantErr},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &operatorPasskeyService{
				repos:              &repositories.Factory{OperatorPasskeySession: tt.session},
				rpID:               "operator.localhost",
				rpName:             "moto",
				operatorOriginHost: "operator.localhost",
			}
			_, err := svc.BeginLogin(context.Background(), tt.origin)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				return
			}
			require.Error(t, err)
		})
	}
}

func TestOperatorPasskeyFinishRegistrationRejectsInvalidSessionState(t *testing.T) {
	t.Parallel()
	operator := &platformModel.Operator{Model: base.Model{ID: 121}, Email: "operator@example.test", DisplayName: "Operator", Active: true}
	otherOperatorID := operator.ID + 1

	tests := []struct {
		name    string
		session *platformModel.OperatorPasskeySession
		repos   *repositories.Factory
		wantErr error
	}{
		{
			name: "consume fails",
			repos: &repositories.Factory{
				OperatorPasskeySession: &operatorPasskeySessionRepoStub{consumeErr: sql.ErrNoRows},
			},
			wantErr: authService.ErrPasskeySessionInvalid,
		},
		{
			name: "missing operator id",
			session: &platformModel.OperatorPasskeySession{
				SessionJSON:    json.RawMessage(`{}`),
				ExpectedOrigin: "http://operator.localhost:3000",
			},
			repos:   &repositories.Factory{OperatorPasskeySession: &operatorPasskeySessionRepoStub{}},
			wantErr: authService.ErrPasskeySessionInvalid,
		},
		{
			name: "wrong operator id",
			session: &platformModel.OperatorPasskeySession{
				OperatorID:     &otherOperatorID,
				SessionJSON:    json.RawMessage(`{}`),
				ExpectedOrigin: "http://operator.localhost:3000",
			},
			repos:   &repositories.Factory{OperatorPasskeySession: &operatorPasskeySessionRepoStub{}},
			wantErr: authService.ErrPasskeySessionInvalid,
		},
		{
			name: "invalid session json",
			session: &platformModel.OperatorPasskeySession{
				OperatorID:     &operator.ID,
				SessionJSON:    json.RawMessage(`{`),
				ExpectedOrigin: "http://operator.localhost:3000",
			},
			repos: &repositories.Factory{OperatorPasskeySession: &operatorPasskeySessionRepoStub{}},
		},
		{
			name: "unknown operator",
			session: &platformModel.OperatorPasskeySession{
				OperatorID:     &operator.ID,
				SessionJSON:    json.RawMessage(`{}`),
				ExpectedOrigin: "http://operator.localhost:3000",
			},
			repos: &repositories.Factory{
				Operator:               &operatorPasskeyOperatorRepoStub{err: sql.ErrNoRows},
				OperatorPasskeySession: &operatorPasskeySessionRepoStub{},
			},
		},
		{
			name: "invalid response json",
			session: &platformModel.OperatorPasskeySession{
				OperatorID:     &operator.ID,
				SessionJSON:    json.RawMessage(`{}`),
				ExpectedOrigin: "http://operator.localhost:3000",
			},
			repos: &repositories.Factory{
				Operator:                  &operatorPasskeyOperatorRepoStub{operator: operator},
				OperatorPasskeyCredential: &operatorPasskeyCredentialRepoStub{},
				OperatorPasskeySession:    &operatorPasskeySessionRepoStub{},
			},
			wantErr: authService.ErrPasskeySessionInvalid,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sessionRepo := tt.repos.OperatorPasskeySession.(*operatorPasskeySessionRepoStub)
			sessionRepo.consumed = tt.session
			svc := &operatorPasskeyService{
				repos:              tt.repos,
				rpID:               "operator.localhost",
				rpName:             "moto",
				operatorOriginHost: "operator.localhost",
			}
			_, err := svc.FinishRegistration(context.Background(), OperatorPasskeyRegistrationFinishRequest{
				OperatorID:         operator.ID,
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

func TestOperatorPasskeyFinishLoginRejectsInvalidSessionState(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		session *platformModel.OperatorPasskeySession
		wantErr error
	}{
		{
			name:    "consume fails",
			wantErr: authService.ErrPasskeySessionInvalid,
		},
		{
			name: "invalid session json",
			session: &platformModel.OperatorPasskeySession{
				SessionJSON:    json.RawMessage(`{`),
				ExpectedOrigin: "http://operator.localhost:3000",
			},
		},
		{
			name: "invalid response json",
			session: &platformModel.OperatorPasskeySession{
				SessionJSON:    json.RawMessage(`{}`),
				ExpectedOrigin: "http://operator.localhost:3000",
			},
			wantErr: authService.ErrPasskeySessionInvalid,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sessionRepo := &operatorPasskeySessionRepoStub{consumed: tt.session}
			if tt.session == nil {
				sessionRepo.consumeErr = sql.ErrNoRows
			}
			svc := &operatorPasskeyService{
				repos:              &repositories.Factory{OperatorPasskeySession: sessionRepo},
				rpID:               "operator.localhost",
				rpName:             "moto",
				operatorOriginHost: "operator.localhost",
			}
			_, err := svc.FinishLogin(context.Background(), OperatorPasskeyLoginFinishRequest{
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

func TestOperatorPasskeyCredentialServiceMethods(t *testing.T) {
	t.Parallel()
	now := time.Now().UTC()
	operator := &platformModel.Operator{Model: base.Model{ID: 81}, Email: "operator@example.test", DisplayName: "Operator"}
	row := &platformModel.OperatorPasskeyCredential{
		Model:          base.Model{ID: 82, CreatedAt: now},
		OperatorID:     operator.ID,
		UserHandle:     []byte("operator-handle"),
		CredentialJSON: json.RawMessage(`{}`),
		Name:           "Admin laptop",
	}
	repo := &operatorPasskeyCredentialRepoStub{rows: []*platformModel.OperatorPasskeyCredential{row}}
	svc := &operatorPasskeyService{repos: &repositories.Factory{OperatorPasskeyCredential: repo}}

	list, err := svc.ListCredentials(context.Background(), operator.ID)
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, "82", list[0].ID)

	require.NoError(t, svc.RevokeCredential(context.Background(), operator.ID, row.ID))
	assert.Equal(t, operator.ID, repo.revokedOperatorID)
	assert.Equal(t, row.ID, repo.revokedCredentialID)

	user, err := svc.passkeyUserForOperator(context.Background(), operator)
	require.NoError(t, err)
	assert.Equal(t, []byte("operator-handle"), user.WebAuthnID())
	assert.Equal(t, operator.Email, user.WebAuthnName())
	assert.Equal(t, operator.DisplayName, user.WebAuthnDisplayName())
	assert.Len(t, user.WebAuthnCredentials(), 1)

	repo.rows = nil
	user, err = svc.passkeyUserForOperator(context.Background(), operator)
	require.NoError(t, err)
	assert.Len(t, user.WebAuthnID(), authService.PasskeyUserHandleBytes)

	user, err = svc.passkeyUserForOperator(context.Background(), operator, []byte("session-handle"))
	require.NoError(t, err)
	assert.Equal(t, []byte("session-handle"), user.WebAuthnID())

	repo.rows = []*platformModel.OperatorPasskeyCredential{{CredentialJSON: json.RawMessage(`{`)}}
	_, err = svc.passkeyUserForOperator(context.Background(), operator)
	require.Error(t, err)
}

func TestOperatorPasskeyCredentialServiceErrors(t *testing.T) {
	t.Parallel()
	wantErr := errors.New("repo down")
	repo := &operatorPasskeyCredentialRepoStub{err: wantErr}
	svc := &operatorPasskeyService{repos: &repositories.Factory{OperatorPasskeyCredential: repo}}

	_, err := svc.ListCredentials(context.Background(), 1)
	require.ErrorIs(t, err, wantErr)

	err = svc.RevokeCredential(context.Background(), 1, 2)
	require.ErrorIs(t, err, authService.ErrPasskeyNotFound)

	_, err = svc.passkeyUserForOperator(context.Background(), &platformModel.Operator{Email: "operator@example.test"})
	require.ErrorIs(t, err, wantErr)
}

type operatorPasskeyCredentialRepoStub struct {
	rows                []*platformModel.OperatorPasskeyCredential
	err                 error
	revokedOperatorID   int64
	revokedCredentialID int64
}

func (r *operatorPasskeyCredentialRepoStub) Create(context.Context, *platformModel.OperatorPasskeyCredential) error {
	return r.err
}

func (r *operatorPasskeyCredentialRepoStub) FindActiveByOperatorID(context.Context, int64) ([]*platformModel.OperatorPasskeyCredential, error) {
	return r.rows, r.err
}

func (r *operatorPasskeyCredentialRepoStub) FindActiveByCredentialIDAndUserHandle(context.Context, []byte, []byte) (*platformModel.OperatorPasskeyCredential, error) {
	if len(r.rows) == 0 {
		return nil, r.err
	}
	return r.rows[0], r.err
}

func (r *operatorPasskeyCredentialRepoStub) UpdateAfterUse(context.Context, int64, []byte, time.Time) error {
	return r.err
}

func (r *operatorPasskeyCredentialRepoStub) Revoke(_ context.Context, operatorID, id int64, _ time.Time) error {
	r.revokedOperatorID = operatorID
	r.revokedCredentialID = id
	return r.err
}

type operatorPasskeySessionRepoStub struct {
	created    *platformModel.OperatorPasskeySession
	consumed   *platformModel.OperatorPasskeySession
	createErr  error
	consumeErr error
}

func (r *operatorPasskeySessionRepoStub) Create(_ context.Context, session *platformModel.OperatorPasskeySession) error {
	r.created = session
	return r.createErr
}

func (r *operatorPasskeySessionRepoStub) Consume(_ context.Context, _, _ string, _ time.Time) (*platformModel.OperatorPasskeySession, error) {
	if r.consumeErr != nil {
		return nil, r.consumeErr
	}
	if r.consumed == nil {
		return nil, sql.ErrNoRows
	}
	return r.consumed, nil
}

func (r *operatorPasskeySessionRepoStub) DeleteExpired(context.Context, time.Time) (int, error) {
	return 0, nil
}

type operatorPasskeyOperatorRepoStub struct {
	operator *platformModel.Operator
	err      error
}

func (r *operatorPasskeyOperatorRepoStub) Create(context.Context, *platformModel.Operator) error {
	return nil
}

func (r *operatorPasskeyOperatorRepoStub) FindByID(context.Context, int64) (*platformModel.Operator, error) {
	if r.err != nil {
		return nil, r.err
	}
	if r.operator == nil {
		return nil, sql.ErrNoRows
	}
	return r.operator, nil
}

func (r *operatorPasskeyOperatorRepoStub) FindByIDForUpdate(ctx context.Context, id int64) (*platformModel.Operator, error) {
	return r.FindByID(ctx, id)
}

func (r *operatorPasskeyOperatorRepoStub) FindByEmail(context.Context, string) (*platformModel.Operator, error) {
	return nil, sql.ErrNoRows
}

func (r *operatorPasskeyOperatorRepoStub) Update(context.Context, *platformModel.Operator) error {
	return nil
}

func (r *operatorPasskeyOperatorRepoStub) Delete(context.Context, int64) error {
	return nil
}

func (r *operatorPasskeyOperatorRepoStub) List(context.Context) ([]*platformModel.Operator, error) {
	return nil, nil
}

func (r *operatorPasskeyOperatorRepoStub) UpdateLastLogin(context.Context, int64) error {
	return nil
}

func (r *operatorPasskeyOperatorRepoStub) IncrementMFAAttempts(context.Context, int64, int, time.Duration) (platformModel.OperatorMFAAttemptResult, error) {
	return platformModel.OperatorMFAAttemptResult{}, nil
}

func (r *operatorPasskeyOperatorRepoStub) ResetMFAAttempts(context.Context, int64) error {
	return nil
}

type operatorPasskeyMFAServiceStub struct {
	challengeToken string
	startErr       error
	verifyErr      error

	startedOperatorID  int64
	verifiedOperatorID int64
	verifiedCode       string
}

func (s *operatorPasskeyMFAServiceStub) HasEnrollment(context.Context, int64) (bool, error) {
	return false, nil
}

func (s *operatorPasskeyMFAServiceStub) StartChallenge(_ context.Context, operatorID int64, _ net.IP) (string, error) {
	s.startedOperatorID = operatorID
	if s.startErr != nil {
		return "", s.startErr
	}
	if s.challengeToken != "" {
		return s.challengeToken, nil
	}
	return "operator-challenge-token", nil
}

func (s *operatorPasskeyMFAServiceStub) VerifyChallenge(context.Context, string, string) (*OperatorVerifiedChallenge, error) {
	return nil, nil
}

func (s *operatorPasskeyMFAServiceStub) ResendChallenge(context.Context, string, net.IP) (string, error) {
	return "operator-challenge-token", nil
}

func (s *operatorPasskeyMFAServiceStub) VerifyCodeForOperator(_ context.Context, operatorID int64, code string) error {
	s.verifiedOperatorID = operatorID
	s.verifiedCode = code
	return s.verifyErr
}

func (s *operatorPasskeyMFAServiceStub) Enroll(context.Context, int64) error {
	return nil
}

func (s *operatorPasskeyMFAServiceStub) Disable(context.Context, int64) error {
	return nil
}

func (s *operatorPasskeyMFAServiceStub) IssueTrustedDevice(context.Context, int64, string, net.IP) (string, time.Time, error) {
	return "", time.Time{}, nil
}

func (s *operatorPasskeyMFAServiceStub) VerifyTrustedDevice(context.Context, int64, string) (bool, error) {
	return false, nil
}

func (s *operatorPasskeyMFAServiceStub) ListTrustedDevices(context.Context, int64) ([]*platformModel.OperatorMFATrustedDevice, error) {
	return nil, nil
}

func (s *operatorPasskeyMFAServiceStub) RevokeTrustedDevice(context.Context, int64, int64) error {
	return nil
}

func TestOperatorPasskeyRepositories(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	requireOperatorPasskeyTables(t, db, "platform.operator_passkey_credentials", "platform.operator_passkey_sessions")

	ctx := context.Background()
	repos := repositories.NewFactory(db)
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())

	operator := &platformModel.Operator{
		Email:        fmt.Sprintf("operator-passkey-%s@example.test", suffix),
		DisplayName:  "Passkey Operator",
		PasswordHash: "hashed-password",
		Active:       true,
	}
	require.NoError(t, repos.Operator.Create(ctx, operator))
	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM platform.operator_passkey_sessions WHERE operator_id = ?`, operator.ID)
		_, _ = db.ExecContext(ctx, `DELETE FROM platform.operator_passkey_credentials WHERE operator_id = ?`, operator.ID)
		_, _ = db.ExecContext(ctx, `DELETE FROM platform.operators WHERE id = ?`, operator.ID)
	})

	credentialID := []byte("operator-credential-" + suffix)
	userHandle := []byte("operator-user-handle-" + suffix)
	credential := &platformModel.OperatorPasskeyCredential{
		OperatorID:     operator.ID,
		UserHandle:     userHandle,
		CredentialID:   credentialID,
		CredentialJSON: json.RawMessage(`{"id":"operator-credential"}`),
		Name:           "Admin laptop",
	}
	require.NoError(t, repos.OperatorPasskeyCredential.Create(ctx, credential))
	require.NotZero(t, credential.ID)

	active, err := repos.OperatorPasskeyCredential.FindActiveByOperatorID(ctx, operator.ID)
	require.NoError(t, err)
	require.Len(t, active, 1)
	assert.Equal(t, "Admin laptop", active[0].Name)

	byHandle, err := repos.OperatorPasskeyCredential.FindActiveByCredentialIDAndUserHandle(ctx, credentialID, userHandle)
	require.NoError(t, err)
	assert.Equal(t, credential.ID, byHandle.ID)

	usedAt := time.Now().UTC().Truncate(time.Microsecond)
	require.NoError(t, repos.OperatorPasskeyCredential.UpdateAfterUse(ctx, credential.ID, json.RawMessage(`{"id":"operator-updated"}`), usedAt))
	updated, err := repos.OperatorPasskeyCredential.FindActiveByCredentialIDAndUserHandle(ctx, credentialID, userHandle)
	require.NoError(t, err)
	require.NotNil(t, updated.LastUsedAt)
	assert.JSONEq(t, `{"id":"operator-updated"}`, string(updated.CredentialJSON))

	require.NoError(t, repos.OperatorPasskeyCredential.Revoke(ctx, operator.ID, credential.ID, time.Now()))
	active, err = repos.OperatorPasskeyCredential.FindActiveByOperatorID(ctx, operator.ID)
	require.NoError(t, err)
	assert.Empty(t, active)
	require.Error(t, repos.OperatorPasskeyCredential.UpdateAfterUse(ctx, credential.ID, json.RawMessage(`{"id":"operator-revoked"}`), time.Now()))

	sessionID := "test-operator-passkey-session-" + suffix
	session := &platformModel.OperatorPasskeySession{
		ID:             sessionID,
		OperatorID:     &operator.ID,
		Purpose:        platformModel.OperatorPasskeySessionPurposeRegistration,
		RPID:           "operator.localhost",
		ExpectedOrigin: "http://operator.localhost:3000",
		SessionJSON:    json.RawMessage(`{"challenge":"abc"}`),
		ExpiresAt:      time.Now().Add(time.Hour),
	}
	require.NoError(t, repos.OperatorPasskeySession.Create(ctx, session))

	consumed, err := repos.OperatorPasskeySession.Consume(ctx, sessionID, platformModel.OperatorPasskeySessionPurposeRegistration, time.Now())
	require.NoError(t, err)
	require.NotNil(t, consumed.ConsumedAt)
	_, err = repos.OperatorPasskeySession.Consume(ctx, sessionID, platformModel.OperatorPasskeySessionPurposeRegistration, time.Now())
	require.Error(t, err)

	expiredID := "test-operator-passkey-expired-" + suffix
	require.NoError(t, repos.OperatorPasskeySession.Create(ctx, &platformModel.OperatorPasskeySession{
		ID:             expiredID,
		OperatorID:     &operator.ID,
		Purpose:        platformModel.OperatorPasskeySessionPurposeLogin,
		RPID:           "operator.localhost",
		ExpectedOrigin: "http://operator.localhost:3000",
		SessionJSON:    json.RawMessage(`{"challenge":"expired"}`),
		ExpiresAt:      time.Now().Add(-time.Hour),
	}))
	deleted, err := repos.OperatorPasskeySession.DeleteExpired(ctx, time.Now())
	require.NoError(t, err)
	assert.Positive(t, deleted)
}

func requireOperatorPasskeyTables(t *testing.T, db *bun.DB, tables ...string) {
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
