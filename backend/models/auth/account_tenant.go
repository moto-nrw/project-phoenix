package auth

import (
	"errors"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
)

// Account tenant status constants
const (
	AccountTenantStatusPending  = "pending"
	AccountTenantStatusActive   = "active"
	AccountTenantStatusInactive = "inactive"
)

// AccountTenant maps an account to a tenant (school) with lifecycle status.
type AccountTenant struct {
	base.Model    `bun:"schema:auth,table:account_tenants"`
	AccountID     int64      `bun:"account_id,notnull" json:"account_id"`
	TenantID      int64      `bun:"tenant_id,notnull" json:"tenant_id"`
	Status        string     `bun:"status,notnull,default:'active'" json:"status"`
	InvitedAt     *time.Time `bun:"invited_at" json:"invited_at,omitempty"`
	ActivatedAt   *time.Time `bun:"activated_at" json:"activated_at,omitempty"`
	DeactivatedAt *time.Time `bun:"deactivated_at" json:"deactivated_at,omitempty"`

	// Relations
}

// Validate ensures account tenant data is valid
func (at *AccountTenant) Validate() error {
	at.Status = strings.TrimSpace(strings.ToLower(at.Status))

	if at.AccountID == 0 {
		return errors.New("account_id is required")
	}
	if at.TenantID == 0 {
		return errors.New("tenant_id is required")
	}
	switch at.Status {
	case AccountTenantStatusPending, AccountTenantStatusActive, AccountTenantStatusInactive:
		// valid
	default:
		return errors.New("status must be one of: pending, active, inactive")
	}
	return nil
}

// IsActive returns true if the account-tenant mapping is active
func (at *AccountTenant) IsActive() bool {
	return at.Status == AccountTenantStatusActive
}
