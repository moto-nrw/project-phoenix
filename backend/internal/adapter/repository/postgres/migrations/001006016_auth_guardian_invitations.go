package migrations

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	AuthGuardianInvitationsVersion     = "1.6.16"
	AuthGuardianInvitationsDescription = "Create auth.guardian_invitations table for tracking guardian account invitations"
)

func init() {
	// Register migration with explicit version
	MigrationRegistry[AuthGuardianInvitationsVersion] = &Migration{
		Version:     AuthGuardianInvitationsVersion,
		Description: AuthGuardianInvitationsDescription,
		DependsOn:   []string{"1.3.5.1", "1.0.1"}, // Depends on guardian_profiles and auth.accounts
	}

	// Migration 1.6.16: Create guardian_invitations table
	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return authGuardianInvitationsUp(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return authGuardianInvitationsDown(ctx, db)
		},
	)
}

// authGuardianInvitationsUp creates the auth.guardian_invitations table
func authGuardianInvitationsUp(ctx context.Context, db *bun.DB) error {
	LogMigration(AuthGuardianInvitationsVersion, "Creating auth.guardian_invitations table...")

	// Begin a transaction for atomicity
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err != sql.ErrTxDone {
			logRollbackError(err)
		}
	}()

	// Create the guardian_invitations table
	_, err = tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS auth.guardian_invitations (
			id BIGSERIAL PRIMARY KEY,
			token TEXT NOT NULL UNIQUE,

			-- References guardian profile (not account - account created on acceptance)
			guardian_profile_id BIGINT NOT NULL,

			-- Metadata
			created_by BIGINT NOT NULL, -- Staff/admin who sent invitation
			expires_at TIMESTAMPTZ NOT NULL,
			accepted_at TIMESTAMPTZ,

			-- Email tracking
			email_sent_at TIMESTAMPTZ,
			email_error TEXT,
			email_retry_count INT DEFAULT 0,

			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

			-- Foreign Keys
			CONSTRAINT fk_guardian_invitation_profile
				FOREIGN KEY (guardian_profile_id) REFERENCES users.guardian_profiles(id) ON DELETE CASCADE,
			CONSTRAINT fk_guardian_invitation_creator
				FOREIGN KEY (created_by) REFERENCES auth.accounts(id) ON DELETE CASCADE
		)
	`)
	if err != nil {
		return fmt.Errorf("error creating guardian_invitations table: %w", err)
	}

	// Create indexes for guardian_invitations
	_, err = tx.ExecContext(ctx, `
		CREATE INDEX IF NOT EXISTS idx_guardian_invitations_token ON auth.guardian_invitations(token);
		CREATE INDEX IF NOT EXISTS idx_guardian_invitations_guardian_profile_id ON auth.guardian_invitations(guardian_profile_id);
		CREATE INDEX IF NOT EXISTS idx_guardian_invitations_created_by ON auth.guardian_invitations(created_by);
		CREATE INDEX IF NOT EXISTS idx_guardian_invitations_expires_at ON auth.guardian_invitations(expires_at);
		CREATE INDEX IF NOT EXISTS idx_guardian_invitations_accepted_at ON auth.guardian_invitations(accepted_at);
	`)
	if err != nil {
		return fmt.Errorf("error creating indexes for guardian_invitations table: %w", err)
	}

	// Create updated_at timestamp trigger
	_, err = tx.ExecContext(ctx, `
		-- Trigger for guardian_invitations
		DROP TRIGGER IF EXISTS update_guardian_invitations_updated_at ON auth.guardian_invitations;
		CREATE TRIGGER update_guardian_invitations_updated_at
		BEFORE UPDATE ON auth.guardian_invitations
		FOR EACH ROW
		EXECUTE FUNCTION update_modified_column();
	`)
	if err != nil {
		return fmt.Errorf("error creating updated_at trigger: %w", err)
	}

	// Commit the transaction
	return tx.Commit()
}

// authGuardianInvitationsDown removes the auth.guardian_invitations table
func authGuardianInvitationsDown(ctx context.Context, db *bun.DB) error {
	LogMigration(AuthGuardianInvitationsVersion, "Rolling back: Removing auth.guardian_invitations table...")

	// Begin a transaction for atomicity
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err != sql.ErrTxDone {
			logRollbackError(err)
		}
	}()

	// Drop the trigger first
	_, err = tx.ExecContext(ctx, `
		DROP TRIGGER IF EXISTS update_guardian_invitations_updated_at ON auth.guardian_invitations;
	`)
	if err != nil {
		return fmt.Errorf("error dropping trigger: %w", err)
	}

	// Drop the table
	_, err = tx.ExecContext(ctx, `
		DROP TABLE IF EXISTS auth.guardian_invitations CASCADE;
	`)
	if err != nil {
		return fmt.Errorf("error dropping auth.guardian_invitations table: %w", err)
	}

	// Commit the transaction
	return tx.Commit()
}
