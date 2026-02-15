package base

// TenantModel is embedded by tenant-scoped models (not platform-scoped).
// Platform models (organizations, operators) do NOT embed this.
// In Phase 1, this struct is created but not yet embedded into existing models.
type TenantModel struct {
	TenantID int64 `bun:"tenant_id,notnull,default:1" json:"tenant_id"`
}

// GetTenantID returns the tenant ID.
func (t *TenantModel) GetTenantID() int64 { return t.TenantID }

// SetTenantID sets the tenant ID.
func (t *TenantModel) SetTenantID(id int64) { t.TenantID = id }

// TenantScoped is the interface for models that carry a tenant_id.
type TenantScoped interface {
	GetTenantID() int64
	SetTenantID(id int64)
}
