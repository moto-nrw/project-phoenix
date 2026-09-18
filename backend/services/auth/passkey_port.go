package auth

import (
	"context"
	"encoding/json"
	"time"
)

// The ceremony purposes a school-portal passkey session is bound to.
const (
	PasskeySessionPurposeRegistration = "registration"
	PasskeySessionPurposeLogin        = "login"
)

// PasskeyCredential is one registered WebAuthn credential of a school-portal
// account, as the retained flow reads and writes it. Identity & Access owns
// the row (#2724); this is the port's value type.
type PasskeyCredential struct {
	ID             int64
	AccountID      int64
	UserHandle     []byte
	CredentialID   []byte
	CredentialJSON json.RawMessage
	Name           string
	LastUsedAt     *time.Time
	RevokedAt      *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// PasskeySession is the server-side state of one school-portal ceremony.
// AccountID is nil for a discoverable login; TenantID names the school whose
// portal started it.
type PasskeySession struct {
	ID             string
	AccountID      *int64
	TenantID       *int64
	Purpose        string
	RPID           string
	ExpectedOrigin string
	SessionJSON    json.RawMessage
	ExpiresAt      time.Time
	ConsumedAt     *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// PasskeyRecords is the consumer-owned port over the school-portal passkey
// credentials and ceremony sessions, which Identity & Access owns (#2724).
// A missing row is (nil, nil): no active credential for a lookup, no
// unconsumed and unexpired ceremony with that purpose for a consumption.
// RevokeCredential reports whether an active credential of that account was
// revoked. RecordCredentialUse fails when the credential is no longer
// active. Every call joins the caller's transaction when one is active.
type PasskeyRecords interface {
	CreateCredential(ctx context.Context, credential *PasskeyCredential) error
	// ListActiveCredentials orders by registration, oldest first.
	ListActiveCredentials(ctx context.Context, accountID int64) ([]*PasskeyCredential, error)
	FindActiveCredential(ctx context.Context, credentialID, userHandle []byte) (*PasskeyCredential, error)
	RecordCredentialUse(ctx context.Context, id int64, credentialJSON []byte, usedAt time.Time) error
	RevokeCredential(ctx context.Context, accountID, id int64, revokedAt time.Time) (bool, error)

	CreateSession(ctx context.Context, session *PasskeySession) error
	ConsumeSession(ctx context.Context, id, purpose string, consumedAt time.Time) (*PasskeySession, error)
}
