package migrations

import (
	"context"
	"fmt"
	models2 "github.com/moto-nrw/project-phoenix/models"

	"github.com/uptrace/bun"
)

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		fmt.Print(" [up migration] add group, student, visit, and feedback tables...")

		// Register models with junction tables for many-to-many relationships
		db.RegisterModel((*models2.GroupSupervisor)(nil))
		db.RegisterModel((*models2.CombinedGroupGroup)(nil))
		db.RegisterModel((*models2.CombinedGroupSpecialist)(nil))

		// Create groups table
		_, err := db.NewCreateTable().
			Model((*models2.Group)(nil)).
			IfNotExists().
			ForeignKey(`("room_id") REFERENCES "rooms" ("id") ON DELETE SET NULL`).
			ForeignKey(`("representative_id") REFERENCES "pedagogical_specialists" ("id") ON DELETE SET NULL`).
			Exec(ctx)
		if err != nil {
			return err
		}

		// Create group_supervisors junction table
		_, err = db.NewCreateTable().
			Model((*models2.GroupSupervisor)(nil)).
			IfNotExists().
			ForeignKey(`("group_id") REFERENCES "groups" ("id") ON DELETE CASCADE`).
			ForeignKey(`("specialist_id") REFERENCES "pedagogical_specialists" ("id") ON DELETE CASCADE`).
			Exec(ctx)
		if err != nil {
			return err
		}

		// Create combined_groups table
		_, err = db.NewCreateTable().
			Model((*models2.CombinedGroup)(nil)).
			IfNotExists().
			ForeignKey(`("specific_group_id") REFERENCES "groups" ("id") ON DELETE SET NULL`).
			Exec(ctx)
		if err != nil {
			return err
		}

		// Create combined_group_groups junction table
		_, err = db.NewCreateTable().
			Model((*models2.CombinedGroupGroup)(nil)).
			IfNotExists().
			ForeignKey(`("combinedgroup_id") REFERENCES "combined_groups" ("id") ON DELETE CASCADE`).
			ForeignKey(`("group_id") REFERENCES "groups" ("id") ON DELETE CASCADE`).
			Exec(ctx)
		if err != nil {
			return err
		}

		// Create combined_group_specialists junction table
		_, err = db.NewCreateTable().
			Model((*models2.CombinedGroupSpecialist)(nil)).
			IfNotExists().
			ForeignKey(`("combinedgroup_id") REFERENCES "combined_groups" ("id") ON DELETE CASCADE`).
			ForeignKey(`("specialist_id") REFERENCES "pedagogical_specialists" ("id") ON DELETE CASCADE`).
			Exec(ctx)
		if err != nil {
			return err
		}

		// Create students table
		_, err = db.NewCreateTable().
			Model((*models2.Student)(nil)).
			IfNotExists().
			ForeignKey(`("custom_user_id") REFERENCES "custom_users" ("id") ON DELETE CASCADE`).
			ForeignKey(`("group_id") REFERENCES "groups" ("id") ON DELETE CASCADE`).
			Exec(ctx)
		if err != nil {
			return err
		}

		// Create visits table
		_, err = db.NewCreateTable().
			Model((*models2.Visit)(nil)).
			IfNotExists().
			ForeignKey(`("student_id") REFERENCES "students" ("id") ON DELETE CASCADE`).
			ForeignKey(`("room_id") REFERENCES "rooms" ("id") ON DELETE CASCADE`).
			ForeignKey(`("timespan_id") REFERENCES "timespans" ("id") ON DELETE CASCADE`).
			Exec(ctx)
		if err != nil {
			return err
		}

		// Create feedback table
		_, err = db.NewCreateTable().
			Model((*models2.Feedback)(nil)).
			IfNotExists().
			ForeignKey(`("student_id") REFERENCES "students" ("id") ON DELETE CASCADE`).
			Exec(ctx)
		if err != nil {
			return err
		}

		fmt.Println(" done")
		return nil
	}, func(ctx context.Context, db *bun.DB) error {
		fmt.Print(" [down migration] drop group and student related tables...")

		// Drop tables in reverse order to handle foreign key constraints
		_, err := db.NewDropTable().
			Model((*models2.Feedback)(nil)).
			IfExists().
			Cascade().
			Exec(ctx)
		if err != nil {
			return err
		}

		_, err = db.NewDropTable().
			Model((*models2.Visit)(nil)).
			IfExists().
			Cascade().
			Exec(ctx)
		if err != nil {
			return err
		}

		_, err = db.NewDropTable().
			Model((*models2.Student)(nil)).
			IfExists().
			Cascade().
			Exec(ctx)
		if err != nil {
			return err
		}

		_, err = db.NewDropTable().
			Model((*models2.CombinedGroupSpecialist)(nil)).
			IfExists().
			Cascade().
			Exec(ctx)
		if err != nil {
			return err
		}

		_, err = db.NewDropTable().
			Model((*models2.CombinedGroupGroup)(nil)).
			IfExists().
			Cascade().
			Exec(ctx)
		if err != nil {
			return err
		}

		_, err = db.NewDropTable().
			Model((*models2.CombinedGroup)(nil)).
			IfExists().
			Cascade().
			Exec(ctx)
		if err != nil {
			return err
		}

		_, err = db.NewDropTable().
			Model((*models2.GroupSupervisor)(nil)).
			IfExists().
			Cascade().
			Exec(ctx)
		if err != nil {
			return err
		}

		_, err = db.NewDropTable().
			Model((*models2.Group)(nil)).
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
