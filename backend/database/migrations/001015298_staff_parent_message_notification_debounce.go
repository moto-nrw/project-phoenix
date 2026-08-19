package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	staffParentMessageNotificationDebounceVersion     = "1.15.298"
	staffParentMessageNotificationDebounceDescription = "Debounce staff notifications for parent messages (#2363)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     staffParentMessageNotificationDebounceVersion,
		Description: staffParentMessageNotificationDebounceDescription,
		DependsOn:   []string{parentMessageTeamHandledVersion},
	})
	Migrations.MustRegister(staffParentMessageNotificationDebounceUp, staffParentMessageNotificationDebounceDown)
}

func staffParentMessageNotificationDebounceUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.298: Adding the staff parent-message notification debounce claim...")
	_, err := db.ExecContext(ctx, `
		ALTER TABLE users.parent_message_threads
			ADD COLUMN IF NOT EXISTS last_staff_message_notification_at TIMESTAMPTZ;
	`)
	if err != nil {
		return fmt.Errorf("add staff parent-message notification debounce claim: %w", err)
	}
	return nil
}

func staffParentMessageNotificationDebounceDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back 1.15.298: Removing the staff parent-message notification debounce claim...")
	_, err := db.ExecContext(ctx, `
		ALTER TABLE users.parent_message_threads
			DROP COLUMN IF EXISTS last_staff_message_notification_at;
	`)
	if err != nil {
		return fmt.Errorf("remove staff parent-message notification debounce claim: %w", err)
	}
	return nil
}
