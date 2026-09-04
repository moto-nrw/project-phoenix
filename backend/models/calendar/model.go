package calendar

import "time"

// Model contains the persistence metadata shared by calendar-owned records.
type Model struct {
	ID        int64     `bun:"id,pk,autoincrement" json:"id"`
	CreatedAt time.Time `bun:"created_at,nullzero,notnull,default:current_timestamp" json:"created_at"`
	UpdatedAt time.Time `bun:"updated_at,nullzero,notnull,default:current_timestamp" json:"updated_at"`
}

// TenantModel carries the school boundary for calendar-owned records.
type TenantModel struct {
	TenantID int64 `bun:"tenant_id,notnull" json:"tenant_id"`
}
