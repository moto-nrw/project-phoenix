package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	enrollmentChangeRequestOriginVersion     = "1.15.197"
	enrollmentChangeRequestOriginDescription = "Distinguish parent proposals from admin enrollment data corrections"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     enrollmentChangeRequestOriginVersion,
		Description: enrollmentChangeRequestOriginDescription,
		DependsOn: []string{
			careOfferingAvailabilityRulesVersion,
			changeRequestCareOfferingCapabilityVersion,
		},
	})

	Migrations.MustRegister(enrollmentChangeRequestOriginUp, enrollmentChangeRequestOriginDown)
}

func enrollmentChangeRequestOriginUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.197: Adding enrollment change request origin...")
	_, err := db.NewRaw(`
		ALTER TABLE enrollment.change_requests
			ADD COLUMN IF NOT EXISTS origin TEXT NOT NULL DEFAULT 'parent';

		ALTER TABLE enrollment.change_requests
			DROP CONSTRAINT IF EXISTS chk_enrollment_change_requests_origin,
			ADD CONSTRAINT chk_enrollment_change_requests_origin
				CHECK (origin IN ('parent', 'admin'));
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed adding enrollment change request origin: %w", err)
	}
	return nil
}

func enrollmentChangeRequestOriginDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.197: Removing enrollment change request origin...")
	_, err := db.NewRaw(`
		ALTER TABLE enrollment.change_requests
			DROP CONSTRAINT IF EXISTS chk_enrollment_change_requests_origin,
			DROP COLUMN IF EXISTS origin;
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed removing enrollment change request origin: %w", err)
	}
	return nil
}
