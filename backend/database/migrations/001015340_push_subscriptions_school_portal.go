package migrations

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/uptrace/bun"
)

const (
	pushSubscriptionsSchoolPortalVersion     = "1.15.340"
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
	slog.Info("migration starting",
		slog.String("migration", pushSubscriptionsSchoolPortalVersion),
		slog.String("detail", "allowing portal 'school' on iot.push_subscriptions"),
	)

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

	slog.Info("migration completed",
		slog.String("migration", pushSubscriptionsSchoolPortalVersion),
		slog.String("detail", "portal 'school' allowed on iot.push_subscriptions"),
	)
	return nil
}

// pushSubscriptionsSchoolPortalDown narrows the table back to the two
// portals. The forward state legitimately holds one row per portal for the
// same (tenant_id, endpoint) — the same browser may be registered in the OGS
// and the school portal — so the old UNIQUE (tenant_id, endpoint) cannot be
// restored before those rows are reconciled: school rows are dropped with the
// portal, and of the remaining staff/parent duplicates the most recently
// touched registration survives. Registrations lost this way come back the
// next time the browser subscribes.
func pushSubscriptionsSchoolPortalDown(ctx context.Context, db *bun.DB) error {
	slog.Info("migration rolling back",
		slog.String("migration", pushSubscriptionsSchoolPortalVersion),
		slog.String("detail", "removing portal 'school' from iot.push_subscriptions"),
	)

	if _, err := db.ExecContext(ctx, `
		DELETE FROM iot.push_subscriptions WHERE portal = 'school'
	`); err != nil {
		return fmt.Errorf("deleting school push subscriptions: %w", err)
	}

	res, err := db.ExecContext(ctx, `
		DELETE FROM iot.push_subscriptions s
		USING iot.push_subscriptions keep
		WHERE s.tenant_id = keep.tenant_id
		  AND s.endpoint = keep.endpoint
		  AND (keep.updated_at, keep.id) > (s.updated_at, s.id)
	`)
	if err != nil {
		return fmt.Errorf("reconciling duplicate push subscriptions: %w", err)
	}
	if removed, err := res.RowsAffected(); err == nil && removed > 0 {
		slog.Warn("migration dropped duplicate push subscriptions",
			slog.String("migration", pushSubscriptionsSchoolPortalVersion),
			slog.Int64("rows", removed),
		)
	}

	if _, err := db.ExecContext(ctx, `
		ALTER TABLE iot.push_subscriptions
			DROP CONSTRAINT IF EXISTS chk_push_subscriptions_portal,
			ADD CONSTRAINT chk_push_subscriptions_portal
				CHECK (portal IN ('staff', 'parent')),
			DROP CONSTRAINT IF EXISTS uq_push_subscriptions_tenant_portal_endpoint,
			ADD CONSTRAINT uq_push_subscriptions_tenant_endpoint
				UNIQUE (tenant_id, endpoint)
	`); err != nil {
		return fmt.Errorf("restoring push subscription portal check: %w", err)
	}

	slog.Info("migration rolled back",
		slog.String("migration", pushSubscriptionsSchoolPortalVersion),
		slog.String("detail", "portal 'school' removed from iot.push_subscriptions"),
	)
	return nil
}
