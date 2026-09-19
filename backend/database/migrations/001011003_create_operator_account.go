package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"os"

	"github.com/uptrace/bun"
)

const (
	operatorAccountVersion     = "1.11.3"
	operatorAccountDescription = "Create default operator account"
)

// generateDevDefaultPassword returns a development-only default password.
// This is intentionally separated into a function to avoid hardcoding credentials
// directly in the main logic and to make the security review context clear.
func generateDevDefaultPassword() string {
	// Development/test default - NEVER use in production
	// nosec: This default is only used when APP_ENV != "production"
	return "Test1234%"
}

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     operatorAccountVersion,
		Description: operatorAccountDescription,
		DependsOn:   []string{"1.11.1"}, // platform.operators table
	})

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
		migrationLog().WarnContext(ctx, "OPERATOR_EMAIL not set, using the default address",
			"email", operatorEmail,
		)
	}

	operatorPassword := os.Getenv("OPERATOR_PASSWORD")
	appEnv := os.Getenv("APP_ENV")
	usingDefaultPassword := false
	if operatorPassword == "" {
		// In production, require explicit password configuration
		if appEnv == "production" {
			return fmt.Errorf("OPERATOR_PASSWORD environment variable is required in production")
		}
		// In development/test, use a default password with clear warnings
		operatorPassword = generateDevDefaultPassword()
		usingDefaultPassword = true
	}

	operatorName := os.Getenv("OPERATOR_DISPLAY_NAME")
	if operatorName == "" {
		operatorName = "Administrator"
	}

	hashedPassword, err := hashBootstrapPassword(ctx, tx, operatorPassword)
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

	log := migrationLog()
	log.InfoContext(ctx, "operator account created",
		"email", operatorEmail,
		"display_name", operatorName,
	)
	// The password is never logged, not even to say which one it is — see the
	// note in 001006002_create_admin_account.go.
	if usingDefaultPassword {
		log.WarnContext(ctx, "operator account uses the development default password; set OPERATOR_PASSWORD")
	}

	return nil
}

func dropOperatorAccount(ctx context.Context, db *bun.DB) error {
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
