package auth

import (
	"net"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
)

// MFAEmailChallenge represents a single 6-digit email code issued to an account
// during MFA verification. The plaintext code is never stored — only its hash.
// `consumed_at` enforces single-use; the row stays around for audit until the
// cleanup job removes expired/consumed entries.
type MFAEmailChallenge struct {
	base.Model `bun:"schema:auth,table:mfa_email_challenges"`
	AccountID  int64      `bun:"account_id,notnull" json:"account_id"`
	CodeHash   string     `bun:"code_hash,notnull" json:"-"`
	ExpiresAt  time.Time  `bun:"expires_at,notnull" json:"expires_at"`
	ConsumedAt *time.Time `bun:"consumed_at" json:"consumed_at,omitempty"`
	IPAddress  net.IP     `bun:"ip_address,type:inet,nullzero" json:"ip_address,omitempty"`
}

// IsConsumed returns true once the code has been redeemed.
func (c *MFAEmailChallenge) IsConsumed() bool { return c.ConsumedAt != nil }
