package auth

import (
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
)

// Token represents an authentication token in the system
type Token struct {
	base.Model `bun:"schema:auth,table:tokens"`
	base.TenantModel
	AccountID   int64     `bun:"account_id,notnull" json:"account_id"`
	Token       string    `bun:"token,notnull" json:"token"`
	Expiry      time.Time `bun:"expiry,notnull" json:"expiry"`
	Mobile      bool      `bun:"mobile,notnull,default:false" json:"mobile"`
	Identifier  *string   `bun:"identifier" json:"identifier,omitempty"`
	PortalScope string    `bun:"portal_scope,notnull,default:'unknown'" json:"portal_scope"`

	// Token family tracking for detecting token theft
	FamilyID          string     `bun:"family_id" json:"family_id,omitempty"`
	Generation        int        `bun:"generation,default:0" json:"generation"`
	RotatedAt         *time.Time `bun:"rotated_at" json:"-"`
	ReplacementToken  *string    `bun:"replacement_token" json:"-"`
	RecoveryProofHash []byte     `bun:"recovery_proof_hash" json:"-"`

	// Relations
	Account *Account `bun:"rel:belongs-to,join:account_id=id" json:"account,omitempty"`
}

const (
	PortalScopeTenant  = "tenant"
	PortalScopeOrg     = "org"
	PortalScopeParent  = "parent"
	PortalScopeSchool  = "school"
	PortalScopeUnknown = "unknown"
)

// Validate ensures token data is valid. It performs pure field validation only.
// The expiry/validity decision is wall-clock policy enforced by the read paths'
// SQL expiry filters, per issue #586 (Rule 12: models hold data, not
// decisions).
func (t *Token) Validate() error {
	if t.AccountID <= 0 {
		return errors.New("account ID is required")
	}

	if t.Token == "" {
		return errors.New("token value is required")
	}
	if t.PortalScope != "" {
		switch t.PortalScope {
		case PortalScopeTenant, PortalScopeOrg, PortalScopeParent, PortalScopeSchool, PortalScopeUnknown:
		default:
			return errors.New("invalid portal scope")
		}
	}
	if (t.RotatedAt == nil) != (t.ReplacementToken == nil) {
		return errors.New("rotation handoff must include both timestamp and replacement token")
	}
	if t.RotatedAt == nil && len(t.RecoveryProofHash) != 0 {
		return errors.New("recovery proof hash requires a rotation handoff")
	}

	return nil
}

// SetExpiry sets the token expiry time to a specified duration from now
func (t *Token) SetExpiry(duration time.Duration) {
	t.Expiry = time.Now().Add(duration)
}
