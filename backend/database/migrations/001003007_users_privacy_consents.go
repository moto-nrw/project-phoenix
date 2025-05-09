package migrations

import (
	"context"
	"database/sql"
	"fmt"

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
		DependsOn:   []string{"1.3.6"}, // Depends on students table
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
	fmt.Println("Migration 1.3.7: Creating users.privacy_consents table...")

	// Begin a transaction for atomicity
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

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
			details JSONB,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			CONSTRAINT fk_privacy_consents_student FOREIGN KEY (student_id) 
				REFERENCES users.students(id) ON DELETE CASCADE,
			CONSTRAINT chk_expires_at_future CHECK (
				expires_at IS NULL OR expires_at > created_at
			)
		)
	`)
	if err != nil {
		return fmt.Errorf("error creating privacy_consents table: %w", err)
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

	// Commit the transaction
	return tx.Commit()
}

// usersPrivacyConsentsDown removes the users.privacy_consents table
func usersPrivacyConsentsDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.3.7: Removing users.privacy_consents table...")

	// Begin a transaction for atomicity
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Drop the view first, then the triggers, function, and table
	_, err = tx.ExecContext(ctx, `
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
