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
	operatorAccountVersion     = "1.11.3"
	operatorAccountDescription = "Create default operator account"
)

func init() {
	MigrationRegistry[operatorAccountVersion] = &Migration{
		Version:     operatorAccountVersion,
		Description: operatorAccountDescription,
		DependsOn:   []string{"1.11.1"}, // platform.operators table
	}

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return createOperatorAccount(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return dropOperatorAccount(ctx, db)
		},
	)
}

func createOperatorAccount(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.11.3: Creating default operator account...")

	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if rbErr := tx.Rollback(); rbErr != nil && err == nil {
			err = rbErr
		}
	}()

	operatorEmail := os.Getenv("OPERATOR_EMAIL")
	if operatorEmail == "" {
		operatorEmail = "operator@example.com"
		fmt.Printf("WARNING: OPERATOR_EMAIL environment variable not set, using default: %s\n", operatorEmail)
	}

	operatorPassword := os.Getenv("OPERATOR_PASSWORD")
	if operatorPassword == "" {
		operatorPassword = "Test1234%"
		fmt.Printf("WARNING: OPERATOR_PASSWORD environment variable not set, using default password!\n")
		fmt.Printf("WARNING: Please set OPERATOR_PASSWORD environment variable for security!\n")
	}

	operatorName := os.Getenv("OPERATOR_DISPLAY_NAME")
	if operatorName == "" {
		operatorName = "moto Operator"
	}

	hashedPassword, err := userpass.HashPassword(operatorPassword, userpass.DefaultParams())
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}

	_, err = tx.ExecContext(ctx, `
		INSERT INTO platform.operators (email, display_name, password_hash, active, created_at, updated_at)
		VALUES (?, ?, ?, true, NOW(), NOW())
		ON CONFLICT (email) DO UPDATE
		SET display_name = EXCLUDED.display_name,
		    password_hash = EXCLUDED.password_hash,
		    updated_at = NOW()
	`, operatorEmail, operatorName, hashedPassword)
	if err != nil {
		return fmt.Errorf("failed to create operator account: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	fmt.Printf("\n=== Operator Account Created ===\n")
	fmt.Printf("Email: %s\n", operatorEmail)
	fmt.Printf("Display Name: %s\n", operatorName)

	if operatorPassword == "Test1234%" {
		fmt.Printf("Password: %s (DEFAULT - CHANGE IMMEDIATELY!)\n", operatorPassword)
		fmt.Printf("WARNING: Using default password! Set OPERATOR_PASSWORD environment variable!\n")
	} else {
		fmt.Printf("Password: Set via OPERATOR_PASSWORD environment variable\n")
	}

	fmt.Printf("================================\n\n")

	return nil
}

func dropOperatorAccount(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.11.3: Removing default operator account...")

	operatorEmail := os.Getenv("OPERATOR_EMAIL")
	if operatorEmail == "" {
		operatorEmail = "operator@example.com"
	}

	_, err := db.ExecContext(ctx, `
		DELETE FROM platform.operators WHERE email = ?
	`, operatorEmail)
	if err != nil {
		return fmt.Errorf("failed to delete operator account: %w", err)
	}

	return nil
}
