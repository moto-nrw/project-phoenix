package schedule

import "time"

// Model and TenantModel keep the legacy schedule DTOs structurally compatible
// while their callers move to the Timetable facade. New owner domain values do
// not use these persistence shapes.
type Model struct {
	ID        int64     `bun:"id,pk,autoincrement" json:"id"`
	CreatedAt time.Time `bun:"created_at,nullzero,notnull,default:current_timestamp" json:"created_at"`
	UpdatedAt time.Time `bun:"updated_at,nullzero,notnull,default:current_timestamp" json:"updated_at"`
}

type TenantModel struct {
	TenantID int64 `bun:"tenant_id,notnull" json:"tenant_id"`
}

func (m *TenantModel) GetTenantID() int64   { return m.TenantID }
func (m *TenantModel) SetTenantID(id int64) { m.TenantID = id }
