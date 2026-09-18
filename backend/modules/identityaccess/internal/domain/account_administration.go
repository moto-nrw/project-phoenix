package domain

import "time"

// AccountVisibilityKind names the account set a caller's context may
// administer. Accounts are platform-wide rows, so every administrative read
// and write carries this predicate: without it a school admin would see and
// change accounts of other schools.
type AccountVisibilityKind int

const (
	// AccountVisibilityGlobal administers every account. It is what a
	// platform-scoped caller and an administrative transaction get: the
	// operator flows and the CLI.
	AccountVisibilityGlobal AccountVisibilityKind = iota
	// AccountVisibilityDenied administers no account. An organisation or
	// school context whose scope could not be resolved lands here rather
	// than falling back to a wider one.
	AccountVisibilityDenied
	// AccountVisibilityOrganization administers the accounts actively
	// mapped to one of the organisation's manageable schools.
	AccountVisibilityOrganization
	// AccountVisibilityTenant administers the accounts actively mapped to
	// the school in context.
	AccountVisibilityTenant
)

// AccountVisibility is the resolved predicate of one caller.
type AccountVisibility struct {
	Kind AccountVisibilityKind
	// TenantID is the school of a tenant-scoped caller.
	TenantID int64
	// SchoolIDs are the manageable schools of an organisation-scoped caller.
	SchoolIDs []int64
}

// ManagedAccount is the platform account row the administration reads. The
// credential hash is filled only by the reads that verify one.
type ManagedAccountRecord struct {
	ID           int64
	Email        string
	Username     string
	Active       bool
	PasswordHash string
	CreatedAt    time.Time
	UpdatedAt    time.Time
	LastLogin    *time.Time
}

// AccountListFilter narrows an account listing. An empty address and a nil
// active flag list every account the caller may administer.
type AccountListFilter struct {
	// Email matches the stored address case-insensitively and in full.
	Email  string
	Active *bool
}

// AccountIdentityUpdate changes the address and, when Username is set, the
// name of one account. A nil Username keeps the stored one.
type AccountIdentityUpdate struct {
	AccountID int64
	Email     string
	Username  *string
}
