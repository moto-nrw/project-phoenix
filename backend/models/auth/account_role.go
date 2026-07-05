package auth

import (
	"errors"

	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/uptrace/bun"
)

// tableAuthAccountRoles is the schema-qualified table name for account roles
const tableAuthAccountRoles = "auth.account_roles"

// AccountRole represents a mapping between accounts and roles
type AccountRole struct {
	base.Model `bun:"schema:auth,table:account_roles"`
	base.TenantModel
	AccountID int64 `bun:"account_id,notnull" json:"account_id"`
	RoleID    int64 `bun:"role_id,notnull" json:"role_id"`

	// Relations
	Account *Account `bun:"rel:belongs-to,join:account_id=id" json:"account,omitempty"`
	Role    *Role    `bun:"rel:belongs-to,join:role_id=id" json:"role,omitempty"`
}

func (ar *AccountRole) BeforeAppendModel(query any) error {
	if q, ok := query.(*bun.UpdateQuery); ok {
		q.ModelTableExpr(tableAuthAccountRoles)
	}
	if q, ok := query.(*bun.DeleteQuery); ok {
		q.ModelTableExpr(tableAuthAccountRoles)
	}
	return nil
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
