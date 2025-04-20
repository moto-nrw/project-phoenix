package migrations

import (
	"context"
	"errors"
	"fmt"
	"github.com/moto-nrw/project-phoenix/auth/userpass"
	"github.com/spf13/viper"
	"github.com/uptrace/bun"
)

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		fmt.Println("adding admin user with password")

		// Get admin password from environment or config
		adminPassword := viper.GetString("admin_password")
		if adminPassword == "" {
			return errors.New("ADMIN_PASSWORD environment variable must be set to create the admin user")
		}

		// Get admin email from environment or config
		adminEmail := viper.GetString("admin_email")
		if adminEmail == "" {
			return errors.New("ADMIN_EMAIL environment variable must be set to create the admin user")
		}

		// Generate password hash for admin user
		passwordHash, err := userpass.HashPassword(adminPassword, userpass.DefaultParams())
		if err != nil {
			return err
		}

		// Check if admin user already exists
		exists, err := db.NewSelect().
			Model((*userpass.Account)(nil)).
			Where("email = ?", adminEmail).
			Exists(ctx)
		if err != nil {
			return err
		}

		// Only add the admin user if it doesn't already exist
		if !exists {
			_, err = db.NewInsert().
				Model(&userpass.Account{
					Email:        adminEmail,
					Name:         "Admin User",
					Active:       true,
					Roles:        []string{"admin", "user"},
					PasswordHash: passwordHash,
				}).
				Exec(ctx)
			if err != nil {
				return err
			}
			fmt.Printf("Added admin user: %s\n", adminEmail)
		} else {
			fmt.Printf("Admin user %s already exists, skipping\n", adminEmail)
		}

		return nil
	}, func(ctx context.Context, db *bun.DB) error {
		// Get admin email from environment or config (for down migration)
		adminEmail := viper.GetString("admin_email")
		if adminEmail == "" {
			return errors.New("ADMIN_EMAIL environment variable must be set to remove the admin user")
		}

		fmt.Printf("removing admin user: %s\n", adminEmail)
		_, err := db.NewDelete().
			Model((*userpass.Account)(nil)).
			Where("email = ?", adminEmail).
			Exec(ctx)
		return err
	})
}
