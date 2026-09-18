package authmodels

import (
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
	FamilyExpiryCap   *time.Time `bun:"family_expiry_cap" json:"-"`
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
