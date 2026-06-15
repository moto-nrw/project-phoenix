package platform

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
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
	now := time.Now().UTC()
	lastUsedAt := now.Add(time.Minute)
	row := &platformModel.OperatorPasskeyCredential{
		Model:      base.Model{ID: 55, CreatedAt: now},
		Name:       "Admin laptop",
		LastUsedAt: &lastUsedAt,
	}
	summary := summarizeOperatorPasskeyCredential(row)

	require.NotNil(t, summary)
	assert.Equal(t, row.ID, summary.ID)
	assert.Equal(t, "Admin laptop", summary.Name)
	assert.Equal(t, now, summary.CreatedAt)
	assert.Equal(t, &lastUsedAt, summary.LastUsedAt)

	user := &operatorPasskeyUser{
		userHandle:  []byte("handle"),
		name:        "operator@example.test",
		displayName: "Operator",
	}
	assert.Equal(t, []byte("handle"), user.WebAuthnID())
	assert.Equal(t, "operator@example.test", user.WebAuthnName())
	assert.Equal(t, "Operator", user.WebAuthnDisplayName())
	assert.Empty(t, user.WebAuthnCredentials())
}

func TestOperatorPasskeyCredentialServiceMethods(t *testing.T) {
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
	assert.Equal(t, row.ID, list[0].ID)

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
	assert.Len(t, user.WebAuthnID(), authService.PasskeyUserHandleBytesForPlatform())

	repo.rows = []*platformModel.OperatorPasskeyCredential{{CredentialJSON: json.RawMessage(`{`)}}
	_, err = svc.passkeyUserForOperator(context.Background(), operator)
	require.Error(t, err)
}

func TestOperatorPasskeyCredentialServiceErrors(t *testing.T) {
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

func (r *operatorPasskeyCredentialRepoStub) FindActiveByCredentialID(context.Context, []byte) (*platformModel.OperatorPasskeyCredential, error) {
	if len(r.rows) == 0 {
		return nil, r.err
	}
	return r.rows[0], r.err
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

func TestOperatorPasskeyRepositories(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
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

	byID, err := repos.OperatorPasskeyCredential.FindActiveByCredentialID(ctx, credentialID)
	require.NoError(t, err)
	assert.Equal(t, credential.ID, byID.ID)

	byHandle, err := repos.OperatorPasskeyCredential.FindActiveByCredentialIDAndUserHandle(ctx, credentialID, userHandle)
	require.NoError(t, err)
	assert.Equal(t, credential.ID, byHandle.ID)

	usedAt := time.Now().UTC().Truncate(time.Microsecond)
	require.NoError(t, repos.OperatorPasskeyCredential.UpdateAfterUse(ctx, credential.ID, json.RawMessage(`{"id":"operator-updated"}`), usedAt))
	updated, err := repos.OperatorPasskeyCredential.FindActiveByCredentialID(ctx, credentialID)
	require.NoError(t, err)
	require.NotNil(t, updated.LastUsedAt)
	assert.JSONEq(t, `{"id":"operator-updated"}`, string(updated.CredentialJSON))

	require.NoError(t, repos.OperatorPasskeyCredential.Revoke(ctx, operator.ID, credential.ID, time.Now()))
	active, err = repos.OperatorPasskeyCredential.FindActiveByOperatorID(ctx, operator.ID)
	require.NoError(t, err)
	assert.Empty(t, active)

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
