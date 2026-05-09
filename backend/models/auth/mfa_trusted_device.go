package auth

import (
	"net"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/uptrace/bun"
)

// MFATrustedDevice persists the server-side record of a trusted-device cookie.
// The cookie value itself is HMAC-signed and contains opaque random bytes;
// only the hash is stored here so the token can be looked up + revoked
// without ever exposing a plaintext credential at rest.
type MFATrustedDevice struct {
	base.Model `bun:"schema:auth,table:mfa_trusted_devices"`
	AccountID  int64      `bun:"account_id,notnull" json:"account_id"`
	TokenHash  string     `bun:"token_hash,notnull" json:"-"`
	UserAgent  *string    `bun:"user_agent" json:"user_agent,omitempty"`
	IPAddress  net.IP     `bun:"ip_address,type:inet,nullzero" json:"ip_address,omitempty"`
	ExpiresAt  time.Time  `bun:"expires_at,notnull" json:"expires_at"`
	LastUsedAt *time.Time `bun:"last_used_at" json:"last_used_at,omitempty"`
	RevokedAt  *time.Time `bun:"revoked_at" json:"revoked_at,omitempty"`
}

// BeforeAppendModel keeps the schema-qualified table name on UPDATE/DELETE.
func (d *MFATrustedDevice) BeforeAppendModel(query any) error {
	if q, ok := query.(*bun.UpdateQuery); ok {
		q.ModelTableExpr(`auth.mfa_trusted_devices AS "mfa_trusted_device"`)
	}
	if q, ok := query.(*bun.DeleteQuery); ok {
		q.ModelTableExpr(`auth.mfa_trusted_devices AS "mfa_trusted_device"`)
	}
	return nil
}

// TableName returns the database table name.
func (d *MFATrustedDevice) TableName() string { return "auth.mfa_trusted_devices" }

// IsExpired returns true once the device's trust window has passed.
func (d *MFATrustedDevice) IsExpired() bool { return time.Now().After(d.ExpiresAt) }

// IsRevoked returns true once the device was explicitly revoked.
func (d *MFATrustedDevice) IsRevoked() bool { return d.RevokedAt != nil }

// IsActive is the convenience predicate for the trusted-device check on login.
func (d *MFATrustedDevice) IsActive() bool { return !d.IsExpired() && !d.IsRevoked() }
