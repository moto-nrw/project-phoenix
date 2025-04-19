package migrations

import (
	"context"
	"fmt"
	"github.com/moto-nrw/project-phoenix/api/room"
	"github.com/moto-nrw/project-phoenix/models"

	"github.com/uptrace/bun"
)

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		fmt.Print(" [up migration] add room table...")
		if _, err := db.NewCreateTable().
			Model((*models.Room)(nil)).
			IfNotExists().
			Exec(ctx); err != nil {
			return err
		}

		fmt.Print(" [up migration] add room_occupancy table...")
		if _, err := db.NewCreateTable().
			Model((*room.RoomOccupancy)(nil)).
			IfNotExists().
			ForeignKey(`("room_id") REFERENCES "rooms" ("id") ON DELETE CASCADE`).
			Exec(ctx); err != nil {
			return err
		}

		fmt.Print(" [up migration] add room_occupancy_supervisor table...")
		if _, err := db.NewCreateTable().
			Model((*room.RoomOccupancySupervisor)(nil)).
			IfNotExists().
			ForeignKey(`("room_occupancy_id") REFERENCES "room_occupancies" ("id") ON DELETE CASCADE`).
			// Will need a foreign key to the pedagogical_specialist table when it's created
			Exec(ctx); err != nil {
			return err
		}

		return nil
	}, func(ctx context.Context, db *bun.DB) error {
		fmt.Print(" [down migration] drop room_occupancy_supervisor table...")
		if _, err := db.NewDropTable().
			Model((*room.RoomOccupancySupervisor)(nil)).
			IfExists().
			Exec(ctx); err != nil {
			return err
		}

		fmt.Print(" [down migration] drop room_occupancy table...")
		if _, err := db.NewDropTable().
			Model((*room.RoomOccupancy)(nil)).
			IfExists().
			Exec(ctx); err != nil {
			return err
		}

		fmt.Print(" [down migration] drop room table...")
		if _, err := db.NewDropTable().
			Model((*models.Room)(nil)).
			IfExists().
			Exec(ctx); err != nil {
			return err
		}

		return nil
	})
}
