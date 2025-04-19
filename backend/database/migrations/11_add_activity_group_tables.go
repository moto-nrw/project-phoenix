package migrations

import (
	"context"
	"fmt"
	"github.com/moto-nrw/project-phoenix/models"

	"github.com/uptrace/bun"
)

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		fmt.Print(" [up migration] add activity group tables...")

		// Register models with junction tables
		db.RegisterModel((*models.StudentAg)(nil))

		// Create ag_categories table
		_, err := db.NewCreateTable().
			Model((*models.AgCategory)(nil)).
			IfNotExists().
			Exec(ctx)
		if err != nil {
			return err
		}

		// Create ags table
		_, err = db.NewCreateTable().
			Model((*models.Ag)(nil)).
			IfNotExists().
			ForeignKey(`("supervisor_id") REFERENCES "pedagogical_specialists" ("id") ON DELETE CASCADE`).
			ForeignKey(`("ag_category_id") REFERENCES "ag_categories" ("id") ON DELETE CASCADE`).
			ForeignKey(`("datespan_id") REFERENCES "timespans" ("id") ON DELETE SET NULL`).
			Exec(ctx)
		if err != nil {
			return err
		}

		// Create ag_times table
		_, err = db.NewCreateTable().
			Model((*models.AgTime)(nil)).
			IfNotExists().
			ForeignKey(`("timespan_id") REFERENCES "timespans" ("id") ON DELETE CASCADE`).
			ForeignKey(`("ag_id") REFERENCES "ags" ("id") ON DELETE CASCADE`).
			Exec(ctx)
		if err != nil {
			return err
		}

		// Create student_ags junction table
		_, err = db.NewCreateTable().
			Model((*models.StudentAg)(nil)).
			IfNotExists().
			ForeignKey(`("student_id") REFERENCES "students" ("id") ON DELETE CASCADE`).
			ForeignKey(`("ag_id") REFERENCES "ags" ("id") ON DELETE CASCADE`).
			Exec(ctx)
		if err != nil {
			return err
		}

		fmt.Println(" done")
		return nil
	}, func(ctx context.Context, db *bun.DB) error {
		fmt.Print(" [down migration] drop activity group tables...")

		// Drop tables in reverse order
		_, err := db.NewDropTable().
			Model((*models.StudentAg)(nil)).
			IfExists().
			Cascade().
			Exec(ctx)
		if err != nil {
			return err
		}

		_, err = db.NewDropTable().
			Model((*models.AgTime)(nil)).
			IfExists().
			Cascade().
			Exec(ctx)
		if err != nil {
			return err
		}

		_, err = db.NewDropTable().
			Model((*models.Ag)(nil)).
			IfExists().
			Cascade().
			Exec(ctx)
		if err != nil {
			return err
		}

		_, err = db.NewDropTable().
			Model((*models.AgCategory)(nil)).
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
