package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	changeRequestCareOfferingCapabilityVersion     = "1.15.179"
	changeRequestCareOfferingCapabilityDescription = "Pin care-offering capability on enrollment change requests"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     changeRequestCareOfferingCapabilityVersion,
		Description: changeRequestCareOfferingCapabilityDescription,
		DependsOn:   []string{addEmailOutboxIdempotencyVersion},
	})
	Migrations.MustRegister(changeRequestCareOfferingCapabilityUp, changeRequestCareOfferingCapabilityDown)
}

func changeRequestCareOfferingCapabilityUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.179: Pinning care-offering capability on enrollment change requests...")
	if _, err := db.NewRaw(`
		ALTER TABLE enrollment.change_requests
			ADD COLUMN IF NOT EXISTS care_offerings_enabled_at_creation BOOLEAN NOT NULL DEFAULT TRUE;
	`).Exec(ctx); err != nil {
		return fmt.Errorf("failed adding enrollment change-request care-offering capability: %w", err)
	}
	return nil
}

func changeRequestCareOfferingCapabilityDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.179: Removing enrollment change-request care-offering capability...")
	if _, err := db.NewRaw(`
		ALTER TABLE enrollment.change_requests
			DROP COLUMN IF EXISTS care_offerings_enabled_at_creation;
	`).Exec(ctx); err != nil {
		return fmt.Errorf("failed removing enrollment change-request care-offering capability: %w", err)
	}
	return nil
}
