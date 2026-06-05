package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	operatorEmailKontaktMotoVersion     = "1.15.100"
	operatorEmailKontaktMotoDescription = "Move bootstrap operator email to kontakt@moto.nrw"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     operatorEmailKontaktMotoVersion,
		Description: operatorEmailKontaktMotoDescription,
		DependsOn:   []string{platformOperatorMFAVersion},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			fmt.Println("Migration 1.15.100: Moving bootstrap operator email to kontakt@moto.nrw...")

			if _, err := db.NewRaw(`
				UPDATE platform.operators
				SET
					email = 'kontakt@moto.nrw',
					updated_at = NOW()
				WHERE lower(email) IN ('operator@example.com', 'operator@moto-app.de')
					AND NOT EXISTS (
						SELECT 1
						FROM platform.operators existing
						WHERE lower(existing.email) = 'kontakt@moto.nrw'
					);
			`).Exec(ctx); err != nil {
				return fmt.Errorf("failed moving bootstrap operator email: %w", err)
			}

			return nil
		},
		func(ctx context.Context, db *bun.DB) error {
			fmt.Println("Rolling back migration 1.15.100: no-op for bootstrap operator email move...")
			return nil
		},
	)
}
