package migrations

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/moto-nrw/project-phoenix/internal/adapter/logger"
	"github.com/uptrace/bun"
)

const (
	PrivacyConsentsVersion     = "1.3.7" // Following after users_students (1.3.6)
	PrivacyConsentsDescription = "Users privacy consents table"
)

func init() {
	// Register migration with explicit version
	MigrationRegistry[PrivacyConsentsVersion] = &Migration{
		Version:     PrivacyConsentsVersion,
		Description: PrivacyConsentsDescription,
		DependsOn:   []string{"1.3.5"}, // Depends on students table
	}

	// Migration 1.3.7: Users privacy consents table
	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return usersPrivacyConsentsUp(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return usersPrivacyConsentsDown(ctx, db)
		},
	)
}

// usersPrivacyConsentsUp creates the users.privacy_consents table
func usersPrivacyConsentsUp(ctx context.Context, db *bun.DB) error {
	logger.Logger.Info("Migration 1.3.7: Creating users.privacy_consents table...")

	// Begin a transaction for atomicity
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return err
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err != sql.ErrTxDone {
			logger.Logger.WithError(err).Warn("Failed to rollback transaction")
		}
	}()

	// Create the privacy_consents table
	_, err = tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS users.privacy_consents (
			id BIGSERIAL PRIMARY KEY,
			student_id BIGINT NOT NULL,
			policy_version TEXT NOT NULL,
			accepted BOOLEAN NOT NULL DEFAULT FALSE,
			accepted_at TIMESTAMPTZ,
			expires_at TIMESTAMPTZ,
			duration_days INTEGER,
			renewal_required BOOLEAN DEFAULT FALSE,
			data_retention_days INTEGER NOT NULL DEFAULT 30,
			details JSONB,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			CONSTRAINT fk_privacy_consents_student FOREIGN KEY (student_id) 
				REFERENCES users.students(id) ON DELETE CASCADE,
			CONSTRAINT chk_expires_at_future CHECK (
				expires_at IS NULL OR expires_at > created_at
			),
			CONSTRAINT chk_data_retention_days_range CHECK (
				data_retention_days >= 1 AND data_retention_days <= 31
			)
		)
	`)
	if err != nil {
		return fmt.Errorf("error creating privacy_consents table: %w", err)
	}

	// Add column comments
	_, err = tx.ExecContext(ctx, `
		COMMENT ON COLUMN users.privacy_consents.data_retention_days IS 
		'Number of days (1-31) to retain student visit data after creation. Visit records older than this will be automatically deleted.';
	`)
	if err != nil {
		return fmt.Errorf("error adding column comment: %w", err)
	}

	// Create indexes for privacy_consents
	_, err = tx.ExecContext(ctx, `
		CREATE INDEX IF NOT EXISTS idx_privacy_consents_student_id ON users.privacy_consents(student_id);
		CREATE INDEX IF NOT EXISTS idx_privacy_consents_expires_at ON users.privacy_consents(expires_at);
		CREATE INDEX IF NOT EXISTS idx_privacy_consents_policy_version ON users.privacy_consents(policy_version);
	`)
	if err != nil {
		return fmt.Errorf("error creating indexes for privacy_consents table: %w", err)
	}

	// Create function to automatically set expiration date based on duration_days
	_, err = tx.ExecContext(ctx, `
		CREATE OR REPLACE FUNCTION set_privacy_consent_expiration() 
		RETURNS TRIGGER AS $$
		BEGIN
			-- Set expires_at based on duration_days if not explicitly set
			IF NEW.duration_days IS NOT NULL AND NEW.expires_at IS NULL AND NEW.accepted_at IS NOT NULL THEN
				NEW.expires_at := NEW.accepted_at + (NEW.duration_days || ' days')::INTERVAL;
			END IF;
			
			-- Set renewal_required flag if expiration is approaching (30 days before)
			IF NEW.expires_at IS NOT NULL AND NOT NEW.renewal_required THEN
				IF NEW.expires_at - INTERVAL '30 days' <= CURRENT_TIMESTAMP THEN
					NEW.renewal_required := TRUE;
				END IF;
			END IF;
			
			RETURN NEW;
		END;
		$$ LANGUAGE plpgsql;
	`)
	if err != nil {
		return fmt.Errorf("error creating set_privacy_consent_expiration function: %w", err)
	}

	// Create triggers for privacy_consents
	_, err = tx.ExecContext(ctx, `
		-- Trigger for updated_at
		DROP TRIGGER IF EXISTS update_privacy_consents_updated_at ON users.privacy_consents;
		CREATE TRIGGER update_privacy_consents_updated_at
		BEFORE UPDATE ON users.privacy_consents
		FOR EACH ROW
		EXECUTE FUNCTION update_modified_column();
		
		-- Trigger for expiration calculation
		DROP TRIGGER IF EXISTS set_privacy_expiration ON users.privacy_consents;
		CREATE TRIGGER set_privacy_expiration
		BEFORE INSERT OR UPDATE ON users.privacy_consents
		FOR EACH ROW
		EXECUTE FUNCTION set_privacy_consent_expiration();
	`)
	if err != nil {
		return fmt.Errorf("error creating triggers for privacy_consents: %w", err)
	}

	// Create view for expired consents
	_, err = tx.ExecContext(ctx, `
		CREATE OR REPLACE VIEW users.expired_privacy_consents AS
		SELECT 
			pc.*,
			s.person_id, 
			s.guardian_name,
			s.guardian_email,
			s.guardian_phone
		FROM users.privacy_consents pc
		JOIN users.students s ON pc.student_id = s.id
		WHERE pc.expires_at < CURRENT_TIMESTAMP 
		  AND pc.accepted = TRUE
		  AND pc.renewal_required = TRUE;
	`)
	if err != nil {
		return fmt.Errorf("error creating expired_privacy_consents view: %w", err)
	}

	// Note: Index on visits table will be created in the visits migration after the table exists

	// Create audit schema and table for data deletion tracking (GDPR compliance)
	_, err = tx.ExecContext(ctx, `
		CREATE SCHEMA IF NOT EXISTS audit;
		
		CREATE TABLE IF NOT EXISTS audit.data_deletions (
			id BIGSERIAL PRIMARY KEY,
			student_id BIGINT NOT NULL,
			deletion_type TEXT NOT NULL, -- 'visit_retention', 'manual', 'gdpr_request'
			records_deleted INT NOT NULL,
			deletion_reason TEXT,
			deleted_by TEXT NOT NULL, -- 'system' or account username
			deleted_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			metadata JSONB, -- Additional information about the deletion
			
			-- Index for querying by student
			CONSTRAINT fk_data_deletions_student FOREIGN KEY (student_id)
				REFERENCES users.students(id) ON DELETE CASCADE
		);
		
		CREATE INDEX IF NOT EXISTS idx_data_deletions_student_id ON audit.data_deletions(student_id);
		CREATE INDEX IF NOT EXISTS idx_data_deletions_deleted_at ON audit.data_deletions(deleted_at);
		CREATE INDEX IF NOT EXISTS idx_data_deletions_type ON audit.data_deletions(deletion_type);
	`)
	if err != nil {
		return fmt.Errorf("error creating audit table: %w", err)
	}

	// Commit the transaction
	return tx.Commit()
}

// usersPrivacyConsentsDown removes the users.privacy_consents table
func usersPrivacyConsentsDown(ctx context.Context, db *bun.DB) error {
	logger.Logger.Info("Rolling back migration 1.3.7: Removing users.privacy_consents table...")

	// Begin a transaction for atomicity
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return err
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err != sql.ErrTxDone {
			logger.Logger.WithError(err).Warn("Failed to rollback transaction in down migration")
		}
	}()

	// Drop the view first, then the triggers, function, and table
	_, err = tx.ExecContext(ctx, `
		-- Drop audit table
		DROP TABLE IF EXISTS audit.data_deletions;
		
		-- Drop view
		DROP VIEW IF EXISTS users.expired_privacy_consents;
		
		-- Drop triggers
		DROP TRIGGER IF EXISTS update_privacy_consents_updated_at ON users.privacy_consents;
		DROP TRIGGER IF EXISTS set_privacy_expiration ON users.privacy_consents;
		
		-- Drop function
		DROP FUNCTION IF EXISTS set_privacy_consent_expiration();
		
		-- Drop table
		DROP TABLE IF EXISTS users.privacy_consents;
	`)
	if err != nil {
		return fmt.Errorf("error dropping users.privacy_consents resources: %w", err)
	}

	// Commit the transaction
	return tx.Commit()
}
