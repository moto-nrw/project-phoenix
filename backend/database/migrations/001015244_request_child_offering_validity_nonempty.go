package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	requestChildOfferingValidityNonemptyVersion     = "1.15.244"
	requestChildOfferingValidityNonemptyDescription = "Require non-empty effective intervals for child care-offering selections."
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     requestChildOfferingValidityNonemptyVersion,
		Description: requestChildOfferingValidityNonemptyDescription,
		DependsOn:   []string{requestChildOfferingValidityVersion},
	})

	Migrations.MustRegister(requestChildOfferingValidityNonemptyUp, requestChildOfferingValidityNonemptyDown)
}

func requestChildOfferingValidityNonemptyUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.244: Requiring non-empty child care-offering validity intervals...")
	if _, err := db.NewRaw(`
		ALTER TABLE enrollment.request_child_offerings
			ADD CONSTRAINT request_child_offerings_nonempty_validity
			CHECK (valid_from IS NULL OR valid_until IS NULL OR valid_from < valid_until);
	`).Exec(ctx); err != nil {
		return fmt.Errorf("failed requiring non-empty request child offering validity: %w", err)
	}
	return nil
}

func requestChildOfferingValidityNonemptyDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.244: Allowing empty child care-offering validity intervals...")
	if _, err := db.NewRaw(`
		ALTER TABLE enrollment.request_child_offerings
			DROP CONSTRAINT IF EXISTS request_child_offerings_nonempty_validity;
	`).Exec(ctx); err != nil {
		return fmt.Errorf("failed removing request child offering non-empty validity constraint: %w", err)
	}
	return nil
}
