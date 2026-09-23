package test

import "time"

// RFIDCardFixture is test data, not a runtime repository or public card model.
type RFIDCardFixture struct {
	ID        string    `bun:"id,pk"`
	TenantID  int64     `bun:"tenant_id,notnull"`
	CreatedAt time.Time `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt time.Time `bun:"updated_at,nullzero,notnull,default:current_timestamp"`
	Active    bool      `bun:"active,notnull,default:true"`
}
