package audit

import "time"

// Model is the persistence-neutral identity/timestamp shape shared by audit
// read models that expose conventional created/updated timestamps.
type Model struct {
	ID        int64     `bun:"id,pk,autoincrement" json:"id"`
	CreatedAt time.Time `bun:"created_at,nullzero,notnull,default:current_timestamp" json:"created_at"`
	UpdatedAt time.Time `bun:"updated_at,nullzero,notnull,default:current_timestamp" json:"updated_at"`
}

// TenantModel keeps tenant attribution inside the Audit domain instead of
// coupling audit events to the transaction-runtime model package.
type TenantModel struct {
	TenantID int64 `bun:"tenant_id,notnull" json:"tenant_id"`
}

func (m *TenantModel) GetTenantID() int64   { return m.TenantID }
func (m *TenantModel) SetTenantID(id int64) { m.TenantID = id }
