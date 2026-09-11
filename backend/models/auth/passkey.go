package auth

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
)

const (
	PasskeySessionPurposeRegistration = "registration"
	PasskeySessionPurposeLogin        = "login"
)

// PasskeyCredential stores a WebAuthn credential for a tenant-portal account.
// The indexed byte columns support lookup; CredentialJSON preserves the full
// library credential record so sign counters and authenticator flags survive
// round-trips without a hand-maintained column list.
type PasskeyCredential struct {
	base.Model     `bun:"schema:auth,table:passkey_credentials"`
	AccountID      int64           `bun:"account_id,notnull" json:"account_id"`
	UserHandle     []byte          `bun:"user_handle,notnull" json:"-"`
	CredentialID   []byte          `bun:"credential_id,notnull" json:"-"`
	CredentialJSON json.RawMessage `bun:"credential_json,type:jsonb,notnull" json:"-"`
	Name           string          `bun:"name" json:"name"`
	LastUsedAt     *time.Time      `bun:"last_used_at" json:"last_used_at,omitempty"`
	RevokedAt      *time.Time      `bun:"revoked_at" json:"revoked_at,omitempty"`
}

func (c *PasskeyCredential) Validate() error {
	if c.AccountID == 0 {
		return errors.New("account_id is required")
	}
	if len(c.UserHandle) == 0 {
		return errors.New("user_handle is required")
	}
	if len(c.CredentialID) == 0 {
		return errors.New("credential_id is required")
	}
	if len(c.CredentialJSON) == 0 || !json.Valid(c.CredentialJSON) {
		return errors.New("credential_json must be valid JSON")
	}
	return nil
}

// PasskeySession stores the server-side WebAuthn ceremony state.
type PasskeySession struct {
	base.StringIDModelWithoutNullZero
	AccountID      *int64          `bun:"account_id" json:"account_id,omitempty"`
	TenantID       *int64          `bun:"tenant_id" json:"tenant_id,omitempty"`
	Purpose        string          `bun:"purpose,notnull" json:"purpose"`
	RPID           string          `bun:"rp_id,notnull" json:"rp_id"`
	ExpectedOrigin string          `bun:"expected_origin,notnull" json:"expected_origin"`
	SessionJSON    json.RawMessage `bun:"session_json,type:jsonb,notnull" json:"-"`
	ExpiresAt      time.Time       `bun:"expires_at,notnull" json:"expires_at"`
	ConsumedAt     *time.Time      `bun:"consumed_at" json:"consumed_at,omitempty"`
}

func (s *PasskeySession) Validate() error {
	if s.ID == "" {
		return errors.New("id is required")
	}
	if s.Purpose != PasskeySessionPurposeRegistration && s.Purpose != PasskeySessionPurposeLogin {
		return errors.New("unsupported passkey session purpose")
	}
	if s.RPID == "" {
		return errors.New("rp_id is required")
	}
	if s.ExpectedOrigin == "" {
		return errors.New("expected_origin is required")
	}
	if len(s.SessionJSON) == 0 || !json.Valid(s.SessionJSON) {
		return errors.New("session_json must be valid JSON")
	}
	if s.ExpiresAt.IsZero() {
		return errors.New("expires_at is required")
	}
	return nil
}
