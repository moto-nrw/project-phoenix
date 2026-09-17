package domain

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The validation these tests pin moved here with the passkey records
// (#2724); it decides which rows the module refuses to store.
func TestAccountPasskeyCredentialValidate(t *testing.T) {
	t.Parallel()
	valid := AccountPasskeyCredential{
		AccountID: 7, UserHandle: []byte("handle"), CredentialID: []byte("credential"), CredentialJSON: json.RawMessage(`{}`),
	}
	require.NoError(t, valid.Validate())

	tests := map[string]struct {
		mutate func(*AccountPasskeyCredential)
		want   string
	}{
		"missing account":    {func(c *AccountPasskeyCredential) { c.AccountID = 0 }, "account_id is required"},
		"missing handle":     {func(c *AccountPasskeyCredential) { c.UserHandle = nil }, "user_handle is required"},
		"missing credential": {func(c *AccountPasskeyCredential) { c.CredentialID = nil }, "credential_id is required"},
		"invalid json":       {func(c *AccountPasskeyCredential) { c.CredentialJSON = json.RawMessage(`{`) }, "credential_json must be valid JSON"},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			credential := valid
			tt.mutate(&credential)
			assert.EqualError(t, credential.Validate(), tt.want)
		})
	}
}

func TestOperatorPasskeyCredentialValidate(t *testing.T) {
	t.Parallel()
	valid := OperatorPasskeyCredential{
		OperatorID: 7, UserHandle: []byte("handle"), CredentialID: []byte("credential"), CredentialJSON: json.RawMessage(`{}`),
	}
	require.NoError(t, valid.Validate())

	tests := map[string]struct {
		mutate func(*OperatorPasskeyCredential)
		want   string
	}{
		"missing operator":   {func(c *OperatorPasskeyCredential) { c.OperatorID = 0 }, "operator_id is required"},
		"missing handle":     {func(c *OperatorPasskeyCredential) { c.UserHandle = nil }, "user_handle is required"},
		"missing credential": {func(c *OperatorPasskeyCredential) { c.CredentialID = nil }, "credential_id is required"},
		"invalid json":       {func(c *OperatorPasskeyCredential) { c.CredentialJSON = json.RawMessage(`{`) }, "credential_json must be valid JSON"},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			credential := valid
			tt.mutate(&credential)
			assert.EqualError(t, credential.Validate(), tt.want)
		})
	}
}

func TestPasskeySessionValidate(t *testing.T) {
	t.Parallel()
	expiresAt := time.Now().Add(time.Hour)
	account := AccountPasskeySession{
		ID: "session", Purpose: PasskeySessionPurposeLogin, RPID: "school.localhost",
		ExpectedOrigin: "http://school.localhost:3000", SessionJSON: json.RawMessage(`{}`), ExpiresAt: expiresAt,
	}
	require.NoError(t, account.Validate())
	operator := OperatorPasskeySession{
		ID: "session", Purpose: PasskeySessionPurposeRegistration, RPID: "operator.localhost",
		ExpectedOrigin: "http://operator.localhost:3000", SessionJSON: json.RawMessage(`{}`), ExpiresAt: expiresAt,
	}
	require.NoError(t, operator.Validate())

	tests := map[string]struct {
		mutate func(*AccountPasskeySession)
		want   string
	}{
		"missing id":      {func(s *AccountPasskeySession) { s.ID = "" }, "id is required"},
		"unknown purpose": {func(s *AccountPasskeySession) { s.Purpose = "recovery" }, "unsupported passkey session purpose"},
		"missing rp id":   {func(s *AccountPasskeySession) { s.RPID = "" }, "rp_id is required"},
		"missing origin":  {func(s *AccountPasskeySession) { s.ExpectedOrigin = "" }, "expected_origin is required"},
		"invalid json":    {func(s *AccountPasskeySession) { s.SessionJSON = json.RawMessage(`{`) }, "session_json must be valid JSON"},
		"missing expiry":  {func(s *AccountPasskeySession) { s.ExpiresAt = time.Time{} }, "expires_at is required"},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			session := account
			tt.mutate(&session)
			assert.EqualError(t, session.Validate(), tt.want)
		})
	}
}
