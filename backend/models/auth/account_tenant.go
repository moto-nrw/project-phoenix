package auth

import (
	"errors"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/uptrace/bun"
)

// Account tenant status constants
const (
	AccountTenantStatusPending  = "pending"
	AccountTenantStatusActive   = "active"
	AccountTenantStatusInactive = "inactive"
)

// tableAuthAccountTenants is the schema-qualified table name
const tableAuthAccountTenants = "auth.account_tenants"

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
	Account *Account `bun:"rel:belongs-to,join:account_id=id" json:"account,omitempty"`
}

func (at *AccountTenant) BeforeAppendModel(query any) error {
	if q, ok := query.(*bun.InsertQuery); ok {
		q.ModelTableExpr(tableAuthAccountTenants)
	}
	if q, ok := query.(*bun.UpdateQuery); ok {
		q.ModelTableExpr(`auth.account_tenants AS "account_tenant"`)
	}
	if q, ok := query.(*bun.DeleteQuery); ok {
		q.ModelTableExpr(`auth.account_tenants AS "account_tenant"`)
	}
	return nil
}

// TableName returns the database table name
func (at *AccountTenant) TableName() string {
	return tableAuthAccountTenants
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

// GetID returns the entity's ID
func (at *AccountTenant) GetID() any {
	return at.ID
}

// GetCreatedAt returns the creation timestamp
func (at *AccountTenant) GetCreatedAt() time.Time {
	return at.CreatedAt
}

// GetUpdatedAt returns the last update timestamp
func (at *AccountTenant) GetUpdatedAt() time.Time {
	return at.UpdatedAt
}

// IsActive returns true if the account-tenant mapping is active
func (at *AccountTenant) IsActive() bool {
	return at.Status == AccountTenantStatusActive
}
