package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

func init() {
	up := []string{
		`ALTER TABLE accounts ADD COLUMN password_hash text`,
	}

	down := []string{
		`ALTER TABLE accounts DROP COLUMN password_hash`,
	}

	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		fmt.Println("adding password_hash field to accounts table")
		for _, q := range up {
			_, err := db.Exec(q)
			if err != nil {
				return err
			}
		}
		return nil
	}, func(ctx context.Context, db *bun.DB) error {
		fmt.Println("removing password_hash field from accounts table")
		for _, q := range down {
			_, err := db.Exec(q)
			if err != nil {
				return err
			}
		}
		return nil
	})
}
