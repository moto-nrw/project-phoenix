package auth

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	authModel "github.com/moto-nrw/project-phoenix/models/auth"
	"github.com/moto-nrw/project-phoenix/models/base"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func TestPasskeyTenantOriginValidation(t *testing.T) {
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
	assert.Equal(t, "example.com", hostWithoutPort("https://Example.COM:443/ignored"))
	assert.Equal(t, "school.localhost", hostWithoutPort("school.localhost:3000"))

	host, err := originHostWithoutPort("http://school.localhost:3000")
	require.NoError(t, err)
	assert.Equal(t, "school.localhost", host)

	assert.Equal(t, "Short name", normalizePasskeyName("  Short name  "))
	assert.Empty(t, normalizePasskeyName("   "))
	assert.Len(t, normalizePasskeyName(strings.Repeat("a", 90)), 80)

	req, err := passkeyResponseRequest(context.Background(), json.RawMessage(`{"id":"credential"}`))
	require.NoError(t, err)
	assert.Equal(t, "application/json", req.Header.Get("Content-Type"))

	_, err = passkeyResponseRequest(context.Background(), json.RawMessage(`{`))
	require.ErrorIs(t, err, ErrPasskeySessionInvalid)

	platformReq, err := PasskeyResponseRequestForPlatform(context.Background(), json.RawMessage(`{"id":"platform"}`))
	require.NoError(t, err)
	assert.Equal(t, "application/json", platformReq.Header.Get("Content-Type"))
	assert.Equal(t, normalizePasskeyName("Example"), NormalizePasskeyNameForPlatform("Example"))
	assert.Equal(t, passkeyUserHandleBytes, PasskeyUserHandleBytesForPlatform())
}

func TestPasskeySummaryAndUser(t *testing.T) {
	now := time.Now().UTC()
	lastUsedAt := now.Add(time.Minute)
	row := &authModel.PasskeyCredential{
		Model:      base.Model{ID: 44, CreatedAt: now},
		Name:       "Laptop",
		LastUsedAt: &lastUsedAt,
	}
	summary := summarizePasskeyCredential(row)

	require.NotNil(t, summary)
	assert.Equal(t, row.ID, summary.ID)
	assert.Equal(t, "Laptop", summary.Name)
	assert.Equal(t, now, summary.CreatedAt)
	assert.Equal(t, &lastUsedAt, summary.LastUsedAt)

	user := &tenantPasskeyUser{
		userHandle:  []byte("handle"),
		name:        "teacher@example.test",
		displayName: "Teacher",
	}
	assert.Equal(t, []byte("handle"), user.WebAuthnID())
	assert.Equal(t, "teacher@example.test", user.WebAuthnName())
	assert.Equal(t, "Teacher", user.WebAuthnDisplayName())
	assert.Empty(t, user.WebAuthnCredentials())
}

func TestPasskeyCredentialServiceMethods(t *testing.T) {
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
	assert.Equal(t, row.ID, list[0].ID)

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
	assert.Len(t, user.WebAuthnID(), passkeyUserHandleBytes)

	repo.rows = []*authModel.PasskeyCredential{{CredentialJSON: json.RawMessage(`{`)}}
	_, err = svc.passkeyUserForAccount(context.Background(), account)
	require.Error(t, err)
}

func TestPasskeyCredentialServiceErrors(t *testing.T) {
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

func (r *passkeyCredentialRepoStub) FindActiveByCredentialID(context.Context, []byte) (*authModel.PasskeyCredential, error) {
	if len(r.rows) == 0 {
		return nil, r.err
	}
	return r.rows[0], r.err
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

func TestPasskeyRepositories(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
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

	byID, err := repos.PasskeyCredential.FindActiveByCredentialID(ctx, credentialID)
	require.NoError(t, err)
	assert.Equal(t, credential.ID, byID.ID)

	byHandle, err := repos.PasskeyCredential.FindActiveByCredentialIDAndUserHandle(ctx, credentialID, userHandle)
	require.NoError(t, err)
	assert.Equal(t, credential.ID, byHandle.ID)

	usedAt := time.Now().UTC().Truncate(time.Microsecond)
	require.NoError(t, repos.PasskeyCredential.UpdateAfterUse(ctx, credential.ID, json.RawMessage(`{"id":"updated"}`), usedAt))
	updated, err := repos.PasskeyCredential.FindActiveByCredentialID(ctx, credentialID)
	require.NoError(t, err)
	require.NotNil(t, updated.LastUsedAt)
	assert.JSONEq(t, `{"id":"updated"}`, string(updated.CredentialJSON))

	require.NoError(t, repos.PasskeyCredential.Revoke(ctx, account.ID, credential.ID, time.Now()))
	active, err = repos.PasskeyCredential.FindActiveByAccountID(ctx, account.ID)
	require.NoError(t, err)
	assert.Empty(t, active)

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
