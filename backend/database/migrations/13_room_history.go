package migrations

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/models"
	"github.com/uptrace/bun"
)

func init() {
	// Migration 13: Add room_history table
	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			fmt.Println("Migration 13: Creating room_history table...")

			// Create the room_history table
			_, err := db.NewCreateTable().
				Model((*models.RoomHistory)(nil)).
				IfNotExists().
				Exec(ctx)
			if err != nil {
				return fmt.Errorf("failed to create room_history table: %w", err)
			}

			// Create indexes
			_, err = db.NewCreateIndex().
				Model((*models.RoomHistory)(nil)).
				Index("room_history_room_id_idx").
				Column("room_id").
				IfNotExists().
				Exec(ctx)
			if err != nil {
				return fmt.Errorf("failed to create index on room_id: %w", err)
			}

			_, err = db.NewCreateIndex().
				Model((*models.RoomHistory)(nil)).
				Index("room_history_day_idx").
				Column("day").
				IfNotExists().
				Exec(ctx)
			if err != nil {
				return fmt.Errorf("failed to create index on day: %w", err)
			}

			_, err = db.NewCreateIndex().
				Model((*models.RoomHistory)(nil)).
				Index("room_history_supervisor_id_idx").
				Column("supervisor_id").
				IfNotExists().
				Exec(ctx)
			if err != nil {
				return fmt.Errorf("failed to create index on supervisor_id: %w", err)
			}

			return nil
		},
		// Down migration
		func(ctx context.Context, db *bun.DB) error {
			fmt.Println("Rolling back migration 13: Dropping room_history table...")

			_, err := db.NewDropTable().
				Model((*models.RoomHistory)(nil)).
				IfExists().
				Cascade().
				Exec(ctx)

			return err
		},
	)
}
