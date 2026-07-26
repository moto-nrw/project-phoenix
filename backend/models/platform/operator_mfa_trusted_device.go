package platform

import (
	"net"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
)

// OperatorMFATrustedDevice persists the server-side record of a trusted-
// device cookie issued to a moto-Operator. Mirror of auth.MFATrustedDevice
// against platform.operator_mfa_trusted_devices.
type OperatorMFATrustedDevice struct {
	base.Model `bun:"schema:platform,table:operator_mfa_trusted_devices"`
	OperatorID int64      `bun:"operator_id,notnull" json:"operator_id"`
	TokenHash  string     `bun:"token_hash,notnull" json:"-"`
	UserAgent  *string    `bun:"user_agent" json:"user_agent,omitempty"`
	IPAddress  net.IP     `bun:"ip_address,type:inet,nullzero" json:"ip_address,omitempty"`
	ExpiresAt  time.Time  `bun:"expires_at,notnull" json:"expires_at"`
	LastUsedAt *time.Time `bun:"last_used_at" json:"last_used_at,omitempty"`
	RevokedAt  *time.Time `bun:"revoked_at" json:"revoked_at,omitempty"`
}

// IsRevoked returns true once the device was explicitly revoked. This is a pure
// field accessor (RevokedAt != nil); the wall-clock expiry/active decision is
// enforced by the repository's active-device finders, per issue #586 (Rule 12).
func (d *OperatorMFATrustedDevice) IsRevoked() bool { return d.RevokedAt != nil }
