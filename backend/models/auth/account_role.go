package auth

import (
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/uptrace/bun"
)

// tableAuthAccountRoles is the schema-qualified table name for account roles
const tableAuthAccountRoles = "auth.account_roles"

// AccountRole represents a mapping between accounts and roles
type AccountRole struct {
	base.Model `bun:"schema:auth,table:account_roles"`
	AccountID  int64 `bun:"account_id,notnull" json:"account_id"`
	RoleID     int64 `bun:"role_id,notnull" json:"role_id"`

	// Relations
	Account *Account `bun:"rel:belongs-to,join:account_id=id" json:"account,omitempty"`
	Role    *Role    `bun:"rel:belongs-to,join:role_id=id" json:"role,omitempty"`
}

func (ar *AccountRole) BeforeAppendModel(query any) error {
	if q, ok := query.(*bun.SelectQuery); ok {
		q.ModelTableExpr(tableAuthAccountRoles)
	}
	if q, ok := query.(*bun.UpdateQuery); ok {
		q.ModelTableExpr(tableAuthAccountRoles)
	}
	if q, ok := query.(*bun.DeleteQuery); ok {
		q.ModelTableExpr(tableAuthAccountRoles)
	}
	return nil
}

// TableName returns the database table name
func (ar *AccountRole) TableName() string {
	return tableAuthAccountRoles
}

// Validate ensures account role mapping data is valid
func (ar *AccountRole) Validate() error {
	if ar.AccountID <= 0 {
		return errors.New("account ID is required")
	}

	if ar.RoleID <= 0 {
		return errors.New("role ID is required")
	}

	return nil
}

// GetID returns the entity's ID
func (m *AccountRole) GetID() interface{} {
	return m.ID
}

// GetCreatedAt returns the creation timestamp
func (m *AccountRole) GetCreatedAt() time.Time {
	return m.CreatedAt
}

// GetUpdatedAt returns the last update timestamp
func (m *AccountRole) GetUpdatedAt() time.Time {
	return m.UpdatedAt
}
