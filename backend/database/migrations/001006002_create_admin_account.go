package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"os"

	"github.com/moto-nrw/project-phoenix/auth/userpass"
	"github.com/uptrace/bun"
)

const (
	AdminAccountVersion     = "1.6.2"
	AdminAccountDescription = "Create default admin account"
)

func init() {
	// Register migration with explicit version
	MigrationRegistry[AdminAccountVersion] = &Migration{
		Version:     AdminAccountVersion,
		Description: AdminAccountDescription,
		DependsOn:   []string{"1.0.1", "1.0.4", "1.0.7"}, // auth.accounts, roles, account_roles
	}

	// Register the actual migration functions
	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return createAdminAccount(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return dropAdminAccount(ctx, db)
		},
	)
}

// createAdminAccount creates the default admin account
func createAdminAccount(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.6.2: Creating default admin account...")

	// Begin a transaction for atomicity
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if rbErr := tx.Rollback(); rbErr != nil && err == nil {
			err = rbErr
		}
	}()

	// Create admin account - read from environment variables
	adminEmail := os.Getenv("ADMIN_EMAIL")
	if adminEmail == "" {
		adminEmail = "admin@example.com" // Fallback default
		fmt.Printf("WARNING: ADMIN_EMAIL environment variable not set, using default: %s\n", adminEmail)
	}

	adminUsername := "admin"
	adminPassword := os.Getenv("ADMIN_PASSWORD")
	if adminPassword == "" {
		adminPassword = "Test1234%" // Fallback default
		fmt.Printf("WARNING: ADMIN_PASSWORD environment variable not set, using default password!\n")
		fmt.Printf("WARNING: Please set ADMIN_PASSWORD environment variable for security!\n")
	}

	// Hash the password
	hashedPassword, err := userpass.HashPassword(adminPassword, userpass.DefaultParams())
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}

	// Insert the admin account
	var accountID int64
	err = tx.NewRaw(`
		INSERT INTO auth.accounts (email, username, active, password_hash, last_login)
		VALUES (?, ?, true, ?, NOW())
		ON CONFLICT (username) DO NOTHING
		RETURNING id
	`, adminEmail, adminUsername, hashedPassword).Scan(ctx, &accountID)

	if err != nil && err != sql.ErrNoRows {
		return fmt.Errorf("failed to create admin account: %w", err)
	}

	// If account already exists, get its ID
	if accountID == 0 {
		err = tx.NewRaw(`
			SELECT id FROM auth.accounts WHERE username = ?
		`, adminUsername).Scan(ctx, &accountID)

		if err != nil {
			return fmt.Errorf("failed to get admin account ID: %w", err)
		}
	}

	// Get admin role ID
	var adminRoleID int64
	err = tx.NewRaw(`
		SELECT id FROM auth.roles WHERE name = ?
	`, "admin").Scan(ctx, &adminRoleID)

	if err != nil {
		return fmt.Errorf("failed to get admin role ID: %w", err)
	}

	// Assign admin role to the account
	_, err = tx.ExecContext(ctx, `
		INSERT INTO auth.account_roles (account_id, role_id)
		VALUES (?, ?)
		ON CONFLICT (account_id, role_id) DO NOTHING
	`, accountID, adminRoleID)

	if err != nil {
		return fmt.Errorf("failed to assign admin role: %w", err)
	}

	// Commit the transaction
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	// Log admin account creation securely
	fmt.Printf("\n=== Admin Account Created ===\n")
	fmt.Printf("Username: %s\n", adminUsername)
	fmt.Printf("Email: %s\n", adminEmail)

	// Only show password if using default fallback
	if adminPassword == "Test1234%" {
		fmt.Printf("Password: %s (DEFAULT - CHANGE IMMEDIATELY!)\n", adminPassword)
		fmt.Printf("WARNING: Using default password! Set ADMIN_PASSWORD environment variable!\n")
	} else {
		fmt.Printf("Password: Set via ADMIN_PASSWORD environment variable\n")
		fmt.Printf("Please ensure you have recorded the password securely.\n")
	}

	fmt.Printf("Please change this password after first login!\n")
	fmt.Printf("===========================\n\n")

	return nil
}

// dropAdminAccount removes the default admin account
func dropAdminAccount(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.6.2: Removing default admin account...")

	// Begin a transaction for atomicity
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if rbErr := tx.Rollback(); rbErr != nil && err == nil {
			err = rbErr
		}
	}()

	// Delete the admin account
	_, err = tx.ExecContext(ctx, `
		DELETE FROM auth.accounts WHERE username = ?
	`, "admin")

	if err != nil {
		return fmt.Errorf("failed to delete admin account: %w", err)
	}

	// Commit the transaction
	return tx.Commit()
}
