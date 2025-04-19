package migrations

import (
	"context"
	"fmt"
	"github.com/moto-nrw/project-phoenix/models"

	"github.com/uptrace/bun"
)

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		fmt.Print(" [up migration] add custom_users, pedagogical_specialists, and devices tables...")

		// Create custom_users table
		_, err := db.NewCreateTable().
			Model((*models.CustomUser)(nil)).
			IfNotExists().
			Exec(ctx)
		if err != nil {
			return err
		}

		// Create pedagogical_specialists table
		_, err = db.NewCreateTable().
			Model((*models.PedagogicalSpecialist)(nil)).
			IfNotExists().
			ForeignKey(`("custom_user_id") REFERENCES "custom_users" ("id") ON DELETE CASCADE`).
			Exec(ctx)
		if err != nil {
			return err
		}

		// Create devices table
		_, err = db.NewCreateTable().
			Model((*models.Device)(nil)).
			IfNotExists().
			Exec(ctx)
		if err != nil {
			return err
		}

		fmt.Println(" done")
		return nil
	}, func(ctx context.Context, db *bun.DB) error {
		fmt.Print(" [down migration] drop custom_users, pedagogical_specialists, and devices tables...")

		// Drop tables in reverse order to handle foreign key constraints
		_, err := db.NewDropTable().
			Model((*models.Device)(nil)).
			IfExists().
			Cascade().
			Exec(ctx)
		if err != nil {
			return err
		}

		_, err = db.NewDropTable().
			Model((*models.PedagogicalSpecialist)(nil)).
			IfExists().
			Cascade().
			Exec(ctx)
		if err != nil {
			return err
		}

		_, err = db.NewDropTable().
			Model((*models.CustomUser)(nil)).
			IfExists().
			Cascade().
			Exec(ctx)
		if err != nil {
			return err
		}

		fmt.Println(" done")
		return nil
	})
}
