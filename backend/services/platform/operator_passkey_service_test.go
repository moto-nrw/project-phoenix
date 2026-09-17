package platform

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
	platformModel "github.com/moto-nrw/project-phoenix/models/platform"
	authService "github.com/moto-nrw/project-phoenix/services/auth"
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
		Records:             &operatorPasskeyRecordsStub{},
		Operators:           &operatorPasskeyOperatorRepoStub{},
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
		{name: "missing records", mutate: func(cfg *OperatorPasskeyServiceConfig) { cfg.Records = nil }},
		{name: "missing operators", mutate: func(cfg *OperatorPasskeyServiceConfig) { cfg.Operators = nil }},
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
		operators:  &operatorPasskeyOperatorRepoStub{operator: operator},
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
		name      string
		operators OperatorDirectory
		mfa       *operatorPasskeyMFAServiceStub
	}{
		{
			name:      "unknown operator",
			operators: &operatorPasskeyOperatorRepoStub{err: sql.ErrNoRows},
			mfa:       &operatorPasskeyMFAServiceStub{},
		},
		{
			name:      "mfa start fails",
			operators: &operatorPasskeyOperatorRepoStub{operator: operator},
			mfa:       &operatorPasskeyMFAServiceStub{startErr: wantErr},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &operatorPasskeyService{operators: tt.operators, mfaService: tt.mfa}
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
		records: &operatorPasskeyRecordsStub{
			credentials: &operatorPasskeyCredentialRepoStub{},
			sessions:    sessions,
		},
		operators:          &operatorPasskeyOperatorRepoStub{operator: operator},
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
		name      string
		req       OperatorPasskeyRegistrationStartRequest
		operators OperatorDirectory
		records   *operatorPasskeyRecordsStub
		mfa       *operatorPasskeyMFAServiceStub
		wantErr   error
	}{
		{
			name: "invalid origin short circuits before mfa",
			req: OperatorPasskeyRegistrationStartRequest{
				OperatorID:     operator.ID,
				ExpectedOrigin: "http://school.localhost:3000",
			},
			operators: &operatorPasskeyOperatorRepoStub{operator: operator},
			mfa:       &operatorPasskeyMFAServiceStub{},
			wantErr:   authService.ErrPasskeyOriginInvalid,
		},
		{
			name: "mfa verify fails",
			req: OperatorPasskeyRegistrationStartRequest{
				OperatorID:     operator.ID,
				ExpectedOrigin: "http://operator.localhost:3000",
				Code:           "000000",
			},
			operators: &operatorPasskeyOperatorRepoStub{operator: operator},
			mfa:       &operatorPasskeyMFAServiceStub{verifyErr: wantErr},
		},
		{
			name: "unknown operator",
			req: OperatorPasskeyRegistrationStartRequest{
				OperatorID:     999,
				ExpectedOrigin: "http://operator.localhost:3000",
			},
			operators: &operatorPasskeyOperatorRepoStub{err: sql.ErrNoRows},
			mfa:       &operatorPasskeyMFAServiceStub{},
		},
		{
			name: "inactive operator",
			req: OperatorPasskeyRegistrationStartRequest{
				OperatorID:     inactive.ID,
				ExpectedOrigin: "http://operator.localhost:3000",
			},
			operators: &operatorPasskeyOperatorRepoStub{operator: inactive},
			mfa:       &operatorPasskeyMFAServiceStub{},
		},
		{
			name: "credential lookup fails while building user",
			req: OperatorPasskeyRegistrationStartRequest{
				OperatorID:     operator.ID,
				ExpectedOrigin: "http://operator.localhost:3000",
			},
			operators: &operatorPasskeyOperatorRepoStub{operator: operator},
			records: &operatorPasskeyRecordsStub{
				credentials: &operatorPasskeyCredentialRepoStub{err: wantErr},
			},
			mfa: &operatorPasskeyMFAServiceStub{},
		},
		{
			name: "session create fails",
			req: OperatorPasskeyRegistrationStartRequest{
				OperatorID:     operator.ID,
				ExpectedOrigin: "http://operator.localhost:3000",
			},
			operators: &operatorPasskeyOperatorRepoStub{operator: operator},
			records: &operatorPasskeyRecordsStub{
				credentials: &operatorPasskeyCredentialRepoStub{},
				sessions:    &operatorPasskeySessionRepoStub{createErr: wantErr},
			},
			mfa: &operatorPasskeyMFAServiceStub{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &operatorPasskeyService{
				records:            tt.records,
				operators:          tt.operators,
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
		records:            &operatorPasskeyRecordsStub{sessions: sessions},
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
				records:            &operatorPasskeyRecordsStub{sessions: tt.session},
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
		name      string
		session   *platformModel.OperatorPasskeySession
		operators OperatorDirectory
		records   *operatorPasskeyRecordsStub
		wantErr   error
	}{
		{
			name:    "no pending ceremony",
			records: &operatorPasskeyRecordsStub{sessions: &operatorPasskeySessionRepoStub{}},
			wantErr: authService.ErrPasskeySessionInvalid,
		},
		{
			name: "consume store failure is not an invalid session",
			records: &operatorPasskeyRecordsStub{
				sessions: &operatorPasskeySessionRepoStub{consumeErr: errOperatorPasskeyStore},
			},
			wantErr: errOperatorPasskeyStore,
		},
		{
			name: "missing operator id",
			session: &platformModel.OperatorPasskeySession{
				SessionJSON:    json.RawMessage(`{}`),
				ExpectedOrigin: "http://operator.localhost:3000",
			},
			records: &operatorPasskeyRecordsStub{sessions: &operatorPasskeySessionRepoStub{}},
			wantErr: authService.ErrPasskeySessionInvalid,
		},
		{
			name: "wrong operator id",
			session: &platformModel.OperatorPasskeySession{
				OperatorID:     &otherOperatorID,
				SessionJSON:    json.RawMessage(`{}`),
				ExpectedOrigin: "http://operator.localhost:3000",
			},
			records: &operatorPasskeyRecordsStub{sessions: &operatorPasskeySessionRepoStub{}},
			wantErr: authService.ErrPasskeySessionInvalid,
		},
		{
			name: "invalid session json",
			session: &platformModel.OperatorPasskeySession{
				OperatorID:     &operator.ID,
				SessionJSON:    json.RawMessage(`{`),
				ExpectedOrigin: "http://operator.localhost:3000",
			},
			records: &operatorPasskeyRecordsStub{sessions: &operatorPasskeySessionRepoStub{}},
		},
		{
			name: "unknown operator",
			session: &platformModel.OperatorPasskeySession{
				OperatorID:     &operator.ID,
				SessionJSON:    json.RawMessage(`{}`),
				ExpectedOrigin: "http://operator.localhost:3000",
			},
			operators: &operatorPasskeyOperatorRepoStub{err: sql.ErrNoRows},
			records: &operatorPasskeyRecordsStub{
				sessions: &operatorPasskeySessionRepoStub{},
			},
		},
		{
			name: "invalid response json",
			session: &platformModel.OperatorPasskeySession{
				OperatorID:     &operator.ID,
				SessionJSON:    json.RawMessage(`{}`),
				ExpectedOrigin: "http://operator.localhost:3000",
			},
			operators: &operatorPasskeyOperatorRepoStub{operator: operator},
			records: &operatorPasskeyRecordsStub{
				credentials: &operatorPasskeyCredentialRepoStub{},
				sessions:    &operatorPasskeySessionRepoStub{},
			},
			wantErr: authService.ErrPasskeySessionInvalid,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sessionRepo := tt.records.sessions
			sessionRepo.consumed = tt.session
			svc := &operatorPasskeyService{
				records:            tt.records,
				operators:          tt.operators,
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
		name       string
		session    *platformModel.OperatorPasskeySession
		consumeErr error
		wantErr    error
	}{
		{
			name:    "no pending ceremony",
			wantErr: authService.ErrPasskeySessionInvalid,
		},
		{
			name:       "consume store failure is not an invalid session",
			consumeErr: errOperatorPasskeyStore,
			wantErr:    errOperatorPasskeyStore,
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
			sessionRepo := &operatorPasskeySessionRepoStub{consumed: tt.session, consumeErr: tt.consumeErr}
			svc := &operatorPasskeyService{
				records:            &operatorPasskeyRecordsStub{sessions: sessionRepo},
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

// operatorPasskeyAssertion is a parseable discoverable-login response. The
// WebAuthn library resolves the credential from it before checking the
// signature, so it drives FinishLogin to the credential lookup.
func operatorPasskeyAssertion(t *testing.T) json.RawMessage {
	t.Helper()
	encode := base64.RawURLEncoding.EncodeToString
	clientData := `{"type":"webauthn.get","challenge":"Y2hhbGxlbmdl","origin":"http://operator.localhost:3000"}`
	raw, err := json.Marshal(map[string]any{
		"id": encode([]byte("credential-id")), "rawId": encode([]byte("credential-id")), "type": "public-key",
		"response": map[string]string{
			"clientDataJSON":    encode([]byte(clientData)),
			"authenticatorData": encode(make([]byte, 37)),
			"signature":         encode([]byte("signature")),
			"userHandle":        encode([]byte("operator-handle")),
		},
	})
	require.NoError(t, err)
	return raw
}

// A failed credential lookup must surface as the store error, not as wrong
// credentials; an unknown credential still reads as wrong credentials.
func TestOperatorPasskeyFinishLoginCredentialLookup(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		credentials *operatorPasskeyCredentialRepoStub
		wantErr     error
		wantInvalid bool
	}{
		{name: "store failure", credentials: &operatorPasskeyCredentialRepoStub{err: errOperatorPasskeyStore}, wantErr: errOperatorPasskeyStore},
		{name: "unknown credential", credentials: &operatorPasskeyCredentialRepoStub{}, wantInvalid: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sessions := &operatorPasskeySessionRepoStub{consumed: &platformModel.OperatorPasskeySession{
				SessionJSON:    json.RawMessage(`{"challenge":"Y2hhbGxlbmdl"}`),
				ExpectedOrigin: "http://operator.localhost:3000",
			}}
			svc := &operatorPasskeyService{
				records:            &operatorPasskeyRecordsStub{credentials: tt.credentials, sessions: sessions},
				rpID:               "operator.localhost",
				rpName:             "moto",
				operatorOriginHost: "operator.localhost",
			}
			_, err := svc.FinishLogin(context.Background(), OperatorPasskeyLoginFinishRequest{
				SessionID:          "session-id",
				CredentialResponse: operatorPasskeyAssertion(t),
			})
			if tt.wantInvalid {
				var invalid *InvalidCredentialsError
				require.ErrorAs(t, err, &invalid)
				return
			}
			require.ErrorIs(t, err, tt.wantErr)
			var invalid *InvalidCredentialsError
			assert.False(t, errors.As(err, &invalid), "a store failure is not a credential failure")
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
	svc := &operatorPasskeyService{records: &operatorPasskeyRecordsStub{credentials: repo}}

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
	svc := &operatorPasskeyService{records: &operatorPasskeyRecordsStub{credentials: repo}}

	_, err := svc.ListCredentials(context.Background(), 1)
	require.ErrorIs(t, err, wantErr)

	err = svc.RevokeCredential(context.Background(), 1, 2)
	require.ErrorIs(t, err, wantErr, "a store failure is not a missing passkey")
	require.NotErrorIs(t, err, authService.ErrPasskeyNotFound)

	_, err = svc.passkeyUserForOperator(context.Background(), &platformModel.Operator{Email: "operator@example.test"})
	require.ErrorIs(t, err, wantErr)

	missing := &operatorPasskeyService{records: &operatorPasskeyRecordsStub{credentials: &operatorPasskeyCredentialRepoStub{notRevoked: true}}}
	err = missing.RevokeCredential(context.Background(), 1, 2)
	require.ErrorIs(t, err, authService.ErrPasskeyNotFound, "a passkey that is gone, revoked or foreign is not found")
}

var errOperatorPasskeyStore = errors.New("passkey store down")

// operatorPasskeyRecordsStub serves the records port from a credential and
// a session stub.
type operatorPasskeyRecordsStub struct {
	credentials *operatorPasskeyCredentialRepoStub
	sessions    *operatorPasskeySessionRepoStub
}

func (r *operatorPasskeyRecordsStub) CreateCredential(ctx context.Context, credential *platformModel.OperatorPasskeyCredential) error {
	return r.credentials.Create(ctx, credential)
}

func (r *operatorPasskeyRecordsStub) ListActiveCredentials(ctx context.Context, operatorID int64) ([]*platformModel.OperatorPasskeyCredential, error) {
	return r.credentials.FindActiveByOperatorID(ctx, operatorID)
}

func (r *operatorPasskeyRecordsStub) FindActiveCredential(ctx context.Context, credentialID, userHandle []byte) (*platformModel.OperatorPasskeyCredential, error) {
	return r.credentials.FindActiveByCredentialIDAndUserHandle(ctx, credentialID, userHandle)
}

func (r *operatorPasskeyRecordsStub) RecordCredentialUse(ctx context.Context, id int64, credentialJSON []byte, usedAt time.Time) error {
	return r.credentials.UpdateAfterUse(ctx, id, credentialJSON, usedAt)
}

func (r *operatorPasskeyRecordsStub) RevokeCredential(ctx context.Context, operatorID, id int64, revokedAt time.Time) (bool, error) {
	return r.credentials.Revoke(ctx, operatorID, id, revokedAt)
}

func (r *operatorPasskeyRecordsStub) CreateSession(ctx context.Context, session *platformModel.OperatorPasskeySession) error {
	return r.sessions.Create(ctx, session)
}

func (r *operatorPasskeyRecordsStub) ConsumeSession(ctx context.Context, id, purpose string, consumedAt time.Time) (*platformModel.OperatorPasskeySession, error) {
	return r.sessions.Consume(ctx, id, purpose, consumedAt)
}

type operatorPasskeyCredentialRepoStub struct {
	rows                []*platformModel.OperatorPasskeyCredential
	err                 error
	notRevoked          bool
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

func (r *operatorPasskeyCredentialRepoStub) Revoke(_ context.Context, operatorID, id int64, _ time.Time) (bool, error) {
	r.revokedOperatorID = operatorID
	r.revokedCredentialID = id
	if r.err != nil {
		return false, r.err
	}
	return !r.notRevoked, nil
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
	return r.consumed, nil
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

func (r *operatorPasskeyOperatorRepoStub) IncrementMFAAttempts(context.Context, int64, int, time.Duration) (OperatorMFAAttempts, error) {
	return OperatorMFAAttempts{}, nil
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
