package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	SeedFirstTraegerVersion     = "1.8.1"
	SeedFirstTraegerDescription = "Create first Träger and link to existing organizations"
)

// firstTraegerID is a fixed ID for the first Träger, allowing reliable referencing
const firstTraegerID = "first-traeger"

func init() {
	MigrationRegistry[SeedFirstTraegerVersion] = &Migration{
		Version:     SeedFirstTraegerVersion,
		Description: SeedFirstTraegerDescription,
		DependsOn:   []string{"1.8.0"}, // Depends on tenant tables existing
	}

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return seedFirstTraeger(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return unseedFirstTraeger(ctx, db)
		},
	)
}

func seedFirstTraeger(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.8.1: Creating first Träger...")

	// Check if any traeger already exists
	var count int
	err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM tenant.traeger`).Scan(&count)
	if err != nil {
		return fmt.Errorf("error checking traeger: %w", err)
	}

	if count > 0 {
		fmt.Println("  Träger already exists, skipping creation")
		return nil
	}

	// Create the first Träger with a fixed ID for reliable referencing
	// Using string formatting because the ID is a known constant value
	query := fmt.Sprintf(`
		INSERT INTO tenant.traeger (id, name, contact_email)
		VALUES ('%s', 'Erster Träger', 'admin@example.com')
		ON CONFLICT (id) DO NOTHING
	`, firstTraegerID)
	_, err = db.ExecContext(ctx, query)
	if err != nil {
		return fmt.Errorf("error creating first Träger: %w", err)
	}
	fmt.Printf("  Created Träger with ID: %s\n", firstTraegerID)

	// Find all distinct traegerId values currently in use in public.organization
	// This handles placeholder values like 'migration-placeholder-traeger',
	// as well as test values like 'traeger-001', 'traeger-123', etc.
	var traegerIds []string
	err = db.NewSelect().
		TableExpr("public.organization").
		Column("traegerId").
		Distinct().
		Where(`"traegerId" IS NOT NULL AND "traegerId" != ''`).
		Scan(ctx, &traegerIds)
	if err != nil {
		return fmt.Errorf("error finding existing traegerId values: %w", err)
	}

	fmt.Printf("  Found %d distinct traegerId values in organizations: %v\n", len(traegerIds), traegerIds)

	// Update ALL organizations to use the first real Träger
	// This consolidates any placeholder or test traegerId values
	updateQuery := fmt.Sprintf(`
		UPDATE public.organization
		SET "traegerId" = '%s'
		WHERE "traegerId" != '%s' OR "traegerId" IS NULL
	`, firstTraegerID, firstTraegerID)
	result, err := db.ExecContext(ctx, updateQuery)
	if err != nil {
		return fmt.Errorf("error updating organizations: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	fmt.Printf("  Updated %d organizations to use real Träger ID\n", rowsAffected)

	fmt.Println("Migration 1.8.1: Successfully created first Träger")
	return nil
}

func unseedFirstTraeger(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.8.1...")

	// NOTE: Rollback is imperfect because we don't know what the original
	// traegerId values were. The safest approach is to restore a placeholder
	// value so organizations remain valid.
	restoreQuery := fmt.Sprintf(`
		UPDATE public.organization
		SET "traegerId" = 'migration-placeholder-traeger'
		WHERE "traegerId" = '%s'
	`, firstTraegerID)
	_, err := db.ExecContext(ctx, restoreQuery)
	if err != nil {
		return fmt.Errorf("error restoring placeholder: %w", err)
	}
	fmt.Println("  Restored placeholder traegerId in organizations")

	// Now we can delete the traeger
	deleteQuery := fmt.Sprintf(`DELETE FROM tenant.traeger WHERE id = '%s'`, firstTraegerID)
	_, err = db.ExecContext(ctx, deleteQuery)
	if err != nil {
		return fmt.Errorf("error deleting first Träger: %w", err)
	}
	fmt.Println("  Deleted first Träger")

	fmt.Println("Migration 1.8.1: Successfully rolled back")
	return nil
}
