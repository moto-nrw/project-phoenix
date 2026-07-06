package auth

import (
	"github.com/moto-nrw/project-phoenix/models/base"
)

// MFAOverrideSetByType discriminates the two surfaces that can write an
// override row. Tenant-admin writes (`account`) are scoped to a single
// tenant; operator writes (`operator`) can be platform-wide (TenantID
// nil) and act across every tenant the account belongs to.
const (
	MFAOverrideSetByTypeOperator = "operator"
)

// Override value constants. The "none" string is the API-level
// representation of "no override" — at the row level it means
// "delete the row", since auth.mfa_overrides has no representation
// for an absent override beyond row absence.
const (
	MFAAdminOverrideNone     = "none"
	MFAAdminOverrideForceOff = "force_off"
	MFAAdminOverrideForceOn  = "force_on"
)

// MFAOverride is one row in auth.mfa_overrides. TenantID is *int64
// because the platform-wide ("operator emergency switch") row uses
// SQL NULL — the rule the resolver enforces is:
//
//  1. Platform-wide row (TenantID == nil) wins → applies to every login.
//  2. Tenant-scoped row (TenantID == loginTenant) wins next.
//  3. No row → tenant policy decides.
//
// The schema enforces "at most one global row per account" and
// "at most one tenant-scoped row per (account, tenant)" via partial
// unique indexes, so the resolver never has to disambiguate duplicates.
type MFAOverride struct {
	base.Model `bun:"schema:auth,table:mfa_overrides"`
	AccountID  int64 `bun:"account_id,notnull" json:"account_id"`
	// TenantID nil = platform-wide ("operator account-wide override").
	// Tenant admins can only ever write rows with a concrete TenantID.
	TenantID *int64 `bun:"tenant_id" json:"tenant_id,omitempty"`
	// Override holds "force_off" or "force_on". A "none" outcome is
	// expressed by *not* having a row — there is no third value here.
	Override string `bun:"override,notnull" json:"override"`
	// SetBy is the principal that wrote this row. Paired with
	// SetByType because the operator and account ID spaces are
	// disjoint tables, not a shared key space.
	SetBy     int64  `bun:"set_by,notnull" json:"set_by"`
	SetByType string `bun:"set_by_type,notnull" json:"set_by_type"`
	Reason    string `bun:"reason,notnull" json:"reason"`
}

// IsGlobal reports whether this row applies platform-wide (any tenant
// the account belongs to). Used by the audit + UI to label rows.
func (o *MFAOverride) IsGlobal() bool { return o.TenantID == nil }
