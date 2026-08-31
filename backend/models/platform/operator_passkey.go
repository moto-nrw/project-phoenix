package platform

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
)

const (
	OperatorPasskeySessionPurposeRegistration = "registration"
	OperatorPasskeySessionPurposeLogin        = "login"
)

type OperatorPasskeyCredential struct {
	base.Model     `bun:"schema:platform,table:operator_passkey_credentials"`
	OperatorID     int64           `bun:"operator_id,notnull" json:"operator_id"`
	UserHandle     []byte          `bun:"user_handle,notnull" json:"-"`
	CredentialID   []byte          `bun:"credential_id,notnull" json:"-"`
	CredentialJSON json.RawMessage `bun:"credential_json,type:jsonb,notnull" json:"-"`
	Name           string          `bun:"name" json:"name"`
	LastUsedAt     *time.Time      `bun:"last_used_at" json:"last_used_at,omitempty"`
	RevokedAt      *time.Time      `bun:"revoked_at" json:"revoked_at,omitempty"`
}

func (c *OperatorPasskeyCredential) Validate() error {
	if c.OperatorID == 0 {
		return errors.New("operator_id is required")
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

type OperatorPasskeySession struct {
	base.StringIDModelWithoutNullZero
	OperatorID     *int64          `bun:"operator_id" json:"operator_id,omitempty"`
	Purpose        string          `bun:"purpose,notnull" json:"purpose"`
	RPID           string          `bun:"rp_id,notnull" json:"rp_id"`
	ExpectedOrigin string          `bun:"expected_origin,notnull" json:"expected_origin"`
	SessionJSON    json.RawMessage `bun:"session_json,type:jsonb,notnull" json:"-"`
	ExpiresAt      time.Time       `bun:"expires_at,notnull" json:"expires_at"`
	ConsumedAt     *time.Time      `bun:"consumed_at" json:"consumed_at,omitempty"`
}

func (s *OperatorPasskeySession) Validate() error {
	if s.ID == "" {
		return errors.New("id is required")
	}
	if s.Purpose != OperatorPasskeySessionPurposeRegistration && s.Purpose != OperatorPasskeySessionPurposeLogin {
		return errors.New("unsupported operator passkey session purpose")
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
