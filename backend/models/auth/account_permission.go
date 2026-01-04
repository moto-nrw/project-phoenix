package auth

import (
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/uptrace/bun"
)

// tableAuthAccountPermissions is the schema-qualified table name for account permissions
const tableAuthAccountPermissions = "auth.account_permissions"

// AccountPermission represents a direct permission assignment to an account
type AccountPermission struct {
	base.Model   `bun:"schema:auth,table:account_permissions"`
	AccountID    int64 `bun:"account_id,notnull" json:"account_id"`
	PermissionID int64 `bun:"permission_id,notnull" json:"permission_id"`
	Granted      bool  `bun:"granted,notnull,default:true" json:"granted"`

	// Relations
	Account    *Account    `bun:"rel:belongs-to,join:account_id=id" json:"account,omitempty"`
	Permission *Permission `bun:"rel:belongs-to,join:permission_id=id" json:"permission,omitempty"`
}

// TableName returns the database table name
func (ap *AccountPermission) TableName() string {
	return tableAuthAccountPermissions
}

func (ap *AccountPermission) BeforeAppendModel(query any) error {
	if q, ok := query.(*bun.SelectQuery); ok {
		q.ModelTableExpr(tableAuthAccountPermissions)
	}
	if q, ok := query.(*bun.UpdateQuery); ok {
		q.ModelTableExpr(tableAuthAccountPermissions)
	}
	if q, ok := query.(*bun.DeleteQuery); ok {
		q.ModelTableExpr(tableAuthAccountPermissions)
	}
	return nil
}

// Validate ensures account permission data is valid
func (ap *AccountPermission) Validate() error {
	if ap.AccountID <= 0 {
		return errors.New("account ID is required")
	}

	if ap.PermissionID <= 0 {
		return errors.New("permission ID is required")
	}

	return nil
}

// IsGranted returns whether this permission is granted or denied
func (ap *AccountPermission) IsGranted() bool {
	return ap.Granted
}

// Grant changes the permission to granted
func (ap *AccountPermission) Grant() {
	ap.Granted = true
}

// Deny changes the permission to denied
func (ap *AccountPermission) Deny() {
	ap.Granted = false
}

// GetID returns the entity's ID
func (m *AccountPermission) GetID() interface{} {
	return m.ID
}

// GetCreatedAt returns the creation timestamp
func (m *AccountPermission) GetCreatedAt() time.Time {
	return m.CreatedAt
}

// GetUpdatedAt returns the last update timestamp
func (m *AccountPermission) GetUpdatedAt() time.Time {
	return m.UpdatedAt
}
