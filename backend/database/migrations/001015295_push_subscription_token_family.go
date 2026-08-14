package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	pushSubscriptionTokenFamilyVersion     = "1.15.295"
	pushSubscriptionTokenFamilyDescription = "Bind Web Push subscriptions to the refresh-token family that registered them"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     pushSubscriptionTokenFamilyVersion,
		Description: pushSubscriptionTokenFamilyDescription,
		DependsOn:   []string{pushSubscriptionsVersion, authTokenPortalScopeVersion},
	})
	Migrations.MustRegister(pushSubscriptionTokenFamilyUp, pushSubscriptionTokenFamilyDown)
}

func pushSubscriptionTokenFamilyUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.295: Binding push subscriptions to token families...")
	_, err := db.ExecContext(ctx, `
		ALTER TABLE iot.push_subscriptions
			ADD COLUMN IF NOT EXISTS token_family_id TEXT NOT NULL DEFAULT '';
		COMMENT ON COLUMN iot.push_subscriptions.token_family_id IS
			'Refresh-token family that registered this device; empty on pre-1.15.295 rows';
		CREATE INDEX IF NOT EXISTS idx_push_subscriptions_token_family
			ON iot.push_subscriptions (account_id, token_family_id)
			WHERE token_family_id <> '';
	`)
	if err != nil {
		return fmt.Errorf("add push subscription token family: %w", err)
	}
	return nil
}

func pushSubscriptionTokenFamilyDown(ctx context.Context, db *bun.DB) error {
	_, err := db.ExecContext(ctx, `
		DROP INDEX IF EXISTS iot.idx_push_subscriptions_token_family;
		ALTER TABLE iot.push_subscriptions
			DROP COLUMN IF EXISTS token_family_id;
	`)
	if err != nil {
		return fmt.Errorf("drop push subscription token family: %w", err)
	}
	return nil
}
