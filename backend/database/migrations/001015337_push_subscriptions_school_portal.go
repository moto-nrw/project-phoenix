package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	pushSubscriptionsSchoolPortalVersion     = "1.15.337"
	pushSubscriptionsSchoolPortalDescription = "Allow 'school' as push subscription portal for moto schule (#2208)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     pushSubscriptionsSchoolPortalVersion,
		Description: pushSubscriptionsSchoolPortalDescription,
		DependsOn:   []string{"1.15.234"},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return pushSubscriptionsSchoolPortalUp(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return pushSubscriptionsSchoolPortalDown(ctx, db)
		},
	)
}

// pushSubscriptionsSchoolPortalUp widens the portal check so a device
// registered through the school portal is recorded as such (#2208). The
// portal decides which deep link a push carries (the school host has its own
// routes) and which subscriptions a logout on that portal revokes — a
// Lehrkraft signing out of "moto schule" must not silently unregister the
// OGS devices of the same account.
func pushSubscriptionsSchoolPortalUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.337: allowing portal 'school' on iot.push_subscriptions...")

	_, err := db.ExecContext(ctx, `
		ALTER TABLE iot.push_subscriptions
			DROP CONSTRAINT IF EXISTS chk_push_subscriptions_portal,
			ADD CONSTRAINT chk_push_subscriptions_portal
				CHECK (portal IN ('staff', 'parent', 'school')),
			DROP CONSTRAINT IF EXISTS uq_push_subscriptions_tenant_endpoint,
			ADD CONSTRAINT uq_push_subscriptions_tenant_portal_endpoint
				UNIQUE (tenant_id, portal, endpoint)
	`)
	if err != nil {
		return fmt.Errorf("widening push subscription portal check: %w", err)
	}
	return nil
}

func pushSubscriptionsSchoolPortalDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.337: removing portal 'school' from iot.push_subscriptions...")

	_, err := db.ExecContext(ctx, `
		DELETE FROM iot.push_subscriptions WHERE portal = 'school';
		ALTER TABLE iot.push_subscriptions
			DROP CONSTRAINT IF EXISTS chk_push_subscriptions_portal,
			ADD CONSTRAINT chk_push_subscriptions_portal
				CHECK (portal IN ('staff', 'parent')),
			DROP CONSTRAINT IF EXISTS uq_push_subscriptions_tenant_portal_endpoint,
			ADD CONSTRAINT uq_push_subscriptions_tenant_endpoint
				UNIQUE (tenant_id, endpoint)
	`)
	if err != nil {
		return fmt.Errorf("restoring push subscription portal check: %w", err)
	}
	return nil
}
