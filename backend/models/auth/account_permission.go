package auth

import (
	"errors"

	"github.com/moto-nrw/project-phoenix/models/base"
)

// AccountPermission represents a direct permission assignment to an account
type AccountPermission struct {
	base.Model
	AccountID    int64 `bun:"account_id,notnull" json:"account_id"`
	PermissionID int64 `bun:"permission_id,notnull" json:"permission_id"`
	Granted      bool  `bun:"granted,notnull,default:true" json:"granted"`

	// Relations
	Account    *Account    `bun:"rel:belongs-to,join:account_id=id" json:"account,omitempty"`
	Permission *Permission `bun:"rel:belongs-to,join:permission_id=id" json:"permission,omitempty"`
}

// TableName returns the database table name
func (ap *AccountPermission) TableName() string {
	return "auth.account_permissions"
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
