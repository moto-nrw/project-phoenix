package auth

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPasskeyCredentialModel(t *testing.T) {
	now := time.Now().UTC()
	credential := &PasskeyCredential{
		Model:          base.Model{ID: 12, CreatedAt: now, UpdatedAt: now},
		AccountID:      34,
		UserHandle:     []byte("user"),
		CredentialID:   []byte("credential"),
		CredentialJSON: json.RawMessage(`{"id":"credential"}`),
	}

	require.NoError(t, credential.Validate())
	assert.Equal(t, credential.ID, credential.GetID())
	assert.Equal(t, now, credential.GetCreatedAt())
	assert.Equal(t, now, credential.GetUpdatedAt())

	tests := []struct {
		name       string
		credential PasskeyCredential
	}{
		{name: "missing account", credential: PasskeyCredential{UserHandle: []byte("u"), CredentialID: []byte("c"), CredentialJSON: json.RawMessage(`{}`)}},
		{name: "missing user handle", credential: PasskeyCredential{AccountID: 1, CredentialID: []byte("c"), CredentialJSON: json.RawMessage(`{}`)}},
		{name: "missing credential id", credential: PasskeyCredential{AccountID: 1, UserHandle: []byte("u"), CredentialJSON: json.RawMessage(`{}`)}},
		{name: "invalid json", credential: PasskeyCredential{AccountID: 1, UserHandle: []byte("u"), CredentialID: []byte("c"), CredentialJSON: json.RawMessage(`{`)}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Error(t, tt.credential.Validate())
		})
	}
}

func TestPasskeySessionModel(t *testing.T) {
	now := time.Now().UTC()
	session := &PasskeySession{
		ID:             "session",
		Purpose:        PasskeySessionPurposeRegistration,
		RPID:           "localhost",
		ExpectedOrigin: "http://localhost:3000",
		SessionJSON:    json.RawMessage(`{"challenge":"abc"}`),
		ExpiresAt:      now.Add(time.Minute),
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	require.NoError(t, session.Validate())
	assert.Equal(t, "session", session.GetID())
	assert.Equal(t, now, session.GetCreatedAt())
	assert.Equal(t, now, session.GetUpdatedAt())

	tests := []struct {
		name    string
		session PasskeySession
	}{
		{name: "missing id", session: PasskeySession{Purpose: PasskeySessionPurposeLogin, RPID: "localhost", ExpectedOrigin: "http://localhost", SessionJSON: json.RawMessage(`{}`), ExpiresAt: now}},
		{name: "bad purpose", session: PasskeySession{ID: "s", Purpose: "bad", RPID: "localhost", ExpectedOrigin: "http://localhost", SessionJSON: json.RawMessage(`{}`), ExpiresAt: now}},
		{name: "missing rp id", session: PasskeySession{ID: "s", Purpose: PasskeySessionPurposeLogin, ExpectedOrigin: "http://localhost", SessionJSON: json.RawMessage(`{}`), ExpiresAt: now}},
		{name: "missing origin", session: PasskeySession{ID: "s", Purpose: PasskeySessionPurposeLogin, RPID: "localhost", SessionJSON: json.RawMessage(`{}`), ExpiresAt: now}},
		{name: "invalid json", session: PasskeySession{ID: "s", Purpose: PasskeySessionPurposeLogin, RPID: "localhost", ExpectedOrigin: "http://localhost", SessionJSON: json.RawMessage(`{`), ExpiresAt: now}},
		{name: "missing expiry", session: PasskeySession{ID: "s", Purpose: PasskeySessionPurposeLogin, RPID: "localhost", ExpectedOrigin: "http://localhost", SessionJSON: json.RawMessage(`{}`)}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Error(t, tt.session.Validate())
		})
	}
}
