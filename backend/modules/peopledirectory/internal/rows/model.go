package users

import "time"

// Model holds the common row columns of the user models. It mirrors
// models/base.Model so the persisted columns and JSON stay unchanged while
// models/users no longer imports models/base for its row basis (#3991).
type Model struct {
	ID        int64     `bun:"id,pk,autoincrement" json:"id"`
	CreatedAt time.Time `bun:"created_at,nullzero,notnull,default:current_timestamp" json:"created_at"`
	UpdatedAt time.Time `bun:"updated_at,nullzero,notnull,default:current_timestamp" json:"updated_at"`
}

// TenantModel is embedded by the tenant-scoped user models.
type TenantModel struct {
	TenantID int64 `bun:"tenant_id,notnull" json:"tenant_id"`
}

// GetTenantID returns the tenant ID.
func (t *TenantModel) GetTenantID() int64 { return t.TenantID }

// SetTenantID sets the tenant ID.
func (t *TenantModel) SetTenantID(id int64) { t.TenantID = id }
