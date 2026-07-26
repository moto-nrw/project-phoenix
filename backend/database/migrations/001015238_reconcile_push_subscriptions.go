package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	reconcilePushSubscriptionsVersion     = "1.15.238"
	reconcilePushSubscriptionsDescription = "Ensure iot.push_subscriptions exists after the 1.15.230 migration version collision"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     reconcilePushSubscriptionsVersion,
		Description: reconcilePushSubscriptionsDescription,
		DependsOn: []string{
			pushSubscriptionsVersion,
			requestChildrenMatchedStudentVersion,
			phaseEligibleGradeLevelsVersion,
		},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return reconcilePushSubscriptionsUp(ctx, db)
		},
		func(ctx context.Context, _ *bun.DB) error {
			return reconcilePushSubscriptionsDown(ctx)
		},
	)
}

func reconcilePushSubscriptionsUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.238: Reconciling iot.push_subscriptions after the 1.15.230 version collision...")
	return ensurePushSubscriptions(ctx, db)
}

func reconcilePushSubscriptionsDown(_ context.Context) error {
	fmt.Println("Rolling back migration 1.15.238: Leaving iot.push_subscriptions owned by migration 1.15.230...")
	return nil
}
