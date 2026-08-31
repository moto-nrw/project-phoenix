package platform

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOperatorPasskeyCredentialModel(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()
	credential := &OperatorPasskeyCredential{
		Model:          base.Model{ID: 12, CreatedAt: now, UpdatedAt: now},
		OperatorID:     34,
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
		credential OperatorPasskeyCredential
	}{
		{name: "missing operator", credential: OperatorPasskeyCredential{UserHandle: []byte("u"), CredentialID: []byte("c"), CredentialJSON: json.RawMessage(`{}`)}},
		{name: "missing user handle", credential: OperatorPasskeyCredential{OperatorID: 1, CredentialID: []byte("c"), CredentialJSON: json.RawMessage(`{}`)}},
		{name: "missing credential id", credential: OperatorPasskeyCredential{OperatorID: 1, UserHandle: []byte("u"), CredentialJSON: json.RawMessage(`{}`)}},
		{name: "invalid json", credential: OperatorPasskeyCredential{OperatorID: 1, UserHandle: []byte("u"), CredentialID: []byte("c"), CredentialJSON: json.RawMessage(`{`)}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Error(t, tt.credential.Validate())
		})
	}
}

func TestOperatorPasskeySessionModel(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()
	session := &OperatorPasskeySession{
		StringIDModelWithoutNullZero: base.StringIDModelWithoutNullZero{ID: "session", CreatedAt: now, UpdatedAt: now},
		Purpose:                      OperatorPasskeySessionPurposeRegistration,
		RPID:                         "operator.localhost",
		ExpectedOrigin:               "http://operator.localhost:3000",
		SessionJSON:                  json.RawMessage(`{"challenge":"abc"}`),
		ExpiresAt:                    now.Add(time.Minute),
	}

	require.NoError(t, session.Validate())
	assert.Equal(t, "session", session.GetID())
	assert.Equal(t, now, session.GetCreatedAt())
	assert.Equal(t, now, session.GetUpdatedAt())

	tests := []struct {
		name    string
		session OperatorPasskeySession
	}{
		{name: "missing id", session: OperatorPasskeySession{Purpose: OperatorPasskeySessionPurposeLogin, RPID: "operator.localhost", ExpectedOrigin: "http://operator.localhost", SessionJSON: json.RawMessage(`{}`), ExpiresAt: now}},
		{name: "bad purpose", session: OperatorPasskeySession{StringIDModelWithoutNullZero: base.StringIDModelWithoutNullZero{ID: "s"}, Purpose: "bad", RPID: "operator.localhost", ExpectedOrigin: "http://operator.localhost", SessionJSON: json.RawMessage(`{}`), ExpiresAt: now}},
		{name: "missing rp id", session: OperatorPasskeySession{StringIDModelWithoutNullZero: base.StringIDModelWithoutNullZero{ID: "s"}, Purpose: OperatorPasskeySessionPurposeLogin, ExpectedOrigin: "http://operator.localhost", SessionJSON: json.RawMessage(`{}`), ExpiresAt: now}},
		{name: "missing origin", session: OperatorPasskeySession{StringIDModelWithoutNullZero: base.StringIDModelWithoutNullZero{ID: "s"}, Purpose: OperatorPasskeySessionPurposeLogin, RPID: "operator.localhost", SessionJSON: json.RawMessage(`{}`), ExpiresAt: now}},
		{name: "invalid json", session: OperatorPasskeySession{StringIDModelWithoutNullZero: base.StringIDModelWithoutNullZero{ID: "s"}, Purpose: OperatorPasskeySessionPurposeLogin, RPID: "operator.localhost", ExpectedOrigin: "http://operator.localhost", SessionJSON: json.RawMessage(`{`), ExpiresAt: now}},
		{name: "missing expiry", session: OperatorPasskeySession{StringIDModelWithoutNullZero: base.StringIDModelWithoutNullZero{ID: "s"}, Purpose: OperatorPasskeySessionPurposeLogin, RPID: "operator.localhost", ExpectedOrigin: "http://operator.localhost", SessionJSON: json.RawMessage(`{}`)}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Error(t, tt.session.Validate())
		})
	}
}
