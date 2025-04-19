package migrations

import (
	"context"
	"fmt"
	"github.com/moto-nrw/project-phoenix/api/rfid"

	"github.com/uptrace/bun"
)

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		fmt.Print(" [up migration] add rfid_tags table...")
		if _, err := db.NewCreateTable().
			Model((*rfid.Tag)(nil)).
			IfNotExists().
			Exec(ctx); err != nil {
			return err
		}
		return nil
	}, func(ctx context.Context, db *bun.DB) error {
		fmt.Print(" [down migration] drop rfid_tags table...")
		if _, err := db.NewDropTable().
			Model((*rfid.Tag)(nil)).
			IfExists().
			Exec(ctx); err != nil {
			return err
		}
		return nil
	})
}
