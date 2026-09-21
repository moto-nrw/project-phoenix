package test

import (
	"context"
	"time"

	"github.com/uptrace/bun"
)

// AccountTenantFixture is persistence-only setup data, not a serving model.
type AccountTenantFixture struct {
	bun.BaseModel `bun:"table:auth.account_tenants,alias:account_tenant"`
	ID            int64      `bun:"id,pk,autoincrement"`
	CreatedAt     time.Time  `bun:"created_at,notnull,default:current_timestamp"`
	UpdatedAt     time.Time  `bun:"updated_at,notnull,default:current_timestamp"`
	AccountID     int64      `bun:"account_id,notnull"`
	TenantID      int64      `bun:"tenant_id,notnull"`
	Status        string     `bun:"status,notnull,default:'active'"`
	InvitedAt     *time.Time `bun:"invited_at"`
	ActivatedAt   *time.Time `bun:"activated_at"`
	DeactivatedAt *time.Time `bun:"deactivated_at"`
}

// ActiveAccountTenantExists inspects persisted membership state for assertions,
// independently of the serving Identity capability.
func ActiveAccountTenantExists(ctx context.Context, db bun.IDB, accountID, tenantID int64) (bool, error) {
	var active bool
	err := db.NewRaw(`SELECT EXISTS (SELECT 1 FROM auth.account_tenants
		WHERE account_id = ? AND tenant_id = ? AND status = 'active')`, accountID, tenantID).Scan(ctx, &active)
	return active, err
}
