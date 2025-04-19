package migrations

import (
	"context"
	"fmt"
	"github.com/moto-nrw/project-phoenix/models"

	"github.com/uptrace/bun"
)

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		fmt.Print(" [up migration] add timespan table...")
		if _, err := db.NewCreateTable().
			Model((*models.Timespan)(nil)).
			IfNotExists().
			Exec(ctx); err != nil {
			return err
		}

		// Add foreign key to room_occupancy table
		fmt.Print(" [up migration] add timespan foreign key to room_occupancy table...")
		_, err := db.ExecContext(
			ctx,
			`ALTER TABLE room_occupancies 
			 ADD CONSTRAINT fk_room_occupancies_timespan 
			 FOREIGN KEY (timespan_id) REFERENCES timespans (id) 
			 ON DELETE CASCADE`,
		)
		if err != nil {
			return err
		}

		return nil
	}, func(ctx context.Context, db *bun.DB) error {
		fmt.Print(" [down migration] remove timespan foreign key from room_occupancy table...")
		_, err := db.ExecContext(
			ctx,
			`ALTER TABLE room_occupancies 
			 DROP CONSTRAINT IF EXISTS fk_room_occupancies_timespan`,
		)
		if err != nil {
			return err
		}

		fmt.Print(" [down migration] drop timespan table...")
		if _, err := db.NewDropTable().
			Model((*models.Timespan)(nil)).
			IfExists().
			Exec(ctx); err != nil {
			return err
		}

		return nil
	})
}
